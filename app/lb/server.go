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

	"github.com/Syncano/codebox/app/common"
	"github.com/Syncano/codebox/app/filerepo"
	"github.com/Syncano/codebox/app/script"
	"github.com/Syncano/pkg-go/v2/cache"
	"github.com/Syncano/pkg-go/v2/limiter"
	"github.com/Syncano/pkg-go/v2/util"
	pb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/lb/v1"
	scriptpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"
)

// Server defines a Load Balancer server implementing both worker plug and script runner interface.
//go:generate go run github.com/vektra/mockery/cmd/mockery -dir ../../proto/gen/go/syncano/codebox/lb -all
type Server struct {
	mu                     sync.Mutex
	workers                *cache.LRUCache                        // workerID->Worker
	workerContainersCached map[string]map[string]*WorkerContainer // script.Definition hash->container ID->*WorkerContainer
	workersByIndex         map[string]map[string]int              // script.Index hash->Worker ID->ref count

	fileRepo filerepo.Repo
	options  *ServerOptions
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
	WorkerErrorThreshold uint

	DefaultScriptMcpu uint

	// Limiter
	LimiterConfig limiter.Config
}

// DefaultOptions holds default options values for LB server.
var DefaultOptions = ServerOptions{
	WorkerRetry:          3,
	WorkerKeepalive:      30 * time.Second,
	WorkerMinReady:       1,
	WorkerErrorThreshold: 2,

	DefaultScriptMcpu: 125,
	LimiterConfig:     limiter.DefaultConfig,
}

const (
	workerRetrySleep = 3 * time.Millisecond
)

// NewServer initializes new LB server.
func NewServer(fileRepo filerepo.Repo, opts *ServerOptions) *Server {
	options := DefaultOptions

	if opts != nil {
		_ = mergo.Merge(&options, opts, mergo.WithOverride)
	}

	workers := cache.NewLRUCache(true,
		cache.WithTTL(options.WorkerKeepalive),
	)
	s := &Server{
		options:                &options,
		workers:                workers,
		workerContainersCached: make(map[string]map[string]*WorkerContainer),
		workersByIndex:         make(map[string]map[string]int),
		fileRepo:               fileRepo,
		limiter: limiter.New(
			limiter.WithQueue(options.LimiterConfig.Queue),
			limiter.WithTTL(options.LimiterConfig.TTL),
		),
		metrics: Metrics(),
	}
	workers.OnValueEvicted(s.onEvictedWorkerHandler)

	return s
}

// Options returns a copy of LB options struct.
func (s *Server) Options() ServerOptions {
	return *s.options
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
		return common.ErrInvalidArgument
	}

	// Set defaults
	if scriptMeta.Options == nil {
		scriptMeta.Options = &scriptpb.RunMeta_Options{}
	}

	if scriptMeta.Options.Mcpu == 0 {
		scriptMeta.Options.Mcpu = uint32(s.options.DefaultScriptMcpu)
	}

	return s.processRun(ctx, logger, stream, meta, scriptMeta)
}

