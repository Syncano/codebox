package script

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"

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
	containerPool  map[string]chan ContainerInfo
	taskPool       chan bool
	taskWaitGroup  sync.WaitGroup

	muHandler          sync.RWMutex
	onContainerRemoved ContainerRemovedHandler
	onSlotReady        SlotReadyHandler

	dockerMgr docker.Manager
	fileRepo  filerepo.Repo
	sys       sys.SystemChecker
	options   Options
}

// Options holds settable options for script runner.
type Options struct {
	// Constraints
	Concurrency     uint
	NodeIOPS        uint64
	MemoryLimit     uint64
	MemoryMargin    uint64
	StreamMaxLength int

	// Docker
	CreateTimeout     time.Duration
	CreateRetryCount  int
	CreateRetrySleep  time.Duration
	PruneImages       bool
	UseExistingImages bool

	// Cache
	ContainerTTL       time.Duration
	ContainersCapacity int

	// File repo
	HostStoragePath string
}

// DefaultOptions holds default options values for script runner.
var DefaultOptions = Options{
	Concurrency:     2,
	NodeIOPS:        150,
	MemoryLimit:     256 * 1024 * 1024,
	MemoryMargin:    100 * 1024 * 1024,
	StreamMaxLength: 512 * 1024,

	CreateTimeout:    10 * time.Second,
	CreateRetryCount: 3,
	CreateRetrySleep: 500 * time.Millisecond,

	ContainerTTL:       45 * time.Minute,
	ContainersCapacity: 250,

	HostStoragePath: "/home/codebox/storage",
}

// SlotReadyHandler is a callback function called whenever slot becomes ready.
type SlotReadyHandler func()

// ContainerRemovedHandler is a callback function called whenever container is removed.
type ContainerRemovedHandler func(ci ContainerInfo)

// RunOptions holds settable options for run command.
type RunOptions struct {
	EntryPoint  string
	OutputLimit uint32
	Timeout     time.Duration

	Args   []byte
	Meta   []byte
	Config []byte
	Files  map[string]FileData
}

// FileData holds info about a file.
type FileData struct {
	Filename    string
	ContentType string
	Data        []byte
}

func (ro RunOptions) String() string {
	return fmt.Sprintf("{EntryPoint:%.25s, OutputLimit:%d, Timeout:%v}", ro.EntryPoint, ro.OutputLimit, ro.Timeout)
}

// ContainerInfo defines unique container information.
type ContainerInfo struct {
	ID          string
	resp        types.HijackedResponse
	volumeKey   string
	SourceHash  string
	Environment string
	UserID      string
}

func (ci ContainerInfo) String() string {
	return fmt.Sprintf("{ID:%s, VolumeKey:%s}", ci.ID, ci.volumeKey)
}

var (
	freeSlotsCounter *expvar.Int

	// ErrUnsupportedRuntime signals prohibited usage of unknown runtime.
	ErrUnsupportedRuntime = errors.New("unsupported runtime")
	// ErrPoolNotRunning signals pool is not yet running.
	ErrPoolNotRunning = errors.New("pool not running")
	// ErrPoolAlreadyCreated signals pool being already created.
	ErrPoolAlreadyCreated = errors.New("pool already created")
	// ErrCriticalContainerError signals that passed response was malformed
	ErrCriticalContainerError = errors.New("critical container error")
)

const (
	containerLabel        = "workerId"
	wrapperMount          = "wrapper"
	userMount             = "code"
	environmentMount      = "img"
	defaultTimeout        = 30 * time.Second
	graceTimeout          = 3 * time.Second
	dockerTimeout         = 8 * time.Second
	dockerDownloadTimeout = 5 * time.Minute
)

