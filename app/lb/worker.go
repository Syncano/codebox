package lb

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/spf13/afero"
	"go.opencensus.io/plugin/ocgrpc"
	"google.golang.org/grpc"

	"github.com/Syncano/codebox/app/common"
	"github.com/Syncano/codebox/app/script"
	"github.com/Syncano/pkg-go/util"
	repopb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/filerepo/v1"
	scriptpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"
)

// Worker holds basic info about the worker.
type Worker struct {
	ID   string
	Addr net.TCPAddr

	conn        *grpc.ClientConn
	waitGroup   sync.WaitGroup
	mCPU        uint32
	defaultMCPU uint32
	memory      uint64

	freeCPU           int32
	freeMemory        uint64
	mu                sync.RWMutex
	alive             bool
	repoCli           repopb.RepoClient
	scriptCli         scriptpb.ScriptRunnerClient
	scriptsRefCounter map[string]int                // script definition hash -> ref count
	scripts           map[string]*script.Definition // script definition hash -> definition
	containers        map[string]*WorkerContainer   // container ID -> worker container
	metrics           *MetricsData

	// These are processed atomically.
	errorCount uint32
}

const (
	chunkSize = 2 * 1024 * 1024
)

// NewWorker initializes new worker info along with worker connection.
func NewWorker(id string, addr net.TCPAddr, mCPU, defaultMCPU uint32, memory uint64, metrics *MetricsData) *Worker {
	conn, err := grpc.Dial(addr.String(),
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(common.MaxGRPCMessageSize)),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
	)
	util.Must(err)

	w := Worker{
		ID:   id,
		Addr: addr,

		alive:             true,
		scripts:           make(map[string]*script.Definition),
		scriptsRefCounter: make(map[string]int),
		containers:        make(map[string]*WorkerContainer),
		conn:              conn,
		mCPU:              mCPU,
		defaultMCPU:       defaultMCPU,
		freeMemory:        memory,
		freeCPU:           int32(mCPU),
		metrics:           metrics,

		repoCli:   repopb.NewRepoClient(conn),
		scriptCli: scriptpb.NewScriptRunnerClient(conn),
	}

	w.metrics.WorkerCPU().Add(int64(mCPU))

	return &w
}

func (w *Worker) String() string {
	return fmt.Sprintf("{ID:%s, Addr:%v}", w.ID, w.Addr)
}

// FreeCPU returns free CPU of worker (in millicpus).
func (w *Worker) FreeCPU() int32 {
	return atomic.LoadInt32(&w.freeCPU)
}

// FreeMemory returns free worker memory.
func (w *Worker) FreeMemory() uint64 {
	return atomic.LoadUint64(&w.freeMemory)
}

// Alive returns true if worker is alive.
func (w *Worker) Alive() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.alive
}

// Reserve checks if worker is alive, decreases resources and increases waitgroup.
// If require is true, returns true only if resources are available.
func (w *Worker) reserve(mCPU, conns uint32, require bool) bool {
	// Async connection - only first one should reserve resources.
	if conns > 1 && w.Alive() {
		w.waitGroup.Add(1)
		return true
	}

	w.mu.Lock()

	if !w.alive || (require && w.freeCPU < int32(mCPU)) {
		w.mu.Unlock()
		return false
	}

	w.freeCPU -= int32(mCPU)
	w.waitGroup.Add(1)
	w.mu.Unlock()

	w.metrics.WorkerCPU().Add(-int64(mCPU))

	return true
}

// Release resources reserved.
func (w *Worker) release(mCPU, conns uint32) {
	if conns != 0 {
		w.waitGroup.Done()
		return
	}

	w.mu.Lock()
	if w.alive {
		w.freeCPU += int32(mCPU)
		w.metrics.WorkerCPU().Add(int64(mCPU))
	}
	w.mu.Unlock()

	w.waitGroup.Done()
}

// IncreaseErrorCount increases error count of worker.
func (w *Worker) IncreaseErrorCount() uint {
	return uint(atomic.AddUint32(&w.errorCount, 1))
}

// ResetErrorCount resets error count of worker.
func (w *Worker) ResetErrorCount() {
	atomic.StoreUint32(&w.errorCount, 0)
}

// Heartbeat updates worker stats.
func (w *Worker) Heartbeat(memory uint64) {
	atomic.StoreUint64(&w.memory, memory)
}

// Exists calls worker RPC and checks if file exists.
func (w *Worker) Exists(ctx context.Context, key string) (*repopb.ExistsResponse, error) {
	return w.repoCli.Exists(ctx, &repopb.ExistsRequest{Key: key})
}

func (w *Worker) NewContainer(async, mcpu uint32) *WorkerContainer {
	if mcpu == 0 {
		mcpu = w.defaultMCPU
	}

	return &WorkerContainer{Worker: w, async: async, mCPU: mcpu}
}

