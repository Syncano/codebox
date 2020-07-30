package broker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	census_trace "go.opencensus.io/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/Syncano/codebox/app/common"
	"github.com/Syncano/pkg-go/v2/celery"
	"github.com/Syncano/pkg-go/v2/util"
	brokerpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/broker/v1"
	repopb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/filerepo/v1"
	lbpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/lb/v1"
	scriptpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"
)

// Server defines a Broker server.
//go:generate go run github.com/vektra/mockery/cmd/mockery -dir ../../proto/gen/go/syncano/codebox/broker -all
type Server struct {
	redisCli  RedisClient
	lbServers []*loadBalancer
	cel       *celery.Celery

	mu         sync.Mutex
	uploads    map[string]chan struct{}
	downloader util.Downloader

	options *ServerOptions
}

type loadBalancer struct {
	addr    string
	conn    *grpc.ClientConn
	repoCli repopb.RepoClient
	lbCli   lbpb.ScriptRunnerClient
}

func (lb *loadBalancer) String() string {
	return fmt.Sprintf("{Addr:%s}", lb.addr)
}

// ServerOptions holds settable options for Broker server.
type ServerOptions struct {
	LBAddr              []string
	LBRetry             int
	DownloadConcurrency uint
	MaxPayloadSize      int64
	MaxTimeout          time.Duration
}

// DefaultOptions holds default options values for Broker server.
var DefaultOptions = &ServerOptions{
	DownloadConcurrency: 16,
	LBRetry:             3,
	MaxPayloadSize:      15 << 20,
	MaxTimeout:          8 * time.Minute,
}

var (
	initOnce         sync.Once
	overheadDuration = stats.Float64(
		"codebox/overhead/duration/seconds",
		"Codebox overhead duration.",
		stats.UnitSeconds)

	overheadDurationView = &view.View{
		Name:        "codebox/overhead/duration/seconds",
		Description: "Codebox overhead distribution.",
		Measure:     overheadDuration,
		Aggregation: view.Distribution(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10),
	}
)

const (
	environmentFileName = "squashfs.img"
	chunkSize           = 2 * 1024 * 1024
	lbRetrySleep        = 3 * time.Millisecond
	downloadTimeout     = 2 * time.Minute
)

// NewServer initializes new Broker server.
func NewServer(redisClient RedisClient, cel *celery.Celery, opts *ServerOptions) (*Server, error) {
	options := *DefaultOptions
	_ = mergo.Merge(&options, opts, mergo.WithOverride)

	// Register prometheus exports.
	initOnce.Do(func() {
		util.Must(view.Register(overheadDurationView))
	})

	lbServers := make([]*loadBalancer, 0, len(options.LBAddr))

	// Initialize all load balancer connections.
	for _, addr := range options.LBAddr {
		logrus.WithField("addr", addr).Info("Initializing connection to Load Balancer")
		conn, err := grpc.Dial(addr,
			grpc.WithInsecure(),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(common.MaxGRPCMessageSize)),
			grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
		)
		util.Must(err)

		lbServers = append(lbServers, &loadBalancer{
			addr:    addr,
			conn:    conn,
			lbCli:   lbpb.NewScriptRunnerClient(conn),
			repoCli: repopb.NewRepoClient(conn),
		})
	}

	return &Server{
		redisCli:   redisClient,
		cel:        cel,
		lbServers:  lbServers,
		uploads:    make(map[string]chan struct{}),
		downloader: util.NewDownloader(util.WithConcurrency(options.DownloadConcurrency)),
		options:    &options,
	}, nil
}

// Options returns a copy of Broker options struct.
func (s *Server) Options() ServerOptions {
	return *s.options
}

// Shutdown stops gracefully Broker server.
func (s *Server) Shutdown() {
	for _, lb := range s.lbServers {
		lb.conn.Close()
	}
}

func (s *Server) SimpleRun(req *brokerpb.SimpleRunRequest, stream brokerpb.ScriptRunner_SimpleRunServer) error {
	ctx, reqID := util.AddDefaultRequestID(stream.Context())
	peerAddr := util.PeerAddr(ctx)
	logger := logrus.WithFields(logrus.Fields{"peer": peerAddr, "reqID": reqID})

	return s.processRun(ctx, logger, req.Meta, req.LbMeta, req.ScriptMeta, nil, stream)
}

