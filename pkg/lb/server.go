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
//go:generate mockery -dir proto -all
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
var DefaultOptions = ServerOptions{
	WorkerRetry:          3,
	WorkerKeepalive:      30 * time.Second,
	WorkerErrorThreshold: 2,
}

var (
	initOnce         sync.Once
	workerCounter    *expvar.Int
	freeSlotsCounter *expvar.Int
	expvarCollector  = prometheus.NewExpvarCollector(map[string]*prometheus.Desc{
		"workers": prometheus.NewDesc(
			"codebox_workers",
			"Codebox workers connected.",
			nil, nil,
		),
		"slots": prometheus.NewDesc(
			"codebox_slots",
			"Codebox free slots.",
			nil, nil,
		),
	})
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
func NewServer(fileRepo filerepo.Repo, options ServerOptions) *Server {
	// Register expvar and prometheus exports.
	initOnce.Do(func() {
		workerCounter = expvar.NewInt("workers")
		freeSlotsCounter = expvar.NewInt("slots")
		prometheus.MustRegister(
			expvarCollector,
			workerMemory,
		)
	})
	mergo.Merge(&options, DefaultOptions) // nolint - error not possible

	workers := cache.NewLRUCache(cache.Options{
		TTL: options.WorkerKeepalive,
	}, cache.LRUOptions{
		AutoRefresh: false,
	})
	s := &Server{
		options:              options,
		workers:              workers,
		workerContainerCache: make(ContainerWorkerCache),
		fileRepo:             fileRepo,
		limiter:              limiter.New(limiter.Options{}),
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

func (s *Server) findWorkerWithMaxFreeSlots() (*Worker, int) {
	var slots int
	var memory uint64

	chosen := s.workers.Reduce(func(key string, val, chosen interface{}) interface{} {
		w := val.(*Worker)
		freeSlots := w.FreeSlots()
		availableMemory := w.AvailableMemory()

		if chosen == nil || freeSlots > slots || (slots == freeSlots && availableMemory > memory) {
			chosen = w
			slots = freeSlots
			memory = availableMemory
		}
		return chosen
	})
	if chosen == nil {
		return nil, 0
	}
	return chosen.(*Worker), slots
}

// Run runs script in secure environment of worker.
func (s *Server) Run(stream pb.ScriptRunner_RunServer) error {
	var runMeta *pb.RunRequest_MetaMessage
	var scriptMeta *scriptpb.RunRequest_MetaMessage
	var scriptChunk []*scriptpb.RunRequest_ChunkMessage

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

	return s.processRun(stream, runMeta, scriptMeta, scriptChunk)
}

func (s *Server) processRun(stream pb.ScriptRunner_RunServer, runMeta *pb.RunRequest_MetaMessage,
	scriptMeta *scriptpb.RunRequest_MetaMessage, scriptChunk []*scriptpb.RunRequest_ChunkMessage) error {

	ctx := stream.Context()
	peerAddr := util.PeerAddr(ctx)
	logger := logrus.WithFields(logrus.Fields{
		"peer":       peerAddr,
		"meta":       runMeta,
		"runtime":    scriptMeta.Runtime,
		"sourceHash": scriptMeta.SourceHash,
		"userID":     scriptMeta.UserID,
	})
	logger.Debug("grpc:lb:Run start")
	start := time.Now()

	if runMeta != nil && runMeta.ConcurrencyLimit >= 0 {
		if err := s.limiter.Lock(ctx, runMeta.ConcurrencyKey, int(runMeta.ConcurrencyLimit)); err != nil {
			logger.WithError(err).Warn("Lock error")
			return status.Error(codes.ResourceExhausted, err.Error())
		}

		defer s.limiter.Unlock(runMeta.ConcurrencyKey, int(runMeta.ConcurrencyLimit))
	}

	ci := ScriptInfo{SourceHash: scriptMeta.SourceHash, Environment: scriptMeta.Environment, UserID: scriptMeta.UserID}
	retry := 0

	if runErr := util.RetryWithCritical(s.options.WorkerRetry, workerRetrySleep, func() (bool, error) {
		retry++
		worker, fromCache := s.grabWorker(ci)
		if worker == nil {
			return true, ErrNoWorkersAvailable
		}
		logger = logger.WithFields(logrus.Fields{"worker": worker, "container": &ci, "try": retry, "fromCache": fromCache})

		// Check and refresh source and environment.
		if s.fileRepo.Get(scriptMeta.SourceHash) == "" || (scriptMeta.Environment != "" && s.fileRepo.Get(scriptMeta.Environment) == "") {
			return true, ErrSourceNotAvailable
		}

		resCh, err := s.processWorkerRun(ctx, logger, worker, scriptMeta, scriptChunk)
		if err != nil {
			logger.WithError(err).Warn("Worker processing Run failed")
			// Release worker slot as it failed prematurely.
			worker.ReleaseSlot()
			freeSlotsCounter.Add(1)
			s.handleWorkerError(worker, err)
			return err == context.Canceled, err
		}
		worker.ResetErrorCount()

		cached := false
		for res := range resCh {
			switch v := res.(type) {
			case *scriptpb.RunResponse:
				cached = v.Cached
				if err = stream.Send(v); err != nil {
					logger.WithField("res", v).Warn("Sending data error")
					return true, err
				}

			case error:
				// Retry on worker error.
				logger.WithError(v).Warn("Worker error")
				s.handleWorkerError(worker, v)
				return false, v
			}
		}

		if cached {
			// Add container to worker cache if we got any response.
			s.mu.Lock()
			worker.AddContainer(ci, s.workerContainerCache)
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

func (s *Server) uploadResource(ctx context.Context, logger logrus.FieldLogger, worker *Worker, key string) error {
	exists, err := worker.Exists(ctx, key)
	if err != nil {
		logger.WithError(err).Warn("Worker grpc:repo:Exists failed")
		return err
	}

	if !exists.Ok {
		if err := worker.Upload(ctx, s.fileRepo.GetFS(), s.fileRepo.Get(key), key); err != nil {
			logger.WithError(err).Warn("Worker grpc:repo:Upload failed")
			return err
		}
	}
	return nil

}

func (s *Server) processWorkerRun(ctx context.Context, logger logrus.FieldLogger, worker *Worker,
	meta *scriptpb.RunRequest_MetaMessage, chunk []*scriptpb.RunRequest_ChunkMessage) (<-chan interface{}, error) {

	for _, key := range []string{meta.SourceHash, meta.Environment} {
		if key != "" {
			if err := s.uploadResource(ctx, logger, worker, key); err != nil {
				return nil, err
			}
		}
	}

	return worker.Run(ctx, meta, chunk)
}

// grabWorker finds worker with container in cache and highest amount of free slots.
// Fallback to any worker with highest amount of free slots.
func (s *Server) grabWorker(ci ScriptInfo) (*Worker, bool) {
	var (
		worker, workerMax       *Worker
		fromCache               bool
		freeSlots, freeSlotsMax int
	)

	s.mu.Lock()
	for {
		freeSlots = 0
		if set, ok := s.workerContainerCache[ci]; ok {

			for _, v := range set.Values() {
				w := v.(*Worker)
				slots := w.FreeSlots()

				if (worker == nil || slots > freeSlots) && s.workers.Contains(w.ID) && w.Alive() {
					fromCache = true
					worker = w
					freeSlots = slots
				}
			}
		}

		// If no worker with cached container found or if there is a big discrepancy between worker with max free slots
		// and worker with cached container - use that instead to balance the load.
		workerMax, freeSlotsMax = s.findWorkerWithMaxFreeSlots()
		if worker == nil || freeSlotsMax > 2*freeSlots {
			fromCache = false
			worker, freeSlots = workerMax, freeSlotsMax
		}

		// If still cannot find a worker - abandon all hope.
		if worker == nil {
			break
		}
		// If free slots are greater than 0, only allow grabbing slot if there are still > 0 slots.
		requireSlot := freeSlots > 0
		if s.workers.Get(worker.ID) != nil && worker.GrabSlot(requireSlot) {
			freeSlotsCounter.Add(-1)
			if fromCache {
				worker.RemoveContainer(ci, s.workerContainerCache)
			}
			break
		}
	}
	s.mu.Unlock()
	return worker, fromCache
}

func (s *Server) handleWorkerError(worker *Worker, err error) {
	if err == context.Canceled {
		return
	}
	if worker.IncreaseErrorCount() >= s.options.WorkerErrorThreshold {
		s.workers.Delete(worker.ID)
	}
}

// SlotReady handles notifications sent by client whenever a slot is ready to accept connections.
func (s *Server) SlotReady(ctx context.Context, in *pb.SlotReadyRequest) (*pb.SlotReadyResponse, error) {
	peerAddr := util.PeerAddr(ctx)
	logrus.WithFields(logrus.Fields{"id": in.GetId(), "peer": peerAddr}).Debug("grpc:lb:SlotReady")

	cur := s.workers.Get(in.Id)
	if cur == nil {
		return nil, ErrUnknownWorkerID
	}

	cur.(*Worker).IncFreeSlots()
	freeSlotsCounter.Add(1)
	return &pb.SlotReadyResponse{}, nil
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
	cur.(*Worker).RemoveContainer(ci, s.workerContainerCache)
	s.mu.Unlock()
	return &pb.ContainerRemovedResponse{}, nil
}

// Register is sent at the beginning by the worker.
func (s *Server) Register(ctx context.Context, in *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	workerCounter.Add(1) // Increase worker count.
	peerAddr := util.PeerAddr(ctx)
	addr := net.TCPAddr{IP: peerAddr.(*net.TCPAddr).IP, Port: int(in.Port)}

	freeSlotsCounter.Add(int64(in.Concurrency))
	w := NewWorker(in.Id, addr, in.Concurrency, in.Memory)
	logrus.WithField("worker", w).Info("grpc:lb:Register")
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
	peerAddr := util.PeerAddr(ctx)
	logrus.WithFields(logrus.Fields{"id": in.GetId(), "peer": peerAddr}).Info("grpc:lb:Disconnect")

	s.workers.Delete(in.Id)
	return &pb.DisconnectResponse{}, nil
}

func (s *Server) onEvictedWorkerHandler(key string, val interface{}) {
	workerMemory.DeleteLabelValues(key)
	workerCounter.Add(-1) // Decrease worker count.
	w := val.(*Worker)
	freeSlotsCounter.Add(-int64(w.FreeSlots()))

	s.mu.Lock()
	w.Shutdown(s.workerContainerCache)
	logrus.WithField("worker", w).Info("Worker removed")
	s.mu.Unlock()
}

// ReadyHandler returns 200 ok when worker number is satisfied.
func (s *Server) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	if workerCounter.Value() >= int64(s.options.WorkerMinReady) {
		w.WriteHeader(200)
	} else {
		w.WriteHeader(400)
	}
}
