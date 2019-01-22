package script

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"io"
	"math"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"

	"github.com/Syncano/codebox/pkg/cache"
	"github.com/Syncano/codebox/pkg/docker"
	"github.com/Syncano/codebox/pkg/filerepo"
	"github.com/Syncano/codebox/pkg/sys"
	"github.com/Syncano/codebox/pkg/util"
)

// DockerRunner provides methods to use to run user scripts securely.
type DockerRunner struct {
	poolID         string
	running        uint32
	createMu       sync.Mutex
	containerCache *cache.StackCache
	containerPool  map[string]chan *Container
	poolSemaphore  *semaphore.Weighted
	taskWaitGroup  sync.WaitGroup

	muHandler          sync.RWMutex
	onContainerRemoved ContainerRemovedHandler
	onRunDone          RunDoneHandler

	dockerMgr docker.Manager
	fileRepo  filerepo.Repo
	sys       sys.SystemChecker
	options   Options
}

// Options holds settable options for script runner.
type Options struct {
	// Constraints
	Concurrency     uint
	MCPU            uint
	NodeIOPS        uint64
	MemoryMargin    uint64
	StreamMaxLength int

	// Docker
	CreateTimeout     time.Duration
	CreateRetryCount  int
	CreateRetrySleep  time.Duration
	PruneImages       bool
	UseExistingImages bool
	Constraints       docker.Constraints

	// Cache
	ContainerTTL       time.Duration
	ContainersCapacity int

	// File repo
	HostStoragePath string
}

// DefaultOptions holds default options values for script runner.
var DefaultOptions = Options{
	Concurrency:     2,
	MCPU:            2000,
	NodeIOPS:        150,
	MemoryMargin:    100 * 1024 * 1024,
	StreamMaxLength: 512 * 1024,

	CreateTimeout:    10 * time.Second,
	CreateRetryCount: 3,
	CreateRetrySleep: 500 * time.Millisecond,
	Constraints: docker.Constraints{
		// CPU and IOPS limit is calculated based on concurrency.
		MemoryLimit:     200 * 1024 * 1024,
		MemorySwapLimit: 0,
		PidLimit:        32,
		NofileUlimit:    1024,
		StorageLimit:    "500M",
	},

	ContainerTTL:       45 * time.Minute,
	ContainersCapacity: 250,

	HostStoragePath: "/home/codebox/storage",
}

// RunDoneHandler is a callback function called whenever run has finished.
type RunDoneHandler func(cont *Container, options *RunOptions)

// ContainerRemovedHandler is a callback function called whenever container is removed.
type ContainerRemovedHandler func(cont *Container)

// RunOptions holds settable options for run command.
type RunOptions struct {
	EntryPoint  string
	OutputLimit uint32
	Timeout     time.Duration
	MCPU        uint32
	Memory      uint64
	Weight      uint

	Args   []byte
	Meta   []byte
	Config []byte
	Files  map[string]File
}

// File holds info about a file.
type File struct {
	Filename    string
	ContentType string
	Data        []byte
}

func (ro RunOptions) String() string {
	return fmt.Sprintf("{EntryPoint:%.25s, OutputLimit:%d, Timeout:%v}", ro.EntryPoint, ro.OutputLimit, ro.Timeout)
}

// Container defines unique container information.
type Container struct {
	ID          string
	SourceHash  string
	Environment string
	UserID      string

	mu        sync.Mutex
	resp      types.HijackedResponse
	conn      io.ReadWriteCloser
	addr      string
	volumeKey string
}

// Conn returns container conn.
func (c *Container) Conn() (io.ReadWriteCloser, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		conn, err := net.DialTimeout("tcp", c.addr, dockerTimeout)
		if err != nil {
			return nil, err
		}

		c.conn = conn
	}

	return c.conn, nil
}

func (c *Container) String() string {
	return fmt.Sprintf("{ID:%s, VolumeKey:%s}", c.ID, c.volumeKey)
}

var (
	initOnceRunner sync.Once
	freeCPUCounter *expvar.Int

	// ErrUnsupportedRuntime signals prohibited usage of unknown runtime.
	ErrUnsupportedRuntime = errors.New("unsupported runtime")
	// ErrPoolNotRunning signals pool is not yet running.
	ErrPoolNotRunning = errors.New("pool not running")
	// ErrPoolAlreadyCreated signals pool being already created.
	ErrPoolAlreadyCreated = errors.New("pool already created")
)

