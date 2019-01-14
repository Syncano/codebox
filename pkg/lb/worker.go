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

	"github.com/Syncano/codebox/pkg/sys"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/spf13/afero"
	"google.golang.org/grpc"

	repopb "github.com/Syncano/codebox/pkg/filerepo/proto"
	scriptpb "github.com/Syncano/codebox/pkg/script/proto"
	"github.com/Syncano/codebox/pkg/util"
)

// Worker holds basic info about the worker.
type Worker struct {
	ID   string
	Addr net.TCPAddr

	conn      *grpc.ClientConn
	waitGroup sync.WaitGroup
	mCPU      uint32
	memory    uint64

	freeCPU    int32
	freeMemory int64
	mu         sync.RWMutex
	alive      bool
	repoCli    repopb.RepoClient
	scriptCli  scriptpb.ScriptRunnerClient
	scripts    map[ScriptInfo]int

	// These are processed atomically.
	errorCount uint32
}

// ScriptInfo defines unique container information.
type ScriptInfo struct {
	SourceHash  string
	Environment string
	UserID      string
	MCPU        uint32
	Memory      uint64
}

func (ci *ScriptInfo) String() string {
	return fmt.Sprintf("{SourceHash:%s, Environment:%s, UserID:%s}", ci.SourceHash, ci.Environment, ci.UserID)
}

// ContainerWorkerCache defines a map - ScriptInfo->set of *Worker.
type ContainerWorkerCache map[ScriptInfo]*hashset.Set

const (
	chunkSize = 2 * 1024 * 1024
)

// NewWorker initializes new worker info along with worker connection.
func NewWorker(id string, addr net.TCPAddr, mCPU uint32, memory uint64) *Worker {
	conn, err := grpc.Dial(addr.String(), sys.DefaultGRPCDialOptions...)
	util.Must(err)

	w := Worker{
		ID:   id,
		Addr: addr,

		alive:      true,
		scripts:    make(map[ScriptInfo]int),
		conn:       conn,
		mCPU:       mCPU,
		memory:     memory,
		freeCPU:    int32(mCPU),
		freeMemory: int64(memory),

		repoCli:   repopb.NewRepoClient(conn),
		scriptCli: scriptpb.NewScriptRunnerClient(conn),
	}

	freeCPUCounter.Add(int64(mCPU))
	freeMemoryCounter.Add(int64(memory))
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
func (w *Worker) FreeMemory() int64 {
	return atomic.LoadInt64(&w.freeMemory)
}

// Alive returns true if worker is alive.
func (w *Worker) Alive() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.alive
}

// Reserve checks if worker is alive, decreases resources and increases waitgroup.
// If require is true, returns true only if resources are available.
func (w *Worker) Reserve(mCPU uint32, memory uint64, require bool) bool {
	w.mu.Lock()

	if !w.alive || (require && w.freeCPU < int32(mCPU)) {
		w.mu.Unlock()
		return false
	}

	w.freeCPU -= int32(mCPU)
	w.freeMemory -= int64(memory)
	w.waitGroup.Add(1)
	w.mu.Unlock()

	freeCPUCounter.Add(-int64(mCPU))
	freeMemoryCounter.Add(-int64(memory))
	return true
}

// Release resources reserved and marks waitgroup as done if waitgroup param is true.
func (w *Worker) Release(mCPU uint32, memory uint64, waitgroup bool) {
	w.mu.Lock()
	w.freeCPU += int32(mCPU)
	w.freeMemory += int64(memory)
	if waitgroup {
		w.waitGroup.Done()
	}
	w.mu.Unlock()

	freeCPUCounter.Add(int64(mCPU))
	freeMemoryCounter.Add(int64(memory))
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

// Upload calls worker RPC and uploads file(s) to it.
func (w *Worker) Upload(ctx context.Context, fs afero.Fs, sourcePath string, key string) error {
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
func (w *Worker) Run(ctx context.Context, meta *scriptpb.RunRequest_MetaMessage, chunk []*scriptpb.RunRequest_ChunkMessage) (<-chan interface{}, error) {
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
				w.waitGroup.Done()
				return
			}
			ch <- runRes
		}
	}()
	return ch, nil
}

// AddCache increases ref count of Container in Worker and if needed adds it to ContainerWorkerCache.
func (w *Worker) AddCache(ci ScriptInfo, cache ContainerWorkerCache) {
	w.mu.Lock()

	w.scripts[ci]++
	if _, ok := cache[ci]; !ok {
		cache[ci] = hashset.New()
	}
	cache[ci].Add(w)

	w.mu.Unlock()
}

// RemoveCache decreases ref count of Container in Worker and if needed removes it from ContainerWorkerCache.
func (w *Worker) RemoveCache(ci ScriptInfo, cache ContainerWorkerCache) {
	w.mu.Lock()

	ref := w.scripts[ci]
	if ref > 1 {
		w.scripts[ci]--
	} else {
		delete(w.scripts, ci)
		set, ok := cache[ci]
		if ok {
			set.Remove(w)

			// If it is the last container, delete whole key from cache.
			if set.Size() == 0 {
				delete(cache, ci)
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
		set, ok := cache[ci]
		if ok {
			set.Remove(w)

			// If it is the last container, delete whole key from cache.
			if set.Size() == 0 {
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
		freeMemoryCounter.Add(-w.FreeMemory())
	}()
}
