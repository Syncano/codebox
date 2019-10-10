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
	"google.golang.org/grpc"

	repopb "github.com/Syncano/codebox/pkg/filerepo/proto"
	scriptpb "github.com/Syncano/codebox/pkg/script/proto"
	"github.com/Syncano/codebox/pkg/sys"
	"github.com/Syncano/codebox/pkg/util"
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

	freeCPU    int32
	freeMemory uint64
	mu         sync.RWMutex
	alive      bool
	repoCli    repopb.RepoClient
	scriptCli  scriptpb.ScriptRunnerClient
	scripts    map[ScriptInfo]int

	// These are processed atomically.
	errorCount uint32
}

// ContainerWorkerCache defines a map - ScriptInfo->set of *Worker.
type ContainerWorkerCache map[ScriptInfo]map[string]*WorkerContainer

const (
	chunkSize = 2 * 1024 * 1024
)

// NewWorker initializes new worker info along with worker connection.
func NewWorker(id string, addr net.TCPAddr, mCPU uint32, defaultMCPU uint32, memory uint64) *Worker {
	conn, err := grpc.Dial(addr.String(), sys.DefaultGRPCDialOptions...)
	util.Must(err)

	w := Worker{
		ID:   id,
		Addr: addr,

		alive:       true,
		scripts:     make(map[ScriptInfo]int),
		conn:        conn,
		mCPU:        mCPU,
		defaultMCPU: defaultMCPU,
		freeMemory:  memory,
		freeCPU:     int32(mCPU),

		repoCli:   repopb.NewRepoClient(conn),
		scriptCli: scriptpb.NewScriptRunnerClient(conn),
	}

	freeCPUCounter.Add(int64(mCPU))

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
func (w *Worker) Reserve(mCPU uint32, require bool) bool {
	w.mu.Lock()
	if !w.alive || (require && w.freeCPU < int32(mCPU)) {
		w.mu.Unlock()
		return false
	}

	w.freeCPU -= int32(mCPU)
	w.waitGroup.Add(1)
	w.mu.Unlock()

	freeCPUCounter.Add(-int64(mCPU))

	return true
}

// Release resources reserved.
func (w *Worker) Release(mCPU uint32) {
	w.mu.Lock()
	w.freeCPU += int32(mCPU)
	w.mu.Unlock()

	freeCPUCounter.Add(int64(mCPU))
}

// IncreaseErrorCount increases error count of worker.
func (w *Worker) IncreaseErrorCount() uint32 {
	return atomic.AddUint32(&w.errorCount, 1)
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

func uploadDir(stream repopb.Repo_UploadClient, fs afero.Fs, key, sourcePath string) error {
	var err error

	if err = stream.Send(&repopb.UploadRequest{
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
		return nil
	}

	if err = afero.Walk(fs, sourcePath, func(path string, info os.FileInfo, err error) error {
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

				if err = stream.Send(&repopb.UploadRequest{
					Value: &repopb.UploadRequest_Chunk{
						Chunk: &repopb.UploadRequest_ChunkMessage{Name: name, Data: buf[:n]},
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
	if err = stream.Send(&repopb.UploadRequest{Value: &repopb.UploadRequest_Done{Done: true}}); err != nil {
		return err
	}
	// Wait for response as a confirmation of finished upload.
	if _, err = stream.Recv(); err != nil {
		return err
	}

	return nil
}

// AddCache increases ref count of Container in Worker and adds it to ContainerWorkerCache.
func (w *Worker) AddCache(cache ContainerWorkerCache, ci ScriptInfo, contID string, container *WorkerContainer) {
	w.mu.Lock()

	container.containerID = contID
	w.scripts[ci]++

	if _, ok := cache[ci]; !ok {
		cache[ci] = map[string]*WorkerContainer{contID: container}
	} else {
		cache[ci][contID] = container
	}

	w.mu.Unlock()
}

// RemoveCache decreases ref count of Container in Worker and if needed removes it from ContainerWorkerCache.
func (w *Worker) RemoveCache(cache ContainerWorkerCache, ci ScriptInfo, contID string) {
	w.mu.Lock()

	ref := w.scripts[ci]
	if ref > 1 {
		w.scripts[ci]--
	} else {
		delete(w.scripts, ci)
		if m, ok := cache[ci]; ok {
			if len(m) == 1 {
				delete(cache, ci)
			} else {
				delete(m, contID)
			}
		}
	}

	w.mu.Unlock()
}

// Shutdown removes all Containers in Worker from ContainerWorkerCache and stops connection when drained.
func (w *Worker) Shutdown(cache ContainerWorkerCache) {
	w.mu.Lock()
	w.alive = false

	for ci := range w.scripts {
		if m, ok := cache[ci]; ok {
			for contID, cont := range m {
				if cont.Worker == w {
					delete(m, contID)
				}
			}

			if len(m) == 0 {
				delete(cache, ci)
			}
		}
	}
	w.mu.Unlock()

	// Wait for all calls to finish and close connection in goroutine.
	go func() {
		w.waitGroup.Wait()
		w.conn.Close() // nolint

		freeCPUCounter.Add(-int64(w.FreeCPU()))
	}()
}

// WorkerContainer defines worker container info.
type WorkerContainer struct {
	*Worker

	containerID string
	conns       uint64
}

// Conns returns number of current connections.
func (w *WorkerContainer) Conns() uint64 {
	return atomic.LoadUint64(&w.conns)
}

// Upload calls worker RPC and uploads file(s) to it.
func (w *WorkerContainer) Upload(ctx context.Context, fs afero.Fs, sourcePath string, key string) error {
	stream, err := w.repoCli.Upload(ctx)
	if err != nil {
		return err
	}

	// Iterate through download results and upload them.
	if err = uploadDir(stream, fs, key, sourcePath); err != nil {
		if exists, e := w.repoCli.Exists(ctx, &repopb.ExistsRequest{Key: key}); e == nil && exists.Ok {
			return nil
		}
	}

	return err
}

// Run calls worker RPC and runs script there.
func (w *WorkerContainer) Run(ctx context.Context, meta *scriptpb.RunRequest_MetaMessage, chunk []*scriptpb.RunRequest_ChunkMessage) (<-chan interface{}, error) {
	stream, err := w.scriptCli.Run(ctx)
	if err != nil {
		return nil, err
	}

	// Send meta header.
	if err = stream.Send(&scriptpb.RunRequest{
		Value: &scriptpb.RunRequest_Meta{
			Meta: meta,
		},
	}); err != nil {
		return nil, err
	}

	// Send chunks.
	for _, m := range chunk {
		if err = stream.Send(&scriptpb.RunRequest{
			Value: &scriptpb.RunRequest_Chunk{
				Chunk: m,
			},
		}); err != nil {
			return nil, err
		}
	}

	if err = stream.CloseSend(); err != nil {
		return nil, err
	}

	ch := make(chan interface{}, 1)

	go func() {
		for {
			runRes, err := stream.Recv()
			if err != nil {
				if err != io.EOF {
					ch <- err
				}
				close(ch)

				atomic.AddUint64(&w.conns, ^uint64(0))
				w.waitGroup.Done()
				return
			}
			ch <- runRes
		}
	}()

	return ch, nil
}

// Reserve calls reserve on worker and increases connection count if successful.
func (w *WorkerContainer) Reserve(mCPU uint32) bool {
	if mCPU == 0 {
		mCPU = w.defaultMCPU
	}

	// If CPU requirements are met, require reservation to be successful.
	require := w.FreeCPU() > int32(mCPU)

	if w.Worker.Reserve(mCPU, require) {
		atomic.AddUint64(&w.conns, 1)
		return true
	}

	return false
}

// Release resources reserved.
func (w *WorkerContainer) Release(mCPU uint32) {
	w.Worker.Release(mCPU)
	w.waitGroup.Done()
	atomic.AddUint64(&w.conns, ^uint64(0))
}

// ScriptInfo defines unique container information.
type ScriptInfo struct {
	SourceHash  string
	Environment string
	UserID      string
	MCPU        uint32
	Async       bool
}

func (ci *ScriptInfo) String() string {
	return fmt.Sprintf("{SourceHash:%s, Environment:%s, UserID:%s}", ci.SourceHash, ci.Environment, ci.UserID)
}
