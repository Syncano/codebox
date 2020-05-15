package lb

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Syncano/codebox/pkg/cache"
	"github.com/Syncano/codebox/pkg/common"
	"github.com/Syncano/codebox/pkg/filerepo"
	"github.com/Syncano/codebox/pkg/limiter"
	"github.com/Syncano/codebox/pkg/util"
	pb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/lb/v1"
	scriptpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"
)

// Server defines a Load Balancer server implementing both worker plug and script runner interface.
//go:generate go run github.com/vektra/mockery/cmd/mockery -dir ../../proto/gen/go/syncano/codebox/lb -all
type Server struct {
	mu                   sync.Mutex
	workers              *cache.LRUCache // workerID->Worker
	workerContainerCache ContainerWorkerCache

	fileRepo filerepo.Repo
	options  ServerOptions
	limiter  *limiter.Limiter
	metrics  *MetricsData
}

// Assert that Server is compatible with proto interface.
var _ pb.WorkerPlugServer = (*Server)(nil)

// ServerOptions holds settable options for LB server.
type ServerOptions struct {
	// Workers
	WorkerRetry          int
	WorkerKeepalive      time.Duration
	WorkerMinReady       int
	WorkerErrorThreshold uint32

	// Limiter
	LimiterOptions limiter.Options
}

// DefaultOptions holds default options values for LB server.
var DefaultOptions = &ServerOptions{
	WorkerRetry:          3,
	WorkerKeepalive:      30 * time.Second,
	WorkerErrorThreshold: 2,
}

var (
	// ErrUnknownWorkerID signals that we received a message for unknown worker.
	ErrUnknownWorkerID = status.Error(codes.InvalidArgument, "unknown worker id")
	// ErrNoWorkersAvailable signals that there are no suitable workers at this moment.
	ErrNoWorkersAvailable = status.Error(codes.ResourceExhausted, "no workers available")
	// ErrSourceNotAvailable signals that specified source hash was not found.
	ErrSourceNotAvailable = status.Error(codes.FailedPrecondition, "source not available")
	ErrInvalidArgument    = status.Error(codes.InvalidArgument, "invalid argument")
)

const (
	workerRetrySleep = 3 * time.Millisecond
)

// NewServer initializes new LB server.
func NewServer(fileRepo filerepo.Repo, options *ServerOptions) *Server {
	if options != nil {
		mergo.Merge(options, DefaultOptions) // nolint - error not possible
	} else {
		options = DefaultOptions
	}

	workers := cache.NewLRUCache(&cache.Options{
		TTL: options.WorkerKeepalive,
	}, &cache.LRUOptions{
		AutoRefresh: false,
	})
	s := &Server{
		options:              *options,
		workers:              workers,
		workerContainerCache: make(ContainerWorkerCache),
		fileRepo:             fileRepo,
		limiter:              limiter.New(&limiter.Options{}),
		metrics:              Metrics(),
	}
	workers.OnValueEvicted(s.onEvictedWorkerHandler)

	return s
}

// Options returns a copy of LB options struct.
func (s *Server) Options() ServerOptions {
	return s.options
}

func (s *Server) Metrics() *MetricsData {
	return s.metrics
}

// Shutdown stops everything.
func (s *Server) Shutdown() {
	s.metrics.WorkerCount().Set(0)

	// Stop cache janitor.
	s.workers.StopJanitor()
	s.limiter.Shutdown()
}

func (s *Server) findWorkerWithMaxFreeCPU() *Worker {
	var (
		mCPU   int32
		memory uint64
	)

	chosen := s.workers.Reduce(func(key string, val, chosen interface{}) interface{} {
		w := val.(*Worker)
		freeCPU := w.FreeCPU()
		freeMemory := w.FreeMemory()

		if chosen == nil || freeCPU > mCPU || (mCPU == freeCPU && freeMemory > memory) {
			chosen = w
			mCPU = freeCPU
			memory = freeMemory
		}
		return chosen
	})
	if chosen == nil {
		return nil
	}

	return chosen.(*Worker)
}

