package lb

import (
	"context"
	"expvar"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/imdario/mergo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Syncano/codebox/pkg/cache"
	"github.com/Syncano/codebox/pkg/filerepo"
	pb "github.com/Syncano/codebox/pkg/lb/proto"
	"github.com/Syncano/codebox/pkg/limiter"
	scriptpb "github.com/Syncano/codebox/pkg/script/proto"
	"github.com/Syncano/codebox/pkg/util"
)

// Server defines a Load Balancer server implementing both worker plug and script runner interface.
//go:generate go run github.com/vektra/mockery/cmd/mockery -dir proto -all
type Server struct {
	mu                   sync.Mutex
	workers              *cache.LRUCache // workerID->Worker
	workerContainerCache ContainerWorkerCache

	fileRepo filerepo.Repo
	options  ServerOptions
	limiter  *limiter.Limiter
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
}

// DefaultOptions holds default options values for LB server.
var DefaultOptions = &ServerOptions{
	WorkerRetry:          3,
	WorkerKeepalive:      30 * time.Second,
	WorkerErrorThreshold: 2,
}

var (
	initOnce        sync.Once
	workerCounter   *expvar.Int
	freeCPUCounter  *expvar.Int
	expvarCollector = prometheus.NewExpvarCollector(map[string]*prometheus.Desc{
		"workers": prometheus.NewDesc(
			"codebox_workers",
			"Codebox workers connected.",
			nil, nil,
		),
		"cpu": prometheus.NewDesc(
			"codebox_cpu",
			"Codebox free mcpu.",
			nil, nil,
		),
		"memory": prometheus.NewDesc(
			"codebox_memory",
			"Codebox free memory.",
			nil, nil,
		),
	})
	workerCPU = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "worker_cpu",
		Help: "Available mCPU of worker.",
	},
		[]string{"id"},
	)
	workerMemory = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "worker_memory",
		Help: "Available memory of worker.",
	},
		[]string{"id"},
	)

	// ErrUnknownWorkerID signals that we received a message for unknown worker.
	ErrUnknownWorkerID = status.Error(codes.InvalidArgument, "unknown worker id")
	// ErrNoWorkersAvailable signals that there are no suitable workers at this moment.
	ErrNoWorkersAvailable = status.Error(codes.ResourceExhausted, "no workers available")
	// ErrSourceNotAvailable signals that specified source hash was not found.
	ErrSourceNotAvailable = status.Error(codes.FailedPrecondition, "source not available")
)

const (
	workerRetrySleep = 3 * time.Millisecond
)

// NewServer initializes new LB server.
func NewServer(fileRepo filerepo.Repo, options *ServerOptions) *Server {
	// Register expvar and prometheus exports.
	initOnce.Do(func() {
		workerCounter = expvar.NewInt("workers")
		freeCPUCounter = expvar.NewInt("cpu")
		prometheus.MustRegister(
			expvarCollector,
			workerCPU,
			workerMemory,
		)
	})
	mergo.Merge(options, DefaultOptions) // nolint - error not possible

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
	}
	workers.OnValueEvicted(s.onEvictedWorkerHandler)

	return s
}

// Options returns a copy of LB options struct.
func (s *Server) Options() ServerOptions {
	return s.options
}

// Shutdown stops everything.
func (s *Server) Shutdown() {
	workerCounter.Set(0)

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
		runMeta     *pb.RunRequest_MetaMessage
		scriptMeta  *scriptpb.RunRequest_MetaMessage
		scriptChunk []*scriptpb.RunRequest_ChunkMessage
	)

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
			runMeta = v.Meta
		case *pb.RunRequest_Request:
			switch r := v.Request.Value.(type) {
			case *scriptpb.RunRequest_Meta:
				scriptMeta = r.Meta
			case *scriptpb.RunRequest_Chunk:
				scriptChunk = append(scriptChunk, r.Chunk)
			}
		}
	}

	if scriptMeta == nil {
		return nil
	}

	if runMeta == nil {
		runMeta = &pb.RunRequest_MetaMessage{}
	}

	if runMeta.RequestID == "" {
		runMeta.RequestID = util.GenerateShortKey()
	}

	scriptMeta.RequestID = runMeta.RequestID

	return s.processRun(stream, runMeta, scriptMeta, scriptChunk)
}

