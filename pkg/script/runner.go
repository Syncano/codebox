package script

import (
	"bufio"
	"context"
	"errors"
	"expvar"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/imdario/mergo"
	"github.com/juju/ratelimit"
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
	containerCache *cache.LRUSetCache
	containerPool  map[string]chan *Container
	poolSemaphore  *semaphore.Weighted
	taskWaitGroup  sync.WaitGroup

	muHandler           sync.RWMutex
	onContainerRemoved  ContainerRemovedHandler
	onContainerReleased ContainerReleasedHandler

	containerWaitLock sync.Mutex
	containerWait     map[string]chan struct{}

	dockerMgr docker.Manager
	fileRepo  filerepo.Repo
	sys       sys.SystemChecker
	redisCli  RedisClient
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
	Constraints       *docker.Constraints

	// Cache
	ContainerTTL       time.Duration
	ContainersCapacity int

	// File repo
	HostStoragePath string

	// Rate limiting
	StreamCapacityLimit    int64
	StreamPerMinuteQuantum int64
}

// DefaultOptions holds default options values for script runner.
var DefaultOptions = &Options{
	Concurrency:     2,
	MCPU:            2000,
	NodeIOPS:        150,
	MemoryMargin:    100 * 1024 * 1024,
	StreamMaxLength: 512 * 1024,

	CreateTimeout:    10 * time.Second,
	CreateRetryCount: 3,
	CreateRetrySleep: 500 * time.Millisecond,
	Constraints: &docker.Constraints{
		// CPU and IOPS limit is calculated based on concurrency.
		MemoryLimit:     200 * 1024 * 1024,
		MemorySwapLimit: 0,
		PidLimit:        32,
		NofileUlimit:    1024,
		StorageLimit:    "500M",
	},

	ContainerTTL:       45 * time.Minute,
	ContainersCapacity: 250,

	StreamCapacityLimit:    5 << 20,
	StreamPerMinuteQuantum: 1 << 20,

	HostStoragePath: "/home/codebox/storage",
}

// ContainerReleasedHandler is a callback function called whenever run has finished.
type ContainerReleasedHandler func(cont *Container, options *RunOptions)

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
	Async       uint32

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

func (ro *RunOptions) String() string {
	return fmt.Sprintf("{EntryPoint:%.25s, OutputLimit:%d, Timeout:%v}", ro.EntryPoint, ro.OutputLimit, ro.Timeout)
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
	// ErrSemaphoreNotAcquired signals a non critical error occurring for context timeouts and similar cases.
	ErrSemaphoreNotAcquired = errors.New("semaphore was not acquired successfully")
)

const (
	containerLabel = "workerId"
	wrapperMount   = "wrapper"
	wrapperCommand = "/app/wrapper/codewrapper"
	wrapperName    = "codewrapper"
	wrapperPathEnv = "WRAPPERPATH"

	userMount             = "code"
	environmentMount      = "env"
	environmentFileName   = "squashfs.img"
	defaultTimeout        = 30 * time.Second
	graceTimeout          = 3 * time.Second
	dockerTimeout         = 8 * time.Second
	dockerDownloadTimeout = 5 * time.Minute
	stdWriterPrefixLen    = 8
	containerLogFormat    = "stream:%s:%s:log"

	freezeCPUQuota  = 1000
	freezeCPUPeriod = 1000000
)

