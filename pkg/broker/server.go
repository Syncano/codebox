package broker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	scriptpb "github.com/Syncano/codebox/pkg/script/proto"
	"github.com/Syncano/codebox/pkg/sys"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	brokerpb "github.com/Syncano/codebox/pkg/broker/proto"
	repopb "github.com/Syncano/codebox/pkg/filerepo/proto"
	lbpb "github.com/Syncano/codebox/pkg/lb/proto"
	"github.com/Syncano/codebox/pkg/util"
)

// Server defines a Broker server.
//go:generate go run github.com/vektra/mockery/cmd/mockery -dir proto -all
type Server struct {
	redisCli  RedisClient
	lbServers []*loadBalancer

	mu         sync.Mutex
	uploads    map[string]chan struct{}
	downloader util.Downloader

	options ServerOptions
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
}

// DefaultOptions holds default options values for Broker server.
var DefaultOptions = &ServerOptions{
	DownloadConcurrency: 16,
	LBRetry:             3,
	MaxPayloadSize:      6 << 20,
}

var (
	// ErrInvalidArgument signals that there are no suitable workers at this moment.
	ErrInvalidArgument = status.Error(codes.InvalidArgument, "invalid argument")

	initOnce                    sync.Once
	executionDurationsHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "codebox_execution_duration_seconds",
		Help:    "Codebox execution latency distributions.",
		Buckets: []float64{.1, .25, .5, 1, 2.5, 10, 30, 60, 120, 180},
	})
	overheadDurationsHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "codebox_overhead_duration_seconds",
		Help: "Codebox overhead latency distributions.",
	})
	executionCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codebox_executions_total",
			Help: "Codebox executions.",
		},
		[]string{"status"},
	)
)

const (
	environmentFileName = "squashfs.img"
	chunkSize           = 2 * 1024 * 1024
	lbRetrySleep        = 3 * time.Millisecond
	defaultTimeout      = 8 * time.Minute
	downloadTimeout     = 2 * time.Minute
)

// NewServer initializes new Broker server.
func NewServer(redisClient RedisClient, options *ServerOptions) (*Server, error) {
	// Register prometheus exports.
	initOnce.Do(func() {
		prometheus.MustRegister(
			executionDurationsHistogram,
			overheadDurationsHistogram,
			executionCounter,
		)
	})

	lbServers := make([]*loadBalancer, 0, len(options.LBAddr))

	// Initialize all load balancer connections.
	for _, addr := range options.LBAddr {
		logrus.WithField("addr", addr).Info("Initializing connection to Load Balancer")
		conn, err := grpc.Dial(addr, sys.DefaultGRPCDialOptions...)
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
		lbServers:  lbServers,
		uploads:    make(map[string]chan struct{}),
		downloader: util.NewDownloader(&util.DownloaderOptions{Concurrency: options.DownloadConcurrency}),
		options:    *options,
	}, nil
}

// Options returns a copy of Broker options struct.
func (s *Server) Options() ServerOptions {
	return s.options
}

// Shutdown stops gracefully Broker server.
func (s *Server) Shutdown() {
	for _, lb := range s.lbServers {
		lb.conn.Close()
	}
}

// Run runs script in secure environment.
func (s *Server) Run(request *brokerpb.RunRequest, stream brokerpb.ScriptRunner_RunServer) error {
	peerAddr := util.PeerAddr(stream.Context())
	start := time.Now()
	logger := logrus.WithField("peer", peerAddr)

	if len(request.GetRequest()) < 1 || request.GetRequest()[0].GetMeta() == nil || request.LbMeta == nil {
		logger.Error("grpc:broker:Run error parsing input")
		return ErrInvalidArgument
	}

	scriptMeta := request.GetRequest()[0].GetMeta()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)

	if request.LbMeta == nil {
		request.LbMeta = &lbpb.RunRequest_MetaMessage{}
	}

	if request.LbMeta.RequestID == "" {
		request.LbMeta.RequestID = util.GenerateShortKey()
	}

	logger = logger.WithFields(logrus.Fields{
		"reqID":      request.LbMeta.RequestID,
		"cKey":       request.LbMeta.ConcurrencyKey,
		"cLimit":     request.LbMeta.ConcurrencyLimit,
		"runtime":    scriptMeta.Runtime,
		"sourceHash": scriptMeta.SourceHash,
		"entryPoint": scriptMeta.GetOptions().GetEntryPoint(),
		"async":      scriptMeta.GetOptions().GetAsync(),
		"mcpu":       scriptMeta.GetOptions().GetMCPU(),
		"userID":     scriptMeta.UserID,
	})

	runStream, err := s.processRun(ctx, logger, request)
	if err != nil {
		logger.WithError(err).Warn("grpc:broker:Run")
		cancel()

		return err
	}

	var retStream brokerpb.ScriptRunner_RunServer

	processFunc := func() error {
		trace, err := s.processResponse(logger, start, request.GetMeta(), runStream, retStream)

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

	if request.GetMeta().Sync {
		// Process response synchronously.
		retStream = stream
		return processFunc()
	}

	// Process response asynchronously.
	go processFunc() // nolint: errcheck

	return nil
}