// Run runs script in secure environment of worker.
func (s *Server) Run(stream pb.ScriptRunner_RunServer) error {
	var (
		meta       *pb.RunMeta
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
		case *pb.RunRequest_Meta:
			meta = v.Meta
		case *pb.RunRequest_ScriptMeta:
			scriptMeta = v.ScriptMeta
		}

		if meta != nil && scriptMeta != nil {
			break
		}
	}

	if meta == nil || scriptMeta == nil {
		logger.Error("grpc:broker:Run error parsing input")
		return ErrInvalidArgument
	}

	return s.processRun(ctx, logger, stream, meta, scriptMeta)
}

func (s *Server) processRun(ctx context.Context, logger logrus.FieldLogger, stream pb.ScriptRunner_RunServer, // nolint: gocyclo
	runMeta *pb.RunMeta, scriptMeta *scriptpb.RunMeta) error {
	logger = logrus.WithFields(logrus.Fields{
		"meta":       runMeta,
		"runtime":    scriptMeta.Runtime,
		"sourceHash": scriptMeta.SourceHash,
		"entryPoint": scriptMeta.GetOptions().GetEntrypoint(),
		"async":      scriptMeta.GetOptions().GetAsync(),
		"mcpu":       scriptMeta.GetOptions().GetMcpu(),
		"userID":     scriptMeta.UserId,
	})
	start := time.Now()

	logger.Debug("grpc:lb:Run start")

	script := s.createScriptInfo(scriptMeta)
	retry := 0

	chunkReader := common.NewChunkReader(func() (*scriptpb.RunChunk, error) {
		req, err := stream.Recv()
		if err != nil {
			return nil, err
		}

		chunk := req.GetScriptChunk()
		if chunk == nil {
			return nil, ErrInvalidArgument
		}

		return chunk, nil
	})

	var (
		cont  *WorkerContainer
		resCh <-chan interface{}
	)

	_, err := util.RetryWithCritical(s.options.WorkerRetry, workerRetrySleep, func() (bool, error) {
		retry++

		// Check and refresh source and environment.
		if s.fileRepo.Get(scriptMeta.SourceHash) == "" || (scriptMeta.Environment != "" && s.fileRepo.Get(scriptMeta.Environment) == "") {
			return true, ErrSourceNotAvailable
		}

		// Grab worker.
		var (
			conns     int
			fromCache bool
		)
		cont, conns, fromCache = s.grabWorker(script)
		if cont == nil {
			return true, ErrNoWorkersAvailable
		}
		defer cont.Release()

		// Limiter.
		if conns == 1 && runMeta != nil && runMeta.ConcurrencyLimit > 0 {
			if err := s.limiter.Lock(ctx, runMeta.ConcurrencyKey, int(runMeta.ConcurrencyLimit)); err != nil {
				return true, status.Error(codes.ResourceExhausted, err.Error())
			}

			defer s.limiter.Unlock(runMeta.ConcurrencyKey, int(runMeta.ConcurrencyLimit))
		}

		logger = logger.WithFields(logrus.Fields{"container": cont, "script": &script, "try": retry, "fromCache": fromCache})

		var err error

		resCh, err = s.processWorkerRun(ctx, logger, cont, scriptMeta, chunkReader)
		if err != nil {
			logger.WithError(err).Warn("Worker processing Run failed")
			// Release worker resources as it failed prematurely.
			s.handleWorkerError(cont, err)

			return chunkReader.IsDirty() || util.IsCancellation(err), err
		}

		return false, nil
	})
	if err != nil {
		logger.WithError(err).Warn("grpc:lb:Run failed")
		return err
	}

	cont.Worker.ResetErrorCount()

	response, err := s.relayReponse(logger, stream, cont, resCh)

	if response != nil && response.Cached {
		// Add container to worker cache if we got any response.
		s.mu.Lock()
		cont.Worker.AddCache(s.workerContainerCache, script, response.ContainerId, cont)
		s.mu.Unlock()
	}

	logger.WithField("took", time.Since(start)).Info("grpc:lb:Run")

	return err
}