// NewRunner initializes a new script runner.
func NewRunner(options *Options, dockerMgr docker.Manager, checker sys.SystemChecker, repo filerepo.Repo, redisClient RedisClient) (*DockerRunner, error) {
	initOnceRunner.Do(func() {
		freeCPUCounter = expvar.NewInt("cpu")
	})
	mergo.Merge(options, DefaultOptions) // nolint - error not possible

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
	options.Constraints.CPULimit = int64((options.MCPU-dockerMgr.Options().ReservedMCPU)*1e6) / int64(options.Concurrency)
	// Use the default setting of 100ms, as is specified in:
	// https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt
	//    	cpu.cfs_period_us=100ms
	options.Constraints.CPUPeriod = int64(100 * time.Millisecond / time.Microsecond)
	options.Constraints.CPUQuota = options.Constraints.CPULimit * options.Constraints.CPUPeriod / 1e9
	options.Constraints.IOPSLimit = options.NodeIOPS / uint64(options.Concurrency)

	// Check memory requirements.
	if err := checker.CheckFreeMemory(uint64(options.Constraints.MemoryLimit)*uint64(options.Concurrency) + options.MemoryMargin); err != nil {
		return nil, err
	}

	containerCache := cache.NewLRUSetCache(&cache.Options{
		TTL:      options.ContainerTTL,
		Capacity: options.ContainersCapacity,
	})
	r := &DockerRunner{
		dockerMgr:      dockerMgr,
		fileRepo:       repo,
		sys:            checker,
		options:        *options,
		containerCache: containerCache,
		redisCli:       redisClient,
	}
	r.containerCache.OnValueEvicted(r.onEvictedContainerHandler)
	r.containerWait = make(map[string]chan struct{})

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

	for i := range cl {
		c := &cl[i]

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

func (r *DockerRunner) processRun(ctx context.Context, logger logrus.FieldLogger, runtime, requestID, sourceHash, environment, userID string, options *RunOptions) (*Result, *Container, bool, error) {
	start := time.Now()

	conn, cont, newContainer, err := r.getContainer(ctx, runtime, requestID, sourceHash, environment, userID, options)
	if err != nil {
		return nil, cont, newContainer, err
	}

	defer conn.Close()

	// Run.
	logger = logger.WithFields(logrus.Fields{
		"containerHash": cont.Hash,
		"new":           newContainer,
		"container":     cont,
	})
	logger.Info("Running in container")

	delim, err := cont.Run(conn, options)
	if err != nil {
		return nil, cont, newContainer, err
	}

	// Return processed response.
	ctx, cancel := context.WithTimeout(ctx, options.Timeout+graceTimeout)
	defer cancel()

	limit := int(options.OutputLimit)
	ret := &Result{
		ContainerID: cont.ID,
		Weight:      options.Weight,
	}

	var output []byte

	if options.Async <= 1 {
		output, ret.Stdout, ret.Stderr, err = r.processOutput(ctx, conn, cont.Stdout(), cont.Stderr(), delim, limit)
	} else {
		output, err = r.processOutputAsync(ctx, conn, limit)
	}

	// Parse result.
	ret.Took = time.Since(start)
	parseErr := ret.Parse(output, r.options.StreamMaxLength, err)

	if err == nil {
		err = parseErr
	}

	logger.WithField("ret", ret).Info("Run finished")

	return ret, cont, newContainer, err
}

func (r *DockerRunner) processOutput(ctx context.Context, conn, stdout, stderr io.Reader, delim string, limit int) (output, stdoutBytes, stderrBytes []byte, err error) {
	var (
		wg     sync.WaitGroup
		e1, e2 error
	)

	// In non async mode simply read stdout and stderr.
	wg.Add(2)

	go func() {
		stdoutBytes, e1 = util.ReadLimitedUntil(ctx, stdout, delim, r.options.StreamMaxLength)

		wg.Done()
	}()

	go func() {
		stderrBytes, e2 = util.ReadLimitedUntil(ctx, stderr, delim, r.options.StreamMaxLength)

		wg.Done()
	}()

	output, err = util.ReadLimited(ctx, conn, limit)

	wg.Wait()

	if err == nil {
		if e1 != nil {
			err = e1
		} else if e2 != nil {
			err = e2
		}
	}

	return output, stdoutBytes, stderrBytes, err
}

func (r *DockerRunner) processOutputAsync(ctx context.Context, conn io.Reader, limit int) ([]byte, error) {
	return util.ReadLimited(ctx, conn, limit)
}

// Run executes given script in a docker container.
func (r *DockerRunner) Run(ctx context.Context, logger logrus.FieldLogger, runtime, requestID, sourceHash, environment, userID string, options *RunOptions) (*Result, error) {
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
		"reqID":       requestID,
		"runtime":     runtime,
		"options":     options,
		"sourceHash":  sourceHash,
		"userID":      userID,
		"environment": environment,
		"weight":      options.Weight,
		"async":       options.Async,
	})

	if !r.IsRunning() {
		logger.WithError(ErrPoolNotRunning).Error("Run failed - pool is not running")
		return nil, ErrPoolNotRunning
	}

	start := time.Now()

	r.taskWaitGroup.Add(1)

	ret, cont, newContainer, err := r.processRun(ctx, logger, runtime, requestID, sourceHash, environment, userID, options)
	took := time.Since(start)

	if err != nil {
		logger = logger.WithError(err)
	}

	if ret != nil {
		ret.Cached = newContainer
		ret.Overhead = took - ret.Took
	}

	// Cleanup only if it's the last connection.
	go r.afterRun(runtime, cont, requestID, options, newContainer, err)

	return ret, err
}