const (
	containerLabel        = "workerId"
	wrapperMount          = "wrapper"
	userMount             = "code"
	environmentMount      = "env"
	environmentFileName   = "squashfs.img"
	defaultTimeout        = 30 * time.Second
	graceTimeout          = 3 * time.Second
	dockerTimeout         = 8 * time.Second
	dockerDownloadTimeout = 5 * time.Minute
	stdWriterPrefixLen    = 8
)

// NewRunner initializes a new script runner.
func NewRunner(options Options, dockerMgr docker.Manager, checker sys.SystemChecker, repo filerepo.Repo) (*DockerRunner, error) {
	initOnceRunner.Do(func() {
		freeCPUCounter = expvar.NewInt("cpu")
	})
	mergo.Merge(&options, DefaultOptions) // nolint - error not possible

	// Set concurrency limits on docker.
	dockerMCPU := uint(dockerMgr.Info().NCPU * 1000)
	if options.MCPU > dockerMCPU {
		logrus.WithFields(
			logrus.Fields{
				"requested": options.MCPU,
				"available": dockerMCPU,
			}).Warn("CPU defined higher than available CPU resources, using what we got instead")
		options.MCPU = dockerMCPU
	}

	// Calculate CPU and IOPS limit.
	options.Constraints.CPULimit = int64((options.MCPU-uint(dockerMgr.Options().ReservedCPU*1e3))*1e6) / int64(options.Concurrency)
	options.Constraints.IOPSLimit = options.NodeIOPS / uint64(options.Concurrency)

	// Check memory requirements.
	if err := checker.CheckFreeMemory(uint64(options.Constraints.MemoryLimit)*uint64(options.Concurrency) + options.MemoryMargin); err != nil {
		return nil, err
	}

	containerCache := cache.NewStackCache(cache.Options{
		TTL:      options.ContainerTTL,
		Capacity: options.ContainersCapacity,
	})
	r := &DockerRunner{
		dockerMgr:      dockerMgr,
		fileRepo:       repo,
		sys:            checker,
		options:        options,
		containerCache: containerCache,
	}
	r.containerCache.OnValueEvicted(r.onEvictedContainerHandler)
	return r, nil
}

// Options returns a copy of runner options struct.
func (r *DockerRunner) Options() Options {
	return r.options
}

// CleanupUnused removes unused docker containers.
func (r *DockerRunner) CleanupUnused() {
	ctx, cancel := context.WithTimeout(context.Background(), dockerTimeout)
	defer cancel()
	logrus.Info("Cleaning up unused containers")
	cl, err := r.dockerMgr.ListContainersByLabel(ctx, containerLabel)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	for _, c := range cl {
		if c.Labels[containerLabel] != r.poolID {
			wg.Add(1)
			go func(cID string) {
				ctx, cancel := context.WithTimeout(context.Background(), dockerTimeout)
				if e := r.dockerMgr.ContainerStop(ctx, cID); e != nil {
					logrus.WithField("containerID", cID).Warn("Stopping container failed")
				}
				cancel()
				wg.Done()
			}(c.ID)
		}
	}
	wg.Wait()

	logrus.Info("Cleaning up unused files")
	r.fileRepo.CleanupUnused()

	if r.options.PruneImages {
		ctx, cancel := context.WithTimeout(context.Background(), dockerTimeout)
		defer cancel()
		logrus.Info("Cleaning up unused images")
		_, err = r.dockerMgr.PruneImages(ctx)
		if err != nil {
			panic(err)
		}
	}
}