func (s *Server) relayReponse(logger logrus.FieldLogger, stream pb.ScriptRunner_RunServer, cont *WorkerContainer, resCh <-chan interface{}) (*scriptpb.RunResponse, error) {
	var response *scriptpb.RunResponse

	for res := range resCh {
		switch v := res.(type) {
		case *scriptpb.RunResponse:
			if response == nil {
				response = v
			}

			if err := stream.Send(v); err != nil {
				logger.WithField("res", v).Warn("Sending data error")

				return response, err
			}

		case error:
			// Retry on worker error.
			logger.WithError(v).Warn("Worker error")
			s.handleWorkerError(cont, v)

			return response, v
		}
	}

	return response, nil
}

func (s *Server) createScriptInfo(meta *scriptpb.RunMeta) ScriptInfo {
	script := ScriptInfo{
		SourceHash:  meta.SourceHash,
		Environment: meta.Environment,
		UserID:      meta.UserId,
		Async:       meta.GetOptions().GetAsync(),
		MCPU:        meta.GetOptions().GetMcpu(),
	}

	return script
}

func (s *Server) uploadResource(ctx context.Context, logger logrus.FieldLogger, cont *WorkerContainer, key string) error {
	exists, err := cont.Worker.Exists(ctx, key)
	if err != nil {
		logger.WithError(err).Warn("Worker grpc:repo:Exists failed")
		return err
	}

	if !exists.Ok {
		if err := cont.Upload(ctx, s.fileRepo.GetFS(), s.fileRepo.Get(key), key); err != nil {
			logger.WithError(err).Warn("Worker grpc:repo:Upload failed")
			return err
		}
	}

	return nil
}

func (s *Server) processWorkerRun(ctx context.Context, logger logrus.FieldLogger, cont *WorkerContainer,
	meta *scriptpb.RunMeta, chunkReader *common.ChunkReader) (<-chan interface{}, error) {
	for _, key := range []string{meta.SourceHash, meta.Environment} {
		if key != "" {
			if err := s.uploadResource(ctx, logger, cont, key); err != nil {
				return nil, err
			}
		}
	}

	return cont.Run(ctx, meta, chunkReader)
}

// grabWorker finds worker with container in cache and highest amount of free CPU.
// Fallback to any worker with highest amount of free CPU.
func (s *Server) grabWorker(script ScriptInfo) (*WorkerContainer, int, bool) { // nolint: gocyclo
	var (
		container *WorkerContainer
		conns     int
		fromCache bool

		workerMax *Worker
		freeCPU   int32
	)

	blacklist := make(map[string]struct{})

	s.mu.Lock()

	for {
		freeCPU = 0

		if m, ok := s.workerContainerCache[script]; ok {
			for _, cont := range m {
				w := cont.Worker
				cpu := w.FreeCPU()

				if _, blacklisted := blacklist[cont.ID]; blacklisted {
					continue
				}

				// Choose alive container with worker that has the highest free CPU or has free async connection.
				if s.workers.Get(cont.Worker.ID) != nil && (script.Async <= 1 && cpu > freeCPU) || (script.Async > 1 && cont.Conns() < script.Async) {
					fromCache = true
					container = cont
					freeCPU = cpu
				}
			}
		}

		// If no worker with cached container found - use container with max free cpu instead.
		if container == nil {
			workerMax = s.findWorkerWithMaxFreeCPU()

			if workerMax != nil {
				fromCache = false
				container = workerMax.NewContainer(script.Async, script.MCPU)
			} else {
				// If still cannot find a worker - abandon all hope.
				break
			}
		}

		if conns = container.Reserve(); conns > 0 {
			break
		} else if container.ID != "" {
			blacklist[container.ID] = struct{}{}
			container = nil
		}
	}

	s.mu.Unlock()

	return container, conns, fromCache
}