func (s *Server) processRun(stream pb.ScriptRunner_RunServer, runMeta *pb.RunRequest_MetaMessage, // nolint: gocyclo
	scriptMeta *scriptpb.RunRequest_MetaMessage, scriptChunk []*scriptpb.RunRequest_ChunkMessage) error {
	ctx := stream.Context()
	peerAddr := util.PeerAddr(ctx)
	logger := logrus.WithFields(logrus.Fields{
		"reqID":      runMeta.RequestID,
		"peer":       peerAddr,
		"meta":       runMeta,
		"runtime":    scriptMeta.Runtime,
		"sourceHash": scriptMeta.SourceHash,
		"entryPoint": scriptMeta.GetOptions().GetEntryPoint(),
		"async":      scriptMeta.GetOptions().GetAsync(),
		"mcpu":       scriptMeta.GetOptions().GetMCPU(),
		"userID":     scriptMeta.UserID,
	})
	start := time.Now()

	logger.Debug("grpc:lb:Run start")

	if runMeta != nil && runMeta.ConcurrencyLimit > 0 {
		if err := s.limiter.Lock(ctx, runMeta.ConcurrencyKey, int(runMeta.ConcurrencyLimit)); err != nil {
			logger.WithError(err).Warn("Lock error")
			return status.Error(codes.ResourceExhausted, err.Error())
		}

		defer s.limiter.Unlock(runMeta.ConcurrencyKey, int(runMeta.ConcurrencyLimit))
	}

	script := s.createScriptInfo(scriptMeta)
	retry := 0

	if runErr := util.RetryWithCritical(s.options.WorkerRetry, workerRetrySleep, func() (bool, error) {
		retry++

		// Check and refresh source and environment.
		if s.fileRepo.Get(scriptMeta.SourceHash) == "" || (scriptMeta.Environment != "" && s.fileRepo.Get(scriptMeta.Environment) == "") {
			return true, ErrSourceNotAvailable
		}

		// Grab worker.
		cont, fromCache := s.grabWorker(script)
		if cont == nil {
			return true, ErrNoWorkersAvailable
		}

		logger = logger.WithFields(logrus.Fields{"container": cont, "script": &script, "try": retry, "fromCache": fromCache})

		resCh, err := s.processWorkerRun(ctx, logger, cont, scriptMeta, scriptChunk)
		defer cont.Release()

		if err != nil {
			logger.WithError(err).Warn("Worker processing Run failed")
			// Release worker resources as it failed prematurely.
			s.handleWorkerError(cont, err)

			return err == context.Canceled, err
		}

		cont.Worker.ResetErrorCount()

		var response *scriptpb.RunResponse
		for res := range resCh {
			switch v := res.(type) {
			case *scriptpb.RunResponse:
				if response == nil {
					response = v
				}
				if err = stream.Send(v); err != nil {
					logger.WithField("res", v).Warn("Sending data error")
					return true, err
				}

			case error:
				// Retry on worker error.
				logger.WithError(v).Warn("Worker error")
				s.handleWorkerError(cont, v)
				return false, v
			}
		}

		if response != nil && response.Cached {
			// Add container to worker cache if we got any response.
			s.mu.Lock()
			cont.Worker.AddCache(s.workerContainerCache, script, response.ContainerID, cont)
			s.mu.Unlock()
		}

		logger.WithField("took", time.Since(start)).Info("grpc:lb:Run")

		return false, nil
	}); runErr != nil {
		logger.WithError(runErr).Warn("grpc:lb:Run failed")
		return runErr
	}

	return nil
}