// DownloadAllImages downloads all docker images for supported runtimes.
func (r *DockerRunner) DownloadAllImages() error {
	logrus.Info("Downloading all docker images")
	set := make(map[string]struct{})
	for _, ri := range SupportedRuntimes {
		set[ri.Image] = struct{}{}
	}
	for image := range set {
		ctx, cancel := context.WithTimeout(context.Background(), dockerDownloadTimeout)
		defer cancel()

		err := r.dockerMgr.DownloadImage(ctx, image, r.options.UseExistingImages)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *DockerRunner) processRun(ctx context.Context, logger logrus.FieldLogger, runtime, sourceHash, environment, containerHash string, options *RunOptions) (*Result, *Container, bool, error) {
	start := time.Now()
	logger = logger.WithFields(logrus.Fields{"runtime": runtime, "options": options, "containerHash": containerHash})
	cont, fromCache, err := r.getContainer(runtime, sourceHash, environment, containerHash, options.Weight)
	if err != nil {
		return nil, cont, fromCache, err
	}

	logger = logger.WithFields(logrus.Fields{"cache": fromCache, "container": cont})
	logger.Info("Running in container")

	// Communicate with stream we got.
	conn, err := cont.Conn()
	if err != nil {
		return nil, cont, fromCache, err
	}

	// Prepare files for context.
	var filesSize int
	var files []contextFile
	for f, data := range options.Files {
		flen := len(data.Data)
		files = append(files, contextFile{
			Name:        f,
			Filename:    data.Filename,
			ContentType: data.ContentType,
			Length:      flen,
		})
		filesSize += flen
	}

	// Prepare context for container.
	scriptContext := wrapperContext{
		Async:       false,
		MuxResponse: MuxResponse,
		EntryPoint:  options.EntryPoint,
		Timeout:     options.Timeout,
		Args:        (*json.RawMessage)(&options.Args),
		Meta:        (*json.RawMessage)(&options.Meta),
		Config:      (*json.RawMessage)(&options.Config),
		Files:       files,
	}
	scriptContextBytes, err := json.Marshal(scriptContext)
	util.Must(err)
	contextSize := len(scriptContextBytes)

	// Send context.
	totalLen := make([]byte, 4)
	contextLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(totalLen, uint32(contextSize+filesSize+8))
	binary.LittleEndian.PutUint32(contextLen, uint32(contextSize))
	for _, data := range [][]byte{totalLen, contextLen, scriptContextBytes} {
		if _, err = conn.Write(data); err != nil {
			return nil, cont, fromCache, err
		}
	}

	// Now send files for context.
	for _, f := range files {
		if _, err = conn.Write(options.Files[f.Name].Data); err != nil {
			return nil, cont, fromCache, err
		}
	}

	// Return processed response.
	ctx, cancel := context.WithTimeout(ctx, options.Timeout+graceTimeout)
	defer cancel()
	mux, err := readMux(ctx, conn, options.OutputLimit)

	// On container crash, get stderr from it.
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		if stream, e := r.dockerMgr.ContainerErrorLog(ctx, cont.ID); e == nil {
			stdcopy.StdCopy(mux[MuxStdout], mux[MuxStderr], stream) // nolint: errcheck
		}
	}

	// Parse result.
	var parseErr error
	ret := &Result{Stdout: mux[MuxStdout].Bytes(), Stderr: mux[MuxStderr].Bytes(), Took: time.Since(start), Weight: options.Weight}
	parseErr = ret.Parse(mux[MuxResponse].Bytes(), r.options.StreamMaxLength, err)
	if err == nil {
		err = parseErr
	}
	return ret, cont, fromCache, err
}