func (r *DockerRunner) processContainerDone(runtime string, cont *Container, requestID string, options *RunOptions, newContainer bool, err error) {
	logger := logrus.WithField("container", cont)
	// Add new container if we took it from the pool.
	recreate := newContainer

	switch {
	case err == filerepo.ErrResourceNotFound:
		// If we encountered missing resource - reuse container.
		// If cleanup volume fails - cleanup whole container and recreate in next block. Otherwise - return it to pool.
		if err = r.fileRepo.CleanupVolume(cont.volumeKey); err == nil {
			r.containerPool[runtime] <- cont
			return
		}
	case err == ErrSemaphoreNotAcquired:
		// If semaphore was not acquired for new container, simply return it to pool.
		r.containerPool[runtime] <- cont
		return
	}

	// Release container resources.
	if e := r.releaseContainer(cont, requestID, options); e != nil && err == nil {
		err = e
	}

	// If we have an error at this point, stop accepting connections to container and mark it for cleanup.
	if err != nil {
		// Check for non critical errors.
		logger = logger.WithError(err)
		logFunc := logger.Warn

		if err != util.ErrLimitReached && err != io.EOF && err != context.DeadlineExceeded && err != context.Canceled {
			logFunc = logger.Error
		}

		if err == context.Canceled {
			recreate = false
		}

		logFunc("Recovering from container error")
		cont.StopAcceptingConnections()
		r.containerCache.Delete(cont.Hash, cont)
	}
	// Add new container if needed.
	if recreate {
		if err = r.recreateContainer(runtime); err != nil {
			// If we did fail to create a container, panic!
			panic(err)
		}
	}
}