func (s *Server) processRun(ctx context.Context, logger logrus.FieldLogger, request *brokerpb.RunRequest) (lbpb.ScriptRunner_RunClient, error) {
	meta := request.GetMeta()
	lbMeta := request.GetLbMeta()
	scriptMeta := request.GetRequest()[0].GetMeta()

	lbID := util.Hash(lbMeta.GetConcurrencyKey()) % uint32(len(s.lbServers))
	lb := s.lbServers[lbID]
	logger = logger.WithField("lb", lb)

	var stream lbpb.ScriptRunner_RunClient

	// Start processing and retry in case of network error.
	if e := util.Retry(s.options.LBRetry, lbRetrySleep, func() error {
		// Upload script files if needed.
		files := meta.GetFiles()
		if err := s.uploadFiles(ctx, lb, scriptMeta.SourceHash, files); err != nil {
			logger.WithError(err).WithField("key", scriptMeta.SourceHash).Error("File upload error")
			return err
		}

		// Upload environment file if needed.
		envURL := meta.GetEnvironmentURL()
		if envURL != "" {
			if err := s.uploadFiles(ctx, lb, scriptMeta.Environment, map[string]string{envURL: environmentFileName}); err != nil {
				logger.WithError(err).WithField("key", scriptMeta.Environment).Error("Environment upload error")
				return err
			}
		}

		var err error
		stream, err = lb.lbCli.Run(ctx)
		if err != nil {
			return err
		}
		if err := stream.Send(&lbpb.RunRequest{
			Value: &lbpb.RunRequest_Meta{
				Meta: lbMeta,
			},
		}); err != nil {
			logger.WithError(err).Error("LB grpc:lb:Run sending LB meta failed")
			return err
		}

		for _, scriptReq := range request.GetRequest() {
			if err := stream.Send(&lbpb.RunRequest{
				Value: &lbpb.RunRequest_Request{
					Request: scriptReq,
				},
			}); err != nil {
				logger.WithError(err).Error("LB grpc:lb:Run sending Script request failed")
				return err
			}
		}
		return stream.CloseSend()
	}); e != nil {
		executionCounter.WithLabelValues(errorStatus).Inc()
		return nil, e
	}

	if err := s.updateTrace(meta.GetTraceID(), meta.GetTrace()); err != nil {
		logger.WithError(err).Warn("UpdateTrace failed")
	}

	return stream, nil
}

func uploadChunks(stream repopb.Repo_UploadClient, key string, resCh <-chan *util.DownloadResult, files map[string]string) error {
	var err error

	// Send meta header.
	if err := stream.Send(&repopb.UploadRequest{
		Value: &repopb.UploadRequest_Meta{
			Meta: &repopb.UploadRequest_MetaMessage{Key: key},
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
					Chunk: &repopb.UploadRequest_ChunkMessage{
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
	fmt.Println("FILES", files)
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

func (s *Server) processResponse(logger logrus.FieldLogger, start time.Time, meta *brokerpb.RunRequest_MetaMessage,
	stream lbpb.ScriptRunner_RunClient, retStream brokerpb.ScriptRunner_RunServer) (*ScriptTrace, error) {
	retSend := func(r *scriptpb.RunResponse) {
		if retStream != nil {
			e := retStream.Send(r)
			if e != nil {
				logger.WithError(e).Warn("LB grpc:broker:Send error")

				retStream = nil
			}
		}
	}

	result, err := stream.Recv()
	if err != nil {
		logger.WithError(err).Warn("LB grpc:lb:Run error")
	} else {
		retSend(result)

		// Read until all chunks arrive.
		for {
			chunk, e := stream.Recv()
			if e != nil {
				if e != io.EOF {
					logger.WithError(e).Warn("LB grpc:lb:Recv error")
				}
				break
			}
			result.Response.Content = append(result.Response.Content, chunk.Response.Content...)
			retSend(chunk)
		}
	}

	updatedTrace := NewScriptTrace(meta.GetTraceID(), result)
	if e := s.saveTrace(meta.GetTrace(), updatedTrace); e != nil {
		logger.WithError(e).Error("SaveTrace failed")
	}

	// Update prometheus stats.
	executionCounter.WithLabelValues(updatedTrace.Status).Inc()

	if result != nil {
		durationSeconds := float64(updatedTrace.Duration) / 1e3
		executionDurationsHistogram.Observe(durationSeconds)
		overheadDurationsHistogram.Observe(time.Since(start).Seconds() - durationSeconds) // network + docker overhead
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

	return NewCelerySaveTask(trace, updatedTrace).Publish()
}

func (s *Server) updateTrace(traceID uint64, trace []byte) error {
	if traceID == 0 || trace == nil {
		return nil
	}

	return NewCeleryUpdateTask(trace).Publish()
}