// Run executes given script in a docker container.
func (r *DockerRunner) Run(ctx context.Context, logger logrus.FieldLogger, runtime, sourceHash, environment, userID string, options *RunOptions) (*Result, error) {
	if _, ok := SupportedRuntimes[runtime]; !ok {
		return nil, ErrUnsupportedRuntime
	}
	if options.EntryPoint == "" {
		options.EntryPoint = SupportedRuntimes[runtime].DefaultEntryPoint
	}
	if options.Timeout <= 0 {
		options.Timeout = defaultTimeout
	}
	if options.MCPU == 0 {
		options.MCPU = uint32(r.options.Constraints.CPULimit / 1e6)
	}
	if options.Weight == 0 {
		options.Weight = uint(math.Ceil(float64(options.MCPU) * 1e6 / float64(r.options.Constraints.CPULimit)))
	}

	logger = logger.WithFields(logrus.Fields{
		"runtime":     runtime,
		"options":     options,
		"sourceHash":  sourceHash,
		"userID":      userID,
		"environment": environment,
		"weight":      options.Weight,
	})

	if !r.IsRunning() {
		logger.WithError(ErrPoolNotRunning).Error("Run failed - pool is not running")
		return nil, ErrPoolNotRunning
	}
	start := time.Now()

	// Acquire from semaphore.
	if err := r.poolSemaphore.Acquire(ctx, int64(options.Weight)); err != nil {
		return nil, err
	}

	// Decrease free resource counter.
	freeCPUCounter.Add(-int64(options.MCPU))
	r.taskWaitGroup.Add(1)

	containerHash := fmt.Sprintf("%s/%s/%s/%x", sourceHash, userID, environment, util.Hash(options.EntryPoint))
	ret, cont, fromCache, err := r.processRun(ctx, logger, runtime, sourceHash, environment, containerHash, options)
	took := time.Since(start)

	if err != nil {
		logger = logger.WithError(err)
	} else {
		// Save container info and store it in cache.
		ret.Cached = true
		cont.SourceHash = sourceHash
		cont.Environment = environment
		cont.UserID = userID
		r.containerCache.Push(containerHash, cont)
	}
	if ret != nil && ret.Took > 0 {
		ret.Overhead = took - ret.Took
	}

	logger.WithFields(logrus.Fields{
		"took":        took,
		"ret":         ret,
		"containerID": cont.ID,
	}).Info("Run finished")

	freeCPUCounter.Add(int64(options.MCPU))
	go r.afterRun(runtime, cont, options, fromCache, err)
	return ret, err
}

func (r *DockerRunner) afterRun(runtime string, cont *Container, options *RunOptions, fromCache bool, err error) {
	logger := logrus.WithError(err).WithField("container", cont)
	// Add new container if we took it from the pool.
	recreate := !fromCache

	// If we encountered missing resource - reuse container.
	if err == filerepo.ErrResourceNotFound {
		// If cleanup volume fails - cleanup whole container and recreate in next block. Otherwise - return it to pool.
		if err = r.fileRepo.CleanupVolume(cont.volumeKey); err == nil {
			r.containerPool[runtime] <- cont
			recreate = false
		}
	}

	if err != nil {
		// Check for non critical errors.
		logFunc := logger.Warn
		if err != ErrLimitReached && err != io.EOF && err != context.DeadlineExceeded {
			logFunc = logger.Error
		}
		logFunc("Recovering from container error")
		r.cleanupContainer(cont)
	}
	// Add new container if needed.
	if recreate {
		// Try to create a container, on fail - clean it up and retry.
		if err = util.Retry(r.options.CreateRetryCount, r.options.CreateRetrySleep, func() error {
			ctx, cancel := context.WithTimeout(context.Background(), r.options.CreateTimeout)
			defer cancel()

			// Allow only 1 concurrent container creation as it is relatively heavy operation.
			r.createMu.Lock()
			newContainer, e := r.createFreshContainer(ctx, runtime)
			r.createMu.Unlock()

			if e != nil {
				r.cleanupContainer(newContainer)
				return err
			}

			r.containerPool[runtime] <- newContainer
			return nil
		}); err != nil {
			// If we did fail to create a container, panic!
			panic(err)
		}
	}

	// Try to reserve needed memory. If not enough memory available - remove LRU containers until satisfied.
	for {
		if err = r.sys.CheckFreeMemory(0); err != nil {
			logger.WithError(err).Warn("Not enough memory, removing LRU container")
			// Try to delete LRU container.
			if !r.containerCache.DeleteLRU() {
				panic(err)
			}
		} else {
			break
		}
	}

	// Release semaphore.
	r.poolSemaphore.Release(int64(options.Weight))
	r.taskWaitGroup.Done()

	r.muHandler.RLock()
	if r.onRunDone != nil {
		go r.onRunDone(cont, options)
	}
	r.muHandler.RUnlock()
}