func uploadDir(stream repopb.Repo_UploadClient, fs afero.Fs, key, sourcePath string) error {
	var err error

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
		return nil
	}

	if err := afero.Walk(fs, sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			name, _ := filepath.Rel(sourcePath, path) // nolint - error not possible
			file, err := fs.Open(path)
			if err != nil {
				return err
			}

			buf := make([]byte, chunkSize)
			for {
				n, e := file.Read(buf)
				if e == io.EOF {
					break
				}

				if err := stream.Send(&repopb.UploadRequest{
					Value: &repopb.UploadRequest_Chunk{
						Chunk: &repopb.UploadChunkMessage{Name: name, Data: buf[:n]},
					},
				}); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// Send done flag.
	if err := stream.Send(&repopb.UploadRequest{Value: &repopb.UploadRequest_Done{Done: true}}); err != nil {
		return err
	}
	// Wait for response as a confirmation of finished upload.
	if _, err := stream.Recv(); err != nil {
		return err
	}

	return nil
}

// AddCache increases ref count of Container in Worker.
func (w *Worker) AddCache(def *script.Definition, container *WorkerContainer) {
	defHash := def.Hash()

	w.mu.Lock()

	w.scriptsRefCounter[defHash]++
	w.scripts[defHash] = def
	w.containers[container.ID] = container

	w.mu.Unlock()
}

// RemoveCache decreases ref count of Container in Worker.
func (w *Worker) RemoveCache(def *script.Definition, contID string) bool {
	defHash := def.Hash()

	w.mu.Lock()

	delete(w.containers, contID)

	ref := w.scriptsRefCounter[defHash]
	if ref > 1 {
		w.scriptsRefCounter[defHash]--
		w.scripts[defHash] = def
	} else {
		delete(w.scriptsRefCounter, defHash)
		delete(w.scripts, defHash)

		w.mu.Unlock()

		return true
	}

	w.mu.Unlock()

	return false
}

// Shutdown stops connection when drained and returns cached Definitions.
func (w *Worker) Shutdown() []*script.Definition {
	w.mu.Lock()

	ret := make([]*script.Definition, len(w.scripts))

	w.alive = false

	i := 0

	for _, scriptHash := range w.scripts {
		ret[i] = scriptHash

		i++
	}

	w.metrics.WorkerCPU().Add(-int64(w.freeCPU))
	w.mu.Unlock()

	// Wait for all calls to finish and close connection in goroutine.
	go func() {
		w.waitGroup.Wait()

		w.conn.Close() // nolint
	}()

	return ret
}

// WorkerContainer defines worker container info.
type WorkerContainer struct {
	Worker *Worker

	ID    string
	conns uint32
	mCPU  uint32
	async uint32
}

// Conns returns number of current connections.
func (w *WorkerContainer) Conns() uint32 {
	return atomic.LoadUint32(&w.conns)
}

// Upload calls worker RPC and uploads file(s) to it.
func (w *WorkerContainer) Upload(ctx context.Context, fs afero.Fs, sourcePath, key string) error {
	stream, err := w.Worker.repoCli.Upload(ctx)
	if err != nil {
		return err
	}

	// Iterate through download results and upload them.
	if err = uploadDir(stream, fs, key, sourcePath); err != nil {
		if exists, e := w.Worker.repoCli.Exists(ctx, &repopb.ExistsRequest{Key: key}); e == nil && exists.Ok {
			return nil
		}
	}

	return err
}

// Run calls worker RPC and runs script there.
func (w *WorkerContainer) Run(ctx context.Context, meta *scriptpb.RunMeta, chunkReader *common.ChunkReader) (<-chan interface{}, error) {
	workerStream, err := w.Worker.scriptCli.Run(ctx)
	if err != nil {
		return nil, err
	}

	// Send meta header.
	if err := workerStream.Send(&scriptpb.RunRequest{
		Value: &scriptpb.RunRequest_Meta{
			Meta: meta,
		},
	}); err != nil {
		return nil, err
	}

	// Send chunks.
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

			if err = workerStream.Send(&scriptpb.RunRequest{
				Value: &scriptpb.RunRequest_Chunk{
					Chunk: chunk,
				},
			}); err != nil {
				err = fmt.Errorf("sending script request failed: %w", err)
				break
			}
		}

		if err != nil {
			return nil, err
		}
	}

	if err := workerStream.CloseSend(); err != nil {
		return nil, err
	}

	ch := make(chan interface{}, 1)

	go func() {
		for {
			runRes, err := workerStream.Recv()
			if err != nil {
				if err != io.EOF {
					ch <- err
				}

				close(ch)

				return
			}
			ch <- runRes
		}
	}()

	return ch, nil
}

// Reserve calls reserve on worker and increases connection count if successful. Returns new connection count.
func (w *WorkerContainer) Reserve() int {
	conns := atomic.AddUint32(&w.conns, 1)

	// Disallow reservation higher than async value in async mode.
	if w.async > 1 && conns > w.async {
		atomic.AddUint32(&w.conns, ^uint32(0))
		return -1
	}

	// If CPU requirements are met, require reservation to be successful.
	require := w.Worker.FreeCPU() > int32(w.mCPU)

	if !w.Worker.reserve(w.mCPU, conns, require) {
		atomic.AddUint32(&w.conns, ^uint32(0))
		return -1
	}

	return int(conns)
}

// Release resources reserved.
func (w *WorkerContainer) Release() {
	conns := atomic.AddUint32(&w.conns, ^uint32(0))

	w.Worker.release(w.mCPU, conns)
}