func (s *Server) processRun(ctx context.Context, logger logrus.FieldLogger, stream pb.ScriptRunner_RunServer, // nolint: gocyclo
	runMeta *pb.RunMeta, scriptMeta *scriptpb.RunMeta) error {
	logger = logger.WithFields(logrus.Fields{
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

	def := script.CreateDefinitionFromScriptMeta(scriptMeta)

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

	var (
		cont      *WorkerContainer
		resCh     <-chan interface{}
		conns     int
		fromCache bool
	)

	// Check and refresh source and environment.
	if s.fileRepo.Get(scriptMeta.SourceHash) == "" || (scriptMeta.Environment != "" && s.fileRepo.Get(scriptMeta.Environment) == "") {
		logger.Error("grpc:lb:Run source not available")
		return common.ErrSourceNotAvailable
	}

	for retry := 0; retry < s.options.WorkerRetry; retry++ {
		// Grab worker.
		cont, conns, fromCache = s.grabWorkerContainer(def)
		if cont == nil {
			logger.Warn("grpc:lb:Run no workers available")

			return ErrNoWorkersAvailable
		}

		defer cont.Release()

		// Limiter.
		var lockInfo *limiter.LockInfo

		if conns == 1 && runMeta != nil && runMeta.ConcurrencyLimit > 0 {
			var err error

			lockInfo, err = s.limiter.Lock(ctx, runMeta.ConcurrencyKey, int(runMeta.ConcurrencyLimit))
			if err != nil {
				logger.WithError(err).Warn("grpc:lb:Run Lock failed")
				return status.Error(codes.ResourceExhausted, err.Error())
			}

			defer s.limiter.Unlock(runMeta.ConcurrencyKey, int(runMeta.ConcurrencyLimit))
		}

		logger = logger.WithFields(logrus.Fields{"container": cont, "script": def, "try": retry + 1, "fromCache": fromCache, "lock": lockInfo})

		var err error

		resCh, err = s.processWorkerRun(ctx, logger, cont, fromCache, scriptMeta, chunkReader)
		if err != nil {
			logger.WithError(err).Warn("Worker processing Run failed")
			// Release worker resources as it failed prematurely.
			s.handleWorkerError(cont, err)

			if chunkReader.IsDirty() || util.IsCancellation(err) {
				logger.WithError(err).Warn("grpc:lb:Run failed")
				return err
			}

			// Retry
			time.Sleep(workerRetrySleep)

			continue
		}

		break
	}

	response, err := s.relayReponse(logger, stream, cont, resCh)

	if response != nil {
		cont.Worker.ResetErrorCount()

		if response.Cached {
			// Add container to worker cache if we got cached response.
			cont.ID = response.ContainerId
			s.addCache(cont, def)
		}
	}

	logger.WithFields(logrus.Fields{"took": time.Since(start), "mcpu": cont.mCPU}).Info("grpc:lb:Run")

	return err
}

func (s *Server) addCache(cont *WorkerContainer, def *script.Definition) {
	idx := def.Index.Hash()
	defHash := def.Hash()

	s.mu.Lock()
	cont.Worker.AddCache(def, cont)

	// Add container to script definition cache.
	if v, ok := s.workerContainersCached[defHash]; ok {
		v[cont.ID] = cont
	} else {
		s.workerContainersCached[defHash] = map[string]*WorkerContainer{cont.ID: cont}
	}

	// Increase script index for given worker ref count.
	if v, ok := s.workersByIndex[idx]; ok {
		v[cont.Worker.ID]++
	} else {
		s.workersByIndex[idx] = map[string]int{cont.Worker.ID: 1}
	}

	s.mu.Unlock()
}

func (s *Server) removeCache(w *Worker, def *script.Definition, containerID string) bool {
	idx := def.Index.Hash()
	defHash := def.Hash()
	removedContainer := false
	removedIdx := false

	s.mu.Lock()

	w.RemoveCache(def, containerID)

	// Remove worker container with specified definition from cache map.
	if m, ok := s.workerContainersCached[defHash]; ok {
		if _, ok = m[containerID]; ok {
			removedContainer = true

			delete(m, containerID)
		}

		if len(m) == 0 {
			delete(s.workerContainersCached, defHash)
		}
	}

	// Decrease refcount of script index for given worker.
	if m, ok := s.workersByIndex[idx]; ok {
		if v, ok := m[w.ID]; ok {
			removedIdx = true

			if v <= 1 {
				delete(m, w.ID)
			} else {
				m[w.ID]--
			}
		}

		if len(m) == 0 {
			delete(s.workersByIndex, idx)
		}
	}

	s.mu.Unlock()

	return removedContainer && removedIdx
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

func (s *Server) processWorkerRun(ctx context.Context, logger logrus.FieldLogger, cont *WorkerContainer, fromCache bool,
	meta *scriptpb.RunMeta, chunkReader *common.ChunkReader) (<-chan interface{}, error) {
	if !fromCache {
		for _, key := range []string{meta.SourceHash, meta.Environment} {
			if key != "" {
				if err := s.uploadResource(ctx, logger, cont, key); err != nil {
					return nil, err
				}
			}
		}
	}

	return cont.Run(ctx, meta, chunkReader)
}

// grabWorkerContainer finds worker with container in cache and highest amount of free CPU.
// Fallback to any worker with highest amount of free CPU.
func (s *Server) grabWorkerContainer(def *script.Definition) (*WorkerContainer, int, bool) { // nolint: gocyclo
	var (
		container *WorkerContainer
		conns     int
		fromCache bool

		workerMax *Worker
		freeCPU   int32
	)

	blacklist := make(map[string]struct{})

	key := def.Hash()

	s.mu.Lock()

	for {
		freeCPU = 0

		if m, ok := s.workerContainersCached[key]; ok {
			for _, cont := range m {
				w := cont.Worker
				cpu := w.FreeCPU()

				if _, blacklisted := blacklist[cont.ID]; blacklisted {
					continue
				}

				// Choose alive container with worker that has the highest free CPU or has free async connection.
				if s.workers.Get(cont.Worker.ID) != nil && (def.Async <= 1 && cpu > freeCPU) || (def.Async > 1 && cont.Conns() < def.Async) {
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
				container = workerMax.NewContainer(def.Async, def.MCPU)
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
	def := script.CreateDefinitionFromContainerRemove(in)

	ctx, reqID := util.AddDefaultRequestID(ctx)
	peerAddr := util.PeerAddr(ctx)
	logger := logrus.WithFields(logrus.Fields{"peer": peerAddr, "reqID": reqID, "id": in.GetId(), "script": def, "containerID": in.GetContainerId()})

	logger.Debug("grpc:lb:ContainerRemoved")

	cur := s.workers.Get(in.Id)
	if cur == nil {
		return nil, ErrUnknownWorkerID
	}

	if s.removeCache(cur.(*Worker), def, in.ContainerId) {
		logger.Info("Removed container from worker cache")
	} else {
		logger.Warn("Container not found in worker cache!")
	}

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

func (s *Server) Delete(ctx context.Context, req *scriptpb.DeleteRequest) (*scriptpb.DeleteResponse, error) {
	var (
		ret *scriptpb.DeleteResponse
		err error
	)

	ret = &scriptpb.DeleteResponse{}
	idx := &script.Index{
		Runtime:    req.Runtime,
		SourceHash: req.SourceHash,
		UserID:     req.UserId,
	}

	if m, ok := s.workersByIndex[idx.Hash()]; ok {
		for wID := range m {
			w := s.workers.Get(wID)
			if w == nil {
				continue
			}

			res, e := w.(*Worker).scriptCli.Delete(ctx, req)
			if e != nil {
				err = e
			} else {
				ret.ContainerIds = append(ret.ContainerIds, res.ContainerIds...)
			}
		}
	}

	return ret, err
}

func (s *Server) onEvictedWorkerHandler(key string, val interface{}) {
	// Decrease worker count.
	s.metrics.WorkerCount().Add(-1)

	w := val.(*Worker)

	s.mu.Lock()
	scriptDefs := w.Shutdown()

	for _, def := range scriptDefs {
		idx := def.Index.Hash()
		defHash := def.Hash()

		// Delete all containers cached for given worker+script definition pair.
		if m, ok := s.workerContainersCached[defHash]; ok {
			for contID, cont := range m {
				if cont.Worker == w {
					delete(m, contID)
				}
			}

			if len(m) == 0 {
				delete(s.workerContainersCached, defHash)
			}
		}

		// Delete script index entry for worker.
		if m, ok := s.workersByIndex[idx]; ok {
			delete(m, w.ID)

			if len(m) == 0 {
				delete(s.workersByIndex, idx)
			}
		}
	}

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