// CreatePool creates the container pool. Meant to run only once.
func (r *DockerRunner) CreatePool() (string, error) {
	if r.IsRunning() {
		return "", ErrPoolAlreadyCreated
	}
	r.poolID = util.GenerateKey()

	logrus.WithField("concurrency", r.options.Concurrency).Info("Creating pool")

	// Reset reserved memory and reserve memory margin.
	r.sys.Reset()
	if err := r.sys.ReserveMemory(r.options.MemoryMargin); err != nil {
		return "", err
	}

	// Create pool semaphore and reserve memory for each concurrency.
	r.poolSemaphore = semaphore.NewWeighted(int64(r.options.Concurrency))
	for i := uint(0); i < r.options.Concurrency; i++ {
		if err := r.sys.ReserveMemory(uint64(r.options.Constraints.MemoryLimit)); err != nil {
			return "", err
		}
	}

	// Set free resources counter.
	freeCPUCounter.Set(int64(r.options.MCPU))

	// Create and fill container pool.
	r.containerPool = make(map[string]chan *Container)
	done := make(chan error, r.options.Concurrency)

	// Process concurrently each runtime.
	for runtime, rInfo := range SupportedRuntimes {
		// Store wrappers.
		if _, err := r.fileRepo.PermStore(runtime, rInfo.Wrapper(), rInfo.FileName); err != nil {
			return "", err
		}

		// Create containers.
		r.containerPool[runtime] = make(chan *Container, r.options.Concurrency)

		for i := uint(0); i < r.options.Concurrency; i++ {
			go func(runtime string) {
				ctx, cancel := context.WithTimeout(context.Background(), r.options.CreateTimeout)
				defer cancel()

				containerID, err := r.createFreshContainer(ctx, runtime)
				if err != nil {
					done <- err
					return
				}
				r.containerPool[runtime] <- containerID
				done <- nil
			}(runtime)
		}

		for i := uint(0); i < r.options.Concurrency; i++ {
			err := <-done
			if err != nil {
				return "", err
			}
		}
	}

	r.setRunning(true)
	return r.poolID, nil
}

// StopPool stops the pool and cleans it up.
func (r *DockerRunner) StopPool() {
	if !r.IsRunning() {
		return
	}

	r.OnContainerRemoved(nil)
	r.OnRunDone(nil)

	r.setRunning(false)
	logrus.Info("Stopping pool")
	// Reset free resources counter.
	freeCPUCounter.Set(0)

	// Wait for all tasks to be done and cleanup all containers.
	r.taskWaitGroup.Wait()
	logrus.Info("All tasks are done")

	if r.containerPool != nil {
		var wg sync.WaitGroup
		for _, ch := range r.containerPool {
		Loop:
			for {
				select {
				case cont := <-ch:
					wg.Add(1)
					go func() {
						r.cleanupContainer(cont)
						wg.Done()
					}()
				default:
					break Loop
				}
			}
		}
		wg.Wait()
	}

	// Clear cache.
	logrus.Info("Stopping cached containers")
	r.containerCache.Flush()
}

// Shutdown stops everything.
func (r *DockerRunner) Shutdown() {
	r.StopPool()

	// Stop cache janitor.
	r.containerCache.StopJanitor()
}

func (r *DockerRunner) getContainer(runtime, sourceHash, environment, containerHash string, weight uint) (cont *Container, fromCache bool, err error) {
	// Try to get a container from cache.
	cacheVal := r.containerCache.Pop(containerHash)
	if cacheVal != nil {
		cont = cacheVal.(*Container)
		return cont, true, nil
	}

	// Fallback to pool.
	cont = <-r.containerPool[runtime]

	// Set CPU/Memory resources if using higher weight.
	if weight > 1 {
		ctx, cancel := context.WithTimeout(context.Background(), dockerTimeout)
		defer cancel()
		if err = r.dockerMgr.ContainerUpdate(ctx, cont.ID, docker.Constraints{
			CPULimit:    int64(weight) * r.options.Constraints.CPULimit,
			MemoryLimit: int64(weight) * r.options.Constraints.MemoryLimit,
		}); err != nil {
			return cont, false, err
		}
	}

	// Linking sources.
	logger := logrus.WithFields(logrus.Fields{"container": cont, "runtime": runtime})
	if err = r.fileRepo.Link(cont.volumeKey, sourceHash, userMount); err != nil {
		logger.WithError(err).WithField("sourceHash", sourceHash).Error("Linking error")
		return cont, false, err
	}

	// Linking environment.
	if environment != "" {
		if err = r.fileRepo.Mount(cont.volumeKey, environment, environmentFileName, environmentMount); err != nil {
			logger.WithError(err).WithField("environment", environment).Error("Mounting error")
		}
	}
	return cont, false, err
}