func (r *DockerRunner) afterRun(runtime string, cont *Container, requestID string, options *RunOptions, newContainer bool, err error) {
	logger := logrus.WithError(err).WithField("container", cont)
	r.processContainerDone(runtime, cont, requestID, options, newContainer, err)

	if !cont.IsAcceptingConnections() && cont.ConnsNum() == 0 {
		r.cleanupContainer(cont)
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
}

func (r *DockerRunner) recreateContainer(runtime string) error {
	// Try to create a container, on fail - clean it up and retry.
	return util.Retry(r.options.CreateRetryCount, r.options.CreateRetrySleep, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), r.options.CreateTimeout)
		defer cancel()

		// Allow only 1 concurrent container creation as it is relatively heavy operation.
		r.createMu.Lock()
		newContainer, err := r.createFreshContainer(ctx, runtime)
		r.createMu.Unlock()

		if err != nil {
			r.cleanupContainer(newContainer)
			return err
		}

		r.containerPool[runtime] <- newContainer
		return nil
	})
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
	sem := make(chan struct{}, 4) // create only 4 at a time
	done := make(chan error, r.options.Concurrency)

	// Store codewrapper in repo.
	f, err := os.Open(os.Getenv(wrapperPathEnv)) // nolint: gosec
	util.Must(err)

	if _, err := r.fileRepo.PermStore(wrapperName, bufio.NewReader(f), wrapperName, os.ModePerm); err != nil {
		return "", err
	}

	// Process concurrently each runtime.
	for runtime, rInfo := range SupportedRuntimes {
		// Store wrappers.
		if _, err := r.fileRepo.PermStore(runtime, rInfo.Wrapper(), rInfo.FileName, 0); err != nil {
			return "", err
		}

		// Create containers.
		r.containerPool[runtime] = make(chan *Container, r.options.Concurrency)

		for i := uint(0); i < r.options.Concurrency; i++ {
			go func(runtime string) {
				sem <- struct{}{}

				defer func() {
					<-sem
				}()

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
	r.OnContainerReleased(nil)

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

func (r *DockerRunner) reserveContainer(ctx context.Context, cont *Container, connID string, newCont bool, options *RunOptions, constraints *docker.Constraints) error {
	err := cont.Reserve(connID, func(numConns int) error {
		if numConns == 1 {
			// If it's the first concurrent connection, acquire semaphore and adapt docker container if needed.
			if r.poolSemaphore.Acquire(ctx, int64(options.Weight)) != nil {
				return ErrSemaphoreNotAcquired
			}

			freeCPUCounter.Add(-int64(options.MCPU))

			// Set CPU/Memory resources for async container or if using higher weight for new non-async container.
			if options.Async > 1 || (newCont && options.Weight > 1) {
				ctx2, cancel := context.WithTimeout(ctx, dockerTimeout)
				defer cancel()

				if options.Async > 0 {
					logrus.WithField("container", cont).Info("Waking up container")
				}

				if err := r.dockerMgr.ContainerUpdate(ctx2, cont.ID, constraints); err != nil {
					return err
				}
			}
		}

		return nil
	})

	return err
}

func (r *DockerRunner) releaseContainer(cont *Container, requestID string, options *RunOptions) error {
	released := false

	if err := cont.Release(requestID, func(numConns int) error {
		if numConns == 0 {
			released = true

			r.poolSemaphore.Release(int64(options.Weight))
			freeCPUCounter.Add(int64(options.MCPU))

			// Freeze CPU resources for async container.
			if options.Async > 1 {
				ctx2, cancel := context.WithTimeout(context.Background(), dockerTimeout)
				defer cancel()

				logrus.WithField("container", cont).Info("Freezing container")

				if err := r.dockerMgr.ContainerUpdate(ctx2, cont.ID, &docker.Constraints{
					CPUPeriod:   freezeCPUPeriod,
					CPUQuota:    freezeCPUQuota,
					MemoryLimit: int64(options.Weight) * r.options.Constraints.MemoryLimit,
				}); err != nil {
					return err
				}
			}
		}

		return nil
	}); err != nil {
		return err
	}

	if released {
		r.muHandler.RLock()

		if r.onContainerReleased != nil {
			go r.onContainerReleased(cont, options)
		}

		r.muHandler.RUnlock()
	}

	return nil
}

func (r *DockerRunner) getContainer(ctx context.Context, runtime, requestID, sourceHash, environment, userID string, options *RunOptions) (conn io.ReadWriteCloser, cont *Container, newContainer bool, err error) { // nolint: gocyclo
	constraints := &docker.Constraints{
		CPUPeriod:   r.options.Constraints.CPUPeriod,
		CPUQuota:    int64(options.Weight) * r.options.Constraints.CPUQuota,
		MemoryLimit: int64(options.Weight) * r.options.Constraints.MemoryLimit,
	}

	// Try to get a container from cache.
	containerHash := fmt.Sprintf("%s/%s/%s/%x", sourceHash, userID, environment, util.Hash(options.EntryPoint))
	cacheVal := r.containerCache.Get(containerHash)

	for _, c := range cacheVal {
		cont = c.(*Container)

		// If container is in good standing, use it from cache.
		if cont.IsAcceptingConnections() && r.containerCache.Refresh(containerHash, c) {
			if err = r.reserveContainer(ctx, cont, requestID, false, options, constraints); err != nil {
				if err == ErrTooManyConnections || !cont.IsAcceptingConnections() {
					continue
				}

				return conn, cont, false, err
			}

			conn, err = cont.Conn()

			return conn, cont, false, err
		}
	}

	// With async, before using container from pool first make a lock to avoid multiple multi-conn containers at once.
	if options.Async > 1 {
		r.containerWaitLock.Lock()
		if ch, ok := r.containerWait[containerHash]; ok {
			r.containerWaitLock.Unlock()
			<-ch

			return r.getContainer(ctx, runtime, requestID, sourceHash, environment, userID, options)
		}

		ch := make(chan struct{})
		r.containerWait[containerHash] = ch

		r.containerWaitLock.Unlock()

		defer func() {
			r.containerWaitLock.Lock()
			delete(r.containerWait, containerHash)
			r.containerWaitLock.Unlock()
			close(ch)
		}()
	}

	// Fallback to pool.
	cont = <-r.containerPool[runtime]
	cont.Configure(options)
	cont.SourceHash = sourceHash
	cont.Environment = environment
	cont.UserID = userID
	cont.Hash = containerHash

	if err := r.reserveContainer(ctx, cont, requestID, true, options, constraints); err != nil {
		return conn, cont, true, err
	}

	// Linking sources.
	logger := logrus.WithFields(logrus.Fields{"container": cont, "runtime": runtime})

	if err = r.fileRepo.Link(cont.volumeKey, sourceHash, userMount); err != nil {
		logger.WithError(err).WithField("sourceHash", sourceHash).Error("Linking error")
		return conn, cont, true, err
	}

	// Linking environment.
	if environment != "" {
		if err = r.fileRepo.Mount(cont.volumeKey, environment, environmentFileName, environmentMount); err != nil {
			logger.WithError(err).WithField("environment", environment).Error("Mounting error")
			return conn, cont, true, err
		}
	}

	// Send setup message.
	if conn, err = cont.Setup(options, constraints); err != nil {
		return conn, cont, true, err
	}

	// In async container start stdout/err reader.
	if options.Async > 1 {
		lim := ratelimit.NewBucketWithQuantum(time.Minute, 5<<20, 1<<20)
		lim.TakeMaxDuration(1, 0)

		ch := fmt.Sprintf(containerLogFormat, userID, sourceHash)

		go util.SubscribeRateLimited(cont.stdout,
			lim,
			func(message []byte) {
				r.redisCli.Publish(ch, cont.ID+"\tO\t"+string(message))
			},
			func(err error) {
				if cont.IsAcceptingConnections() {
					logger.WithError(err).Warn("Stdout streaming error")
					cont.StopAcceptingConnections()
				}
			})

		go util.SubscribeRateLimited(cont.stderr,
			lim,
			func(message []byte) {
				r.redisCli.Publish(ch, cont.ID+"\tE\t"+string(message))
			},
			func(err error) {
				if cont.IsAcceptingConnections() {
					logger.WithError(err).Warn("Stderr streaming error")
					cont.StopAcceptingConnections()
				}
			})
	}

	// Add container to cache.
	r.containerCache.Add(containerHash, cont)

	return conn, cont, true, err
}

func (r *DockerRunner) createFreshContainer(ctx context.Context, runtime string) (*Container, error) {
	var err error

	cont := NewContainer(runtime)
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
		ctx, rInfo.Image, rInfo.User, []string{wrapperCommand},
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
	cont.resp, err = r.dockerMgr.ContainerAttach(ctx, cont.ID)
	if err != nil {
		return cont, err
	}

	err = r.dockerMgr.ContainerStart(ctx, cont.ID)
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
	cont.Stop()

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

	r.muHandler.RLock()
	if r.onContainerRemoved != nil {
		go r.onContainerRemoved(cont)
	}
	r.muHandler.RUnlock()
}

func (r *DockerRunner) onEvictedContainerHandler(key string, val interface{}) {
	cont := val.(*Container)
	// Do not cleanup container already being marked for clean up (doing after run of last connection).
	if !cont.IsAcceptingConnections() {
		return
	}

	cont.StopAcceptingConnections()

	if cont.ConnsNum() == 0 {
		r.cleanupContainer(cont)
	}
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

// OnContainerReleased sets an (optional) function that when container has been released.
// Set to nil to disable.
func (r *DockerRunner) OnContainerReleased(f ContainerReleasedHandler) {
	r.muHandler.Lock()
	r.onContainerReleased = f
	r.muHandler.Unlock()
}

func (r *DockerRunner) createWrapperVolume(runtime string) (volKey, volPath string, err error) {
	volKey, volPath, err = r.fileRepo.CreateVolume()
	if err != nil {
		return "", "", err
	}

	for _, resKey := range []string{wrapperName, runtime} {
		if err := r.fileRepo.Link(volKey, resKey, wrapperMount); err != nil {
			return "", "", err
		}
	}

	volRelPath, err := r.fileRepo.RelativePath(volPath)

	util.Must(err)

	volHostPath := filepath.Join(r.options.HostStoragePath, volRelPath)

	return volKey, volHostPath, nil
}