func (s *Server) handleWorkerError(cont *WorkerContainer, err error) {
	if util.IsCancellation(err) {
		return
	}

	if cont.Worker.IncreaseErrorCount() >= s.options.WorkerErrorThreshold {
		s.workers.Delete(cont.Worker.ID)
	}
}

// ContainerRemoved handles notifications sent by client whenever a container gets removed from cache.
func (s *Server) ContainerRemoved(ctx context.Context, in *pb.ContainerRemovedRequest) (*pb.ContainerRemovedResponse, error) {
	ci := ScriptInfo{SourceHash: in.SourceHash, Environment: in.Environment, UserID: in.UserId}

	ctx, reqID := util.AddDefaultRequestID(ctx)
	peerAddr := util.PeerAddr(ctx)
	logger := logrus.WithFields(logrus.Fields{"peer": peerAddr, "reqID": reqID})

	logger.WithFields(logrus.Fields{"id": in.GetId(), "container": &ci}).Debug("grpc:lb:ContainerRemoved")

	cur := s.workers.Get(in.Id)
	if cur == nil {
		return nil, ErrUnknownWorkerID
	}

	s.mu.Lock()
	cur.(*Worker).RemoveCache(s.workerContainerCache, ci, in.ContainerId)
	s.mu.Unlock()

	return &pb.ContainerRemovedResponse{}, nil
}

// Register is sent at the beginning by the worker.
func (s *Server) Register(ctx context.Context, in *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	// Increase worker count.
	s.metrics.WorkerCount().Add(1)

	peerAddr := util.PeerAddr(ctx)
	addr := net.TCPAddr{IP: peerAddr.(*net.TCPAddr).IP, Port: int(in.Port)}

	w := NewWorker(in.Id, addr, in.Mcpu, in.DefaultMcpu, in.Memory, s.metrics)

	logrus.WithFields(logrus.Fields{"worker": w, "mcpu": in.Mcpu, "defaultMCPU": in.DefaultMcpu, "memory": in.Memory}).Info("grpc:lb:Register")
	s.workers.Set(in.Id, w)

	return &pb.RegisterResponse{}, nil
}

// Heartbeat is meant to be called periodically by worker. If it's not sent for some time, worker will be removed.
func (s *Server) Heartbeat(ctx context.Context, in *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	w := s.workers.Get(in.Id)
	if w == nil {
		return nil, ErrUnknownWorkerID
	}

	w.(*Worker).Heartbeat(in.Memory)

	return &pb.HeartbeatResponse{}, nil
}

// Disconnect gracefully removes worker.
func (s *Server) Disconnect(ctx context.Context, in *pb.DisconnectRequest) (*pb.DisconnectResponse, error) {
	ctx, reqID := util.AddDefaultRequestID(ctx)
	peerAddr := util.PeerAddr(ctx)
	logger := logrus.WithFields(logrus.Fields{"peer": peerAddr, "reqID": reqID})

	logger.WithFields(logrus.Fields{"id": in.GetId()}).Info("grpc:lb:Disconnect")
	s.workers.Delete(in.Id)

	return &pb.DisconnectResponse{}, nil
}

func (s *Server) onEvictedWorkerHandler(key string, val interface{}) {
	// Decrease worker count.
	s.metrics.WorkerCount().Add(-1)

	w := val.(*Worker)

	s.mu.Lock()
	w.Shutdown(s.workerContainerCache)
	logrus.WithField("worker", w).Info("Worker removed")
	s.mu.Unlock()
}

// ReadyHandler returns 200 ok when worker number is satisfied.
func (s *Server) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	if s.metrics.WorkerCount().Value() >= int64(s.options.WorkerMinReady) {
		w.WriteHeader(200)
	} else {
		w.WriteHeader(400)
	}
}