func (r *DockerRunner) createFreshContainer(ctx context.Context, runtime string) (*Container, error) {
	var err error
	cont := new(Container)
	logger := logrus.WithField("runtime", runtime)
	start := time.Now()
	rInfo := SupportedRuntimes[runtime]

	// Create new volume and link wrapper.
	var volHostPath string
	cont.volumeKey, volHostPath, err = r.createWrapperVolume(runtime)
	if err != nil {
		return cont, err
	}

	// Create container for given runtime with default constraints.
	cont.ID, err = r.dockerMgr.ContainerCreate(
		ctx, rInfo.Image, rInfo.User, rInfo.Command,
		rInfo.Environment,
		map[string]string{containerLabel: r.poolID},
		r.options.Constraints,
		[]string{volHostPath + ":/app:ro,rslave"},
	)
	if err != nil {
		return cont, err
	}
	logger = logger.WithField("container", cont)

	// Start and attach to container (wrapper mode).
	err = r.dockerMgr.ContainerStart(ctx, cont.ID)
	if err != nil {
		return cont, err
	}

	cont.resp, err = r.dockerMgr.ContainerAttach(ctx, cont.ID)
	if err != nil {
		return cont, err
	}

	// Make sure container wrapper is ready.
	retCh := make(chan string)
	go func() {
		l, _, _ := cont.resp.Reader.ReadLine()
		if len(l) > stdWriterPrefixLen {
			l = l[stdWriterPrefixLen:]
		}
		retCh <- string(l)
		cont.resp.Close()
	}()
	select {
	case cont.addr = <-retCh:
	case <-ctx.Done():
		return cont, ctx.Err()
	}

	logger.WithField("took", time.Since(start)).Info("Container created and reported as ready")
	return cont, err
}

func (r *DockerRunner) cleanupContainer(cont *Container) {
	logger := logrus.WithFields(logrus.Fields{"container": cont})
	logger.Info("Stopping and cleaning up container")

	// Stop the container.
	if cont.ID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), dockerTimeout)
		if err := r.dockerMgr.ContainerStop(ctx, cont.ID); err != nil {
			logger.WithError(err).Warn("Stopping container failed")
		}
		cancel()
	}

	if cont.volumeKey != "" {
		if err := r.fileRepo.DeleteVolume(cont.volumeKey); err != nil {
			logger.WithError(err).Warn("Removing volume failed")
		}
	}
}

func (r *DockerRunner) onEvictedContainerHandler(key string, val interface{}) {
	cont := val.(*Container)

	r.muHandler.RLock()
	if r.onContainerRemoved != nil {
		go r.onContainerRemoved(cont)
	}
	r.muHandler.RUnlock()
	r.cleanupContainer(cont)
}

// IsRunning returns true if pool is setup and running.
func (r *DockerRunner) IsRunning() bool {
	return (atomic.LoadUint32(&r.running) == 1)
}

func (r *DockerRunner) setRunning(running bool) {
	if running {
		atomic.StoreUint32(&r.running, 1)
	} else {
		atomic.StoreUint32(&r.running, 0)
	}
}

// OnContainerRemoved sets an (optional) function that when container is removed.
// Set to nil to disable.
func (r *DockerRunner) OnContainerRemoved(f ContainerRemovedHandler) {
	r.muHandler.Lock()
	r.onContainerRemoved = f
	r.muHandler.Unlock()
}

// OnRunDone sets an (optional) function that when run has finished.
// Set to nil to disable.
func (r *DockerRunner) OnRunDone(f RunDoneHandler) {
	r.muHandler.Lock()
	r.onRunDone = f
	r.muHandler.Unlock()
}

func (r *DockerRunner) createWrapperVolume(runtime string) (string, string, error) {
	volKey, volPath, err := r.fileRepo.CreateVolume()
	if err != nil {
		return "", "", err
	}
	if err = r.fileRepo.Link(volKey, runtime, wrapperMount); err != nil {
		return "", "", err
	}
	volRelPath, err := r.fileRepo.RelativePath(volPath)
	util.Must(err)
	volHostPath := filepath.Join(r.options.HostStoragePath, volRelPath)
	return volKey, volHostPath, nil
}