func (s *Server) Run(stream brokerpb.ScriptRunner_RunServer) error {
	var (
		meta       *brokerpb.RunMeta
		lbMeta     *lbpb.RunMeta
		scriptMeta *scriptpb.RunMeta
	)

	ctx, reqID := util.AddDefaultRequestID(stream.Context())
	peerAddr := util.PeerAddr(ctx)
	logger := logrus.WithFields(logrus.Fields{"peer": peerAddr, "reqID": reqID})

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		switch v := in.Value.(type) {
		case *brokerpb.RunRequest_Meta:
			meta = v.Meta
		case *brokerpb.RunRequest_LbMeta:
			lbMeta = v.LbMeta
		case *brokerpb.RunRequest_ScriptMeta:
			scriptMeta = v.ScriptMeta
		default:
			logger.Error("grpc:broker:Run error parsing input")
			return common.ErrInvalidArgument
		}

		if meta != nil && lbMeta != nil && scriptMeta != nil {
			break
		}
	}

	chunkReader := common.NewChunkReader(func() (*scriptpb.RunChunk, error) {
		req, err := stream.Recv()
		if err != nil {
			return nil, err
		}

		chunk := req.GetScriptChunk()
		if chunk == nil {
			return nil, common.ErrInvalidArgument
		}

		return chunk, nil
	})

	return s.processRun(ctx, logger, meta, lbMeta, scriptMeta, chunkReader, stream)
}

func (s *Server) Delete(ctx context.Context, req *scriptpb.DeleteRequest) (*scriptpb.DeleteResponse, error) {
	var (
		ret *scriptpb.DeleteResponse
		err error
	)

	ret = &scriptpb.DeleteResponse{}

	for _, lb := range s.lbServers {
		res, e := lb.lbCli.Delete(ctx, req)
		if e != nil {
			err = e
		} else {
			ret.ContainerIds = append(ret.ContainerIds, res.ContainerIds...)
		}
	}

	return ret, err
}

func (s *Server) processRun(ctx context.Context, logger logrus.FieldLogger,
	meta *brokerpb.RunMeta, lbMeta *lbpb.RunMeta, scriptMeta *scriptpb.RunMeta, chunkReader *common.ChunkReader,
	stream StreamReponder) error {
	if meta == nil || lbMeta == nil || scriptMeta == nil {
		logger.Error("grpc:broker:Run error parsing input")
		return common.ErrInvalidArgument
	}

	start := time.Now()

	// Create new context and add trace metadata to it to process traces even for canceled contexts.
	newCtx, cancel := context.WithTimeout(
		census_trace.NewContext(context.Background(), census_trace.FromContext(ctx)),
		s.options.MaxTimeout)
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		newCtx = metadata.NewOutgoingContext(newCtx, md)
	}

	ctx = newCtx

	logger = logger.WithFields(logrus.Fields{
		"cKey":       lbMeta.ConcurrencyKey,
		"cLimit":     lbMeta.ConcurrencyLimit,
		"runtime":    scriptMeta.Runtime,
		"sourceHash": scriptMeta.SourceHash,
		"entryPoint": scriptMeta.GetOptions().GetEntrypoint(),
		"async":      scriptMeta.GetOptions().GetAsync(),
		"mcpu":       scriptMeta.GetOptions().GetMcpu(),
		"userID":     scriptMeta.UserId,
	})

	lbStream, err := s.sendRunToLoadbalancer(ctx, logger, meta, lbMeta, scriptMeta, chunkReader)
	if err != nil {
		logger.WithError(err).Warn("grpc:broker:Run")
		cancel()

		return err
	}

	processFunc := func(ctx context.Context, retStream StreamReponder) error {
		trace, err := s.processResponse(ctx, logger, start, meta, lbStream, retStream)

		cancel()

		took := time.Duration(trace.Duration) * time.Millisecond
		logger = logger.WithFields(logrus.Fields{
			"took":     took,
			"overhead": time.Since(start) - took,
		})

		if err != nil {
			logger.WithError(err).Warn("grpc:broker:Run")
		} else {
			logger.Info("grpc:broker:Run")
		}

		return err
	}

	if meta.Sync {
		// Process response synchronously.
		return processFunc(ctx, stream)
	}

	// Process response asynchronously.
	go processFunc(ctx, nil) // nolint: errcheck

	return nil
}