// NewRunner initializes a new script runner.
func NewRunner(options Options, dockerMgr docker.Manager, checker sys.SystemChecker, repo filerepo.Repo) (*DockerRunner, error) {
	if freeSlotsCounter == nil {
		freeSlotsCounter = expvar.NewInt("slots")
	}
	mergo.Merge(&options, DefaultOptions) // nolint - error not possible

	// Set concurrency limits on docker.
	dockerMgr.SetLimits(options.Concurrency, options.NodeIOPS)

	// Check memory requirements.
	if err := checker.CheckFreeMemory(options.MemoryLimit*uint64(options.Concurrency) + options.MemoryMargin); err != nil {
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
				if e := r.dockerMgr.StopContainer(ctx, cID); e != nil {
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

func (r *DockerRunner) processRun(logger logrus.FieldLogger, runtime, sourceHash, environment, containerHash string, options RunOptions) (*Result, ContainerInfo, bool, error) {
	start := time.Now()
	logger = logger.WithFields(logrus.Fields{"runtime": runtime, "options": options, "containerHash": containerHash})
	cInfo, fromCache, err := r.getContainer(runtime, sourceHash, environment, containerHash)
	if err != nil {
		return nil, cInfo, fromCache, err
	}

	logger = logger.WithFields(logrus.Fields{"cache": fromCache, "container": cInfo})
	logger.Info("Running in container")

	// Communicate with stream we got.
	resp := cInfo.resp

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
	magicString := util.GenerateKey()
	scriptContext := wrapperContext{
		EntryPoint:      options.EntryPoint,
		OutputSeparator: util.GenerateKey(),
		MagicString:     magicString,
		Timeout:         options.Timeout,
		Args:            (*json.RawMessage)(&options.Args),
		Meta:            (*json.RawMessage)(&options.Meta),
		Config:          (*json.RawMessage)(&options.Config),
		Files:           files,
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
		if _, err = resp.Conn.Write(data); err != nil {
			return nil, cInfo, fromCache, err
		}
	}

	// Now send files for context.
	for _, f := range files {
		if _, err = resp.Conn.Write(options.Files[f.Name].Data); err != nil {
			return nil, cInfo, fromCache, err
		}
	}

	// Return processed response.
	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout+graceTimeout)
	defer cancel()
	stdout, stderr, err := r.dockerMgr.ProcessResponse(ctx, resp, magicString, options.OutputLimit)

	// Process critical container error.
	sep := []byte(scriptContext.OutputSeparator)
	if bytes.HasPrefix(stderr, sep) {
		logger.WithField("stderr", string(stderr[len(sep):])).Error("Critical container error")
		err = ErrCriticalContainerError
		return nil, cInfo, fromCache, err
	}

	// Parse result.
	var parseErr error
	ret := &Result{Stdout: stdout, Stderr: stderr, Took: time.Since(start)}
	parseErr = ret.Parse(sep, r.options.StreamMaxLength, err)
	if err == nil {
		err = parseErr
	}
	return ret, cInfo, fromCache, err
}

// Run executes given script in a docker container.
func (r *DockerRunner) Run(logger logrus.FieldLogger, runtime, sourceHash, environment, userID string, options RunOptions) (*Result, error) {
	if _, ok := SupportedRuntimes[runtime]; !ok {
		return nil, ErrUnsupportedRuntime
	}
	if options.EntryPoint == "" {
		options.EntryPoint = SupportedRuntimes[runtime].DefaultEntryPoint
	}
	if options.Timeout == 0 {
		options.Timeout = defaultTimeout
	}
	logger = logger.WithFields(logrus.Fields{
		"runtime":     runtime,
		"options":     options,
		"sourceHash":  sourceHash,
		"userID":      userID,
		"environment": environment,
	})

	if !r.IsRunning() {
		logger.WithError(ErrPoolNotRunning).Error("Run failed - pool is not running")
		return nil, ErrPoolNotRunning
	}
	start := time.Now()

	// Acquire lock from pool.
	<-r.taskPool
	freeSlotsCounter.Add(-1) // Decrease free slots counter.
	r.taskWaitGroup.Add(1)

	containerHash := fmt.Sprintf("%s/%s/%s/%x", sourceHash, userID, environment, util.Hash(options.EntryPoint))
	ret, cInfo, fromCache, err := r.processRun(logger, runtime, sourceHash, environment, containerHash, options)
	took := time.Since(start)

	if err != nil {
		logger = logger.WithError(err)
	} else {
		// Save container info and store it in cache.
		ret.Cached = true
		cInfo.SourceHash = sourceHash
		cInfo.Environment = environment
		cInfo.UserID = userID
		r.containerCache.Push(containerHash, cInfo)
	}
	if ret != nil && ret.Took > 0 {
		ret.Overhead = took - ret.Took
	}

	logger.WithFields(logrus.Fields{
		"took":        took,
		"ret":         ret,
		"containerID": cInfo.ID,
	}).Info("Run finished")

	go r.afterRun(runtime, cInfo, fromCache, err)
	return ret, err
}

func (r *DockerRunner) afterRun(runtime string, cInfo ContainerInfo, fromCache bool, err error) {
	logger := logrus.WithError(err).WithField("container", cInfo)
	// Add new container if we took it from the pool.
	recreate := !fromCache

	// If we encountered missing resource - reuse container.
	if err == filerepo.ErrResourceNotFound {
		// If cleanup volume fails - cleanup whole container and recreate in next block. Otherwise - return it to pool.
		if err = r.fileRepo.CleanupVolume(cInfo.volumeKey); err == nil {
			r.containerPool[runtime] <- cInfo
			recreate = false
		}
	}

	if err != nil {
		// Check for non critical errors.
		if err == docker.ErrLimitReached || err == ErrCriticalContainerError || err == io.EOF || err == context.DeadlineExceeded {
			logger.Warn("Recovering from container error")
		} else {
			logger.Error("Recovering from container error")
		}
		r.cleanupContainer(cInfo)
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

	r.taskWaitGroup.Done()
	r.taskPool <- true
	freeSlotsCounter.Add(1) // Increase free slots counter.

	r.muHandler.RLock()
	if r.onSlotReady != nil {
		go r.onSlotReady()
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

	// Create and fill task pool and reserve memory for each concurrency.
	r.taskPool = make(chan bool, r.options.Concurrency)
	for i := uint(0); i < r.options.Concurrency; i++ {
		r.taskPool <- true

		if err := r.sys.ReserveMemory(r.options.MemoryLimit); err != nil {
			return "", err
		}
	}

	// Set free slots counter.
	freeSlotsCounter.Set(int64(r.options.Concurrency))

	// Create and fill container pool.
	r.containerPool = make(map[string]chan ContainerInfo)
	done := make(chan error, r.options.Concurrency)

	// Process concurrently each runtime.
	for runtime, rInfo := range SupportedRuntimes {
		// Store wrappers.
		if _, err := r.fileRepo.PermStore(runtime, rInfo.Wrapper(), rInfo.FileName); err != nil {
			return "", err
		}

		// Create containers.
		r.containerPool[runtime] = make(chan ContainerInfo, r.options.Concurrency)

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
	r.OnSlotReady(nil)

	r.setRunning(false)
	logrus.Info("Stopping pool")
	freeSlotsCounter.Set(0) // Reset free slots counter.

	// Wait for all tasks to be done and cleanup all containers.
	r.taskWaitGroup.Wait()
	logrus.Info("All tasks are done")

	if r.containerPool != nil {
		var wg sync.WaitGroup
		for _, ch := range r.containerPool {
		Loop:
			for {
				select {
				case cInfo := <-ch:
					wg.Add(1)
					go func() {
						r.cleanupContainer(cInfo)
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

func (r *DockerRunner) getContainer(runtime, sourceHash, environment, containerHash string) (cInfo ContainerInfo, fromCache bool, err error) {
	// Try to get a container from cache.
	cacheVal := r.containerCache.Pop(containerHash)
	if cacheVal != nil {
		cInfo = cacheVal.(ContainerInfo)
		fromCache = true
		return
	}

	// Fallback to pool.
	cInfo = <-r.containerPool[runtime]

	// Linking sources.
	logger := logrus.WithFields(logrus.Fields{"container": cInfo, "runtime": runtime})
	if err = r.fileRepo.Link(cInfo.volumeKey, sourceHash, userMount); err != nil {
		logger.WithError(err).WithField("sourceHash", sourceHash).Error("Linking error")
		return
	}

	// Linking environment.
	if environment != "" {
		if err = r.fileRepo.Link(cInfo.volumeKey, environment, environmentMount); err != nil {
			logger.WithError(err).WithField("environment", environment).Error("Linking error")
		}
	}
	return
}

func (r *DockerRunner) createFreshContainer(ctx context.Context, runtime string) (ContainerInfo, error) {
	var (
		cInfo ContainerInfo
		err   error
	)
	logger := logrus.WithField("runtime", runtime)
	start := time.Now()
	rInfo := SupportedRuntimes[runtime]

	// Create new volume and link wrapper.
	var volHostPath string
	cInfo.volumeKey, volHostPath, err = r.createWrapperVolume(runtime)
	if err != nil {
		return cInfo, err
	}

	// Create container for given runtime with default constraints.
	cInfo.ID, err = r.dockerMgr.CreateContainer(
		ctx, rInfo.Image, rInfo.User, rInfo.Command,
		rInfo.Environment,
		map[string]string{containerLabel: r.poolID},
		rInfo.Constraints,
		[]string{volHostPath + ":/app:ro"},
	)
	if err != nil {
		return cInfo, err
	}
	logger = logger.WithField("container", cInfo)

	// Start and attach to container (wrapper mode).
	err = r.dockerMgr.StartContainer(ctx, cInfo.ID)
	if err != nil {
		return cInfo, err
	}

	cInfo.resp, err = r.dockerMgr.AttachContainer(ctx, cInfo.ID)
	if err != nil {
		return cInfo, err
	}

	_, _, err = r.dockerMgr.ProcessResponse(ctx, cInfo.resp, "ready", 0)
	if err == nil {
		logger.WithField("took", time.Since(start)).Info("Container created and reported as ready")
	}
	return cInfo, err
}

func (r *DockerRunner) cleanupContainer(cInfo ContainerInfo) {
	logger := logrus.WithFields(logrus.Fields{"container": cInfo})
	logger.Info("Stopping and cleaning up container")

	// Stop the container.
	if cInfo.ID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), dockerTimeout)
		if err := r.dockerMgr.StopContainer(ctx, cInfo.ID); err != nil {
			logger.WithError(err).Warn("Stopping container failed")
		}
		cancel()
	}

	if cInfo.volumeKey != "" {
		if err := r.fileRepo.DeleteVolume(cInfo.volumeKey); err != nil {
			logger.WithError(err).Warn("Removing volume failed")
		}
	}
}

func (r *DockerRunner) onEvictedContainerHandler(key string, val interface{}) {
	cInfo := val.(ContainerInfo)

	r.muHandler.RLock()
	if r.onContainerRemoved != nil {
		go r.onContainerRemoved(cInfo)
	}
	r.muHandler.RUnlock()
	r.cleanupContainer(cInfo)
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

// OnSlotReady sets an (optional) function that when slot becomes ready.
// Set to nil to disable.
func (r *DockerRunner) OnSlotReady(f SlotReadyHandler) {
	r.muHandler.Lock()
	r.onSlotReady = f
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