func (s *Server) createScriptInfo(meta *scriptpb.RunRequest_MetaMessage) ScriptInfo {
	script := ScriptInfo{
		SourceHash:  meta.SourceHash,
		Environment: meta.Environment,
		UserID:      meta.UserID,
		Async:       meta.GetOptions().GetAsync(),
		MCPU:        meta.GetOptions().GetMCPU(),
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
	meta *scriptpb.RunRequest_MetaMessage, chunk []*scriptpb.RunRequest_ChunkMessage) (<-chan interface{}, error) {
	for _, key := range []string{meta.SourceHash, meta.Environment} {
		if key != "" {
			if err := s.uploadResource(ctx, logger, cont, key); err != nil {
				return nil, err
			}
		}
	}

	return cont.Run(ctx, meta, chunk)
}

// grabWorker finds worker with container in cache and highest amount of free CPU.
// Fallback to any worker with highest amount of free CPU.
func (s *Server) grabWorker(script ScriptInfo) (*WorkerContainer, bool) { // nolint: gocyclo
	var (
		workerMax *Worker
		container *WorkerContainer
		fromCache bool
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
				if (script.Async <= 1 && cpu > freeCPU) || (script.Async > 1 && cont.Conns() < script.Async) {
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

		if s.workers.Get(container.Worker.ID) != nil && container.Reserve() {
			break
		} else {
			blacklist[container.ID] = struct{}{}
			container = nil
		}
	}

	s.mu.Unlock()

	return container, fromCache
}

func (s *Server) handleWorkerError(cont *WorkerContainer, err error) {
	if err == context.Canceled {
		return
	}

	if cont.Worker.IncreaseErrorCount() >= s.options.WorkerErrorThreshold {
		s.workers.Delete(cont.Worker.ID)
	}
}

// ContainerRemoved handles notifications sent by client whenever a container gets removed from cache.
func (s *Server) ContainerRemoved(ctx context.Context, in *pb.ContainerRemovedRequest) (*pb.ContainerRemovedResponse, error) {
	peerAddr := util.PeerAddr(ctx)
	ci := ScriptInfo{SourceHash: in.SourceHash, Environment: in.Environment, UserID: in.UserID}

	logrus.WithFields(logrus.Fields{"id": in.GetId(), "container": &ci, "peer": peerAddr}).Debug("grpc:lb:ContainerRemoved")

	cur := s.workers.Get(in.Id)
	if cur == nil {
		return nil, ErrUnknownWorkerID
	}

	s.mu.Lock()
	cur.(*Worker).RemoveCache(s.workerContainerCache, ci, in.ContainerID)
	s.mu.Unlock()

	return &pb.ContainerRemovedResponse{}, nil
}

// Register is sent at the beginning by the worker.
func (s *Server) Register(ctx context.Context, in *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	workerCounter.Add(1) // Increase worker count.

	peerAddr := util.PeerAddr(ctx)
	addr := net.TCPAddr{IP: peerAddr.(*net.TCPAddr).IP, Port: int(in.Port)}

	workerMemory.WithLabelValues(in.Id).Set(float64(in.Memory))
	workerCPU.WithLabelValues(in.Id).Set(float64(in.MCPU))

	w := NewWorker(in.Id, addr, in.MCPU, in.DefaultMCPU, in.Memory)

	logrus.WithFields(logrus.Fields{"worker": w, "mcpu": in.MCPU, "defaultMCPU": in.DefaultMCPU, "memory": in.Memory}).Info("grpc:lb:Register")
	s.workers.Set(in.Id, w)

	return &pb.RegisterResponse{}, nil
}

// Heartbeat is meant to be called periodically by worker. If it's not sent for some time, worker will be removed.
func (s *Server) Heartbeat(ctx context.Context, in *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	w := s.workers.Get(in.Id)
	if w == nil {
		return nil, ErrUnknownWorkerID
	}

	workerMemory.WithLabelValues(in.Id).Set(float64(in.Memory))
	w.(*Worker).Heartbeat(in.Memory)

	return &pb.HeartbeatResponse{}, nil
}

// Disconnect gracefully removes worker.
func (s *Server) Disconnect(ctx context.Context, in *pb.DisconnectRequest) (*pb.DisconnectResponse, error) {
	logrus.WithFields(logrus.Fields{"id": in.GetId(), "peer": util.PeerAddr(ctx)}).Info("grpc:lb:Disconnect")

	s.workers.Delete(in.Id)

	return &pb.DisconnectResponse{}, nil
}

func (s *Server) onEvictedWorkerHandler(key string, val interface{}) {
	workerCounter.Add(-1) // Decrease worker count.

	w := val.(*Worker)

	s.mu.Lock()
	w.Shutdown(s.workerContainerCache)
	logrus.WithField("worker", w).Info("Worker removed")
	s.mu.Unlock()

	workerCPU.DeleteLabelValues(key)
	workerMemory.DeleteLabelValues(key)
}

// ReadyHandler returns 200 ok when worker number is satisfied.
func (s *Server) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	if workerCounter.Value() >= int64(s.options.WorkerMinReady) {
		w.WriteHeader(200)
	} else {
		w.WriteHeader(400)
	}
}