func (s *Server) sendRunToLoadbalancer(ctx context.Context, logger logrus.FieldLogger,
	meta *brokerpb.RunMeta, lbMeta *lbpb.RunMeta, scriptMeta *scriptpb.RunMeta, chunkReader *common.ChunkReader) (lbpb.ScriptRunner_RunClient, error) {
	lbID := util.Hash(lbMeta.GetConcurrencyKey()) % uint32(len(s.lbServers))
	lb := s.lbServers[lbID]
	logger = logger.WithFields(logrus.Fields{"lb": lb, "sourceHash": scriptMeta.SourceHash, "envHash": scriptMeta.Environment})

	var lbStream lbpb.ScriptRunner_RunClient

	// Start processing and retry in case of network error.
	canceled, err := util.RetryNotCancelled(s.options.LBRetry, lbRetrySleep, func() error {
		// Upload script files if needed.
		files := meta.GetFiles()
		if err := s.uploadFiles(ctx, lb, scriptMeta.SourceHash, files); err != nil {
			return fmt.Errorf("file upload error: %w", err)
		}

		// Upload environment file if needed.
		envURL := meta.GetEnvironmentUrl()
		if envURL != "" {
			if err := s.uploadFiles(ctx, lb, scriptMeta.Environment, map[string]string{envURL: environmentFileName}); err != nil {
				return fmt.Errorf("environment upload error: %w", err)
			}
		}

		var err error
		lbStream, err = lb.lbCli.Run(ctx)
		if err != nil {
			return fmt.Errorf("lb run error: %w", err)
		}
		if err := lbStream.Send(&lbpb.RunRequest{
			Value: &lbpb.RunRequest_Meta{
				Meta: lbMeta,
			},
		}); err != nil {
			return fmt.Errorf("sending lb meta failed: %w", err)
		}

		if err := lbStream.Send(&lbpb.RunRequest{
			Value: &lbpb.RunRequest_ScriptMeta{
				ScriptMeta: scriptMeta,
			},
		}); err != nil {
			return fmt.Errorf("sending script meta failed: %w", err)
		}

		return nil
	})

	if err != nil {
		if !canceled {
			logrus.WithError(err).Error("grpc:lb:Run error")
		}

		return nil, err
	}

	// Send remaining script chunks
	if chunkReader != nil {
		var chunk *scriptpb.RunChunk

		for {
			chunk, err = chunkReader.Get()
			if err == io.EOF {
				err = nil
				break
			}

			if err != nil {
				err = fmt.Errorf("reading script chunk failed: %w", err)
				break
			}

			if err = lbStream.Send(&lbpb.RunRequest{
				Value: &lbpb.RunRequest_ScriptChunk{
					ScriptChunk: chunk,
				},
			}); err != nil {
				err = fmt.Errorf("sending script request failed: %w", err)
				break
			}
		}

		if err != nil {
			if !util.IsContextError(err) {
				logrus.WithError(err).Error("grpc:lb:Run error")
			}

			return nil, err
		}
	}

	_ = lbStream.CloseSend()

	if err := s.updateTrace(meta.GetTraceId(), meta.GetTrace()); err != nil {
		logger.WithError(err).Warn("UpdateTrace failed")
	}

	return lbStream, nil
}

func uploadChunks(stream repopb.Repo_UploadClient, key string, resCh <-chan *util.DownloadResult, files map[string]string) error {
	var err error

	// Send meta header.
	if err := stream.Send(&repopb.UploadRequest{
		Value: &repopb.UploadRequest_Meta{
			Meta: &repopb.UploadMetaMessage{Key: key},
		},
	}); err != nil {
		return err
	}

	// Wait for response to see if upload was accepted or not.
	var r *repopb.UploadResponse

	if r, err = stream.Recv(); err != nil {
		return err
	}

	if !r.Accepted {
		logrus.WithField("key", key).Debug("Upload Rejected")
		return nil
	}

	logrus.WithField("key", key).Debug("Upload Accepted")

	for res := range resCh {
		logrus.WithFields(logrus.Fields{
			"key": key,
			"res": res,
		}).Debug("Sending Download result")

		if res.Error != nil {
			return res.Error
		}

		// Send chunks of files.
		name := files[res.URL]
		buf := make([]byte, chunkSize)
		reader := bytes.NewReader(res.Data)

		for {
			n, e := reader.Read(buf)
			if e == io.EOF {
				break
			}

			if err := stream.Send(&repopb.UploadRequest{
				Value: &repopb.UploadRequest_Chunk{
					Chunk: &repopb.UploadChunkMessage{
						Name: name,
						Data: buf[:n],
					},
				},
			}); err != nil {
				return err
			}
		}
	}

	// Send done flag.
	if err := stream.Send(&repopb.UploadRequest{Value: &repopb.UploadRequest_Done{Done: true}}); err != nil {
		return err
	}
	// Wait for response as a confirmation of finished upload.
	_, err = stream.Recv()

	return err
}

func (s *Server) uploadFiles(ctx context.Context, lb *loadBalancer, key string, files map[string]string) error {
	exists, err := lb.repoCli.Exists(ctx, &repopb.ExistsRequest{Key: key}, grpc.WaitForReady(true))
	if err != nil {
		return err
	}

	if exists.Ok {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"lb":  lb,
		"key": key,
	}).Debug("Downloading files and uploading them to load balancer")

	// Check first if there is a concurrent download going on.
	uploadKey := fmt.Sprintf("%s;%s", lb.addr, key)

	s.mu.Lock()

	doneCh, ok := s.uploads[uploadKey]
	// And if there is one - wait for it to be done.
	if ok {
		s.mu.Unlock()
		<-doneCh

		return nil
	}

	doneCh = make(chan struct{})
	s.uploads[uploadKey] = doneCh

	s.mu.Unlock()

	// Download and once done, close channel.
	defer func() {
		close(doneCh)
		s.mu.Lock()
		delete(s.uploads, uploadKey)
		s.mu.Unlock()
	}()

	stream, err := lb.repoCli.Upload(ctx)
	if err != nil {
		return err
	}

	// Start downloading files.
	fileURLs := make([]string, 0, len(files))

	for url := range files {
		fileURLs = append(fileURLs, url)
	}
	// Start a separate timeout so we cancel downloads when needed.
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	resCh := s.downloader.Download(ctx, fileURLs)

	// Iterate through download results and upload them.
	if err = uploadChunks(stream, key, resCh, files); err != nil {
		if exists, e := lb.repoCli.Exists(ctx, &repopb.ExistsRequest{Key: key}); e == nil && exists.Ok {
			return nil
		}
	}

	return err
}

func (s *Server) processResponse(ctx context.Context, logger logrus.FieldLogger, start time.Time, meta *brokerpb.RunMeta,
	stream lbpb.ScriptRunner_RunClient, retStream StreamReponder) (*ScriptTrace, error) {
	retSend := func(r *scriptpb.RunResponse) {
		if retStream != nil {
			e := retStream.Send(r)
			if e != nil {
				logger.WithError(e).Warn("grpc:broker:Send error")

				retStream = nil
			}
		}
	}

	result, err := stream.Recv()
	if err != nil {
		logger.WithError(err).Warn("grpc:lb:Run error")
	} else {
		retSend(result)

		// Read until all chunks arrive.
		for {
			chunk, e := stream.Recv()
			if e != nil {
				if e != io.EOF {
					logger.WithError(e).Warn("grpc:lb:Recv error")
				}
				break
			}
			result.Response.Content = append(result.Response.Content, chunk.Response.Content...)
			retSend(chunk)
		}
	}

	updatedTrace := NewScriptTrace(meta.GetTraceId(), result)
	if e := s.saveTrace(meta.GetTrace(), updatedTrace); e != nil {
		logger.WithError(e).Error("SaveTrace failed")
	}

	// Update prometheus stats.

	if result != nil {
		durationSeconds := float64(updatedTrace.Duration) / 1e3
		stats.Record(ctx, overheadDuration.M(time.Since(start).Seconds()-durationSeconds)) // network + docker overhead
	}

	return updatedTrace, err
}

func (s *Server) saveTrace(trace []byte, updatedTrace *ScriptTrace) error {
	if trace == nil {
		return nil
	}

	traceID := updatedTrace.ID

	defer func() {
		updatedTrace.ID = traceID
	}()

	return NewCelerySaveTask(trace, updatedTrace).Publish(s.cel)
}

func (s *Server) updateTrace(traceID uint64, trace []byte) error {
	if traceID == 0 || trace == nil {
		return nil
	}

	return NewCeleryUpdateTask(trace).Publish(s.cel)
}
