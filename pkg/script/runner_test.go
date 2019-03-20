package script

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"expvar"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	"golang.org/x/sync/semaphore"

	"github.com/Syncano/codebox/pkg/cache"
	"github.com/Syncano/codebox/pkg/docker"
	dockermock "github.com/Syncano/codebox/pkg/docker/mocks"
	"github.com/Syncano/codebox/pkg/filerepo"
	repomock "github.com/Syncano/codebox/pkg/filerepo/mocks"
	"github.com/Syncano/codebox/pkg/sys"
	sysmock "github.com/Syncano/codebox/pkg/sys/mocks"
	"github.com/Syncano/codebox/pkg/util"
)

type MockConn struct {
	mock.Mock
	net.TCPConn
}

func (m *MockConn) Read(b []byte) (int, error) {
	ret := m.Called(b)
	return ret.Get(0).(int), ret.Error(1)
}

func (m *MockConn) Write(b []byte) (int, error) {
	ret := m.Called(b)
	return ret.Get(0).(int), ret.Error(1)
}

func (m *MockConn) Close() error {
	return m.Called().Error(0)
}

type mockReader struct{}

func (mr *mockReader) Read(p []byte) (int, error) {
	time.Sleep(30 * time.Second)
	return 0, io.EOF
}

type mockReadCloser struct {
	bytes.Buffer
}

func (mc *mockReadCloser) Close() error { return nil }

func TestNewRunner(t *testing.T) {
	logrus.SetOutput(ioutil.Discard)
	logrus.SetLevel(logrus.DebugLevel)

	Convey("Given mocked docker manager, sys checker and filerepo, NewRunner", t, func() {
		dockerMgr := new(dockermock.Manager)
		checker := new(sysmock.SystemChecker)
		repo := new(repomock.Repo)
		redisCli := new(MockRedisClient)
		opts := Options{Concurrency: 1, CreateRetrySleep: 1 * time.Millisecond, MCPU: 1000}

		err := errors.New("some error")
		defaultRuntime := "nodejs_v6"
		volKey := "volkey"
		volPath := "volpath"
		volRelPath := "volrelpath"
		envKey := "env"
		cID := "cid"

		mc := new(MockConn)
		mc.On("Close").Return(nil)
		stdoutConn := new(MockConn)
		stderrConn := new(MockConn)
		conn := new(MockConn)
		yamuxSession := new(MockYamuxSession)
		yamuxSession.On("Open").Return(conn, nil)
		yamuxSession.On("Close").Return(nil)

		cont := &Container{
			ID:          cID,
			Environment: envKey,
			Runtime:     defaultRuntime,
			ok:          1,
			volumeKey:   volKey,
			resp: types.HijackedResponse{
				Conn:   mc,
				Reader: bufio.NewReader(bytes.NewBuffer([]byte(`127.0.0.1:1000\n`))),
			},
			session:   yamuxSession,
			stdout:    stdoutConn,
			stderr:    stderrConn,
			connLimit: 2,
			conns:     make(map[string]struct{}),
		}
		dockerMgr.On("Info").Return(types.Info{NCPU: 1})
		dockerMgr.On("Options").Return(docker.Options{})

		Convey("sets MCPU based on docker Info", func() {
			checker.On("CheckFreeMemory", mock.Anything).Return(nil).Once()

			opts.MCPU = 2000
			r, e := NewRunner(opts, dockerMgr, checker, repo, redisCli)
			So(e, ShouldBeNil)
			So(r.Options().MCPU, ShouldEqual, 1000)
		})

		Convey("sets up everything", func() {
			checker.On("CheckFreeMemory", mock.Anything).Return(nil).Once()

			r, e := NewRunner(opts, dockerMgr, checker, repo, redisCli)
			So(e, ShouldBeNil)
			So(r.IsRunning(), ShouldBeFalse)

			Convey("Run runs script in container", func() {
				Convey("fails when pool is not running", func() {
					_, e := r.Run(context.Background(), logrus.StandardLogger(), defaultRuntime, "hash", "", "user", &RunOptions{})
					So(e, ShouldEqual, ErrPoolNotRunning)
				})
				Convey("fails on unsupported runtime", func() {
					_, e := r.Run(context.Background(), logrus.StandardLogger(), "runtime", "hash", "", "user", &RunOptions{})
					So(e, ShouldEqual, ErrUnsupportedRuntime)
				})
				Convey("given fake running pool", func() {
					r.setRunning(true)
					r.poolSemaphore = semaphore.NewWeighted(int64(opts.Concurrency + 1))
					r.containerPool = make(map[string]chan *Container)
					r.containerPool[defaultRuntime] = make(chan *Container, opts.Concurrency)
					mc.On("Close").Return(nil)

					Convey("runs script in container", func() {
						checker.On("CheckFreeMemory", uint64(0)).Return(nil).Once()
						conn.On("Write", mock.Anything).Return(0, nil)
						conn.On("Read", mock.Anything).Return(0, io.EOF)
						conn.On("Close").Return(nil)
						stdoutConn.On("Write", mock.Anything).Return(0, nil)
						stdoutConn.On("Read", mock.Anything).Return(0, io.EOF)
						stderrConn.On("Read", mock.Anything).Return(0, io.EOF)
						dockerMgr.On("ContainerStop", mock.Anything, cont.ID).Return(nil)
						repo.On("DeleteVolume", mock.Anything).Return(nil)

						Convey("without files", func() {
							Convey("from pool", func() {
								r.containerPool[defaultRuntime] <- cont
								repo.On("Link", volKey, mock.Anything, mock.Anything).Return(nil)
								repo.On("Mount", volKey, mock.Anything, environmentFileName, environmentMount).Return(nil)

								// mocks for afterRun's createFreshContainer.
								cID2 := "cID2"
								repo.On("CreateVolume").Return(volKey, volPath, nil).Once()
								repo.On("RelativePath", volPath).Return(volRelPath, nil).Once()
								dockerMgr.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
									mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(cID2, nil).Once()
								dockerMgr.On("ContainerStart", mock.Anything, cID2).Return(nil).Once()
								dockerMgr.On("ContainerAttach", mock.Anything, cID2).Return(cont.resp, nil).Once()

								_, e := r.Run(context.Background(), logrus.StandardLogger(), defaultRuntime, "hash", envKey, "user", &RunOptions{})
								So(e, ShouldEqual, io.EOF)
								cont2 := <-r.containerPool[defaultRuntime]
								So(cont2.ID, ShouldEqual, cID2)
							})
							Convey("from cache", func() {
								r.containerCache.Set(fmt.Sprintf("hash/user//%x", util.Hash("main.js")), cont)
								_, e := r.Run(context.Background(), logrus.StandardLogger(), defaultRuntime, "hash", "", "user", &RunOptions{})
								So(e, ShouldEqual, io.EOF)
							})
						})
						Convey("with files", func() {
							files := map[string]File{"file": {Data: []byte("content")}}
							r.containerCache.Set(fmt.Sprintf("hash/user//%x", util.Hash("main.js")), cont)
							_, e := r.Run(context.Background(), logrus.StandardLogger(), defaultRuntime, "hash", "", "user", &RunOptions{Files: files})
							So(e, ShouldEqual, io.EOF)
						})

						r.taskWaitGroup.Wait()
					})
					Convey("propagates semaphore Acquire error and recovers from it", func() {
						checker.On("CheckFreeMemory", uint64(0)).Return(nil).Once()
						r.containerPool[defaultRuntime] <- cont
						ctx, cancel := context.WithCancel(context.Background())
						cancel()
						_, e := r.Run(ctx, logrus.StandardLogger(), defaultRuntime, "hash", "", "run", &RunOptions{MCPU: 10000})
						So(e, ShouldResemble, ErrSemaphoreNotAcquired)
						cont2 := <-r.containerPool[defaultRuntime]
						So(cont2.ID, ShouldEqual, cID)
					})
					Convey("propagates and cleans up errors", func() {
						files := map[string]File{"file": {Data: []byte("content")}}
						r.containerPool[defaultRuntime] <- cont
						expectedErr := err
						env := ""
						opts := &RunOptions{Files: files}
						ctx := context.Background()

						Convey("ContainerUpdate error", func() {
							opts.MCPU = 2000
							dockerMgr.On("ContainerUpdate", mock.Anything, cID, mock.Anything).Return(err).Once()
						})

						Convey("Link source error", func() {
							repo.On("Link", volKey, mock.Anything, mock.Anything).Return(err).Once()
						})
						Convey("Link environment error", func() {
							env = "env"
							repo.On("Link", volKey, mock.Anything, mock.Anything).Return(nil).Once()
							repo.On("Mount", volKey, mock.Anything, mock.Anything, mock.Anything).Return(err).Once()
						})

						Convey("for container conn dial", func() {
							cont.session = nil
							cont.addr = "invalid"
							expectedErr = &net.OpError{Net: "tcp", Op: "dial",
								Err: &net.AddrError{Err: "missing port in address", Addr: cont.addr}}
						})

						// mocks for afterRun's cleanupContainer.
						dockerMgr.On("ContainerStop", mock.Anything, cID).Return(nil).Once()
						repo.On("DeleteVolume", volKey).Return(nil).Once()

						// mocks for afterRun's createFreshContainer.
						cID2 := "cID2"
						checker.On("CheckFreeMemory", uint64(0)).Return(nil).Once()
						repo.On("CreateVolume").Return(volKey, volPath, nil).Once()
						repo.On("Link", volKey, mock.Anything, mock.Anything).Return(nil)
						repo.On("RelativePath", volPath).Return(volRelPath, nil).Once()
						dockerMgr.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
							mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(cID2, nil).Once()
						dockerMgr.On("ContainerStart", mock.Anything, cID2).Return(nil).Once()
						dockerMgr.On("ContainerAttach", mock.Anything, cID2).Return(cont.resp, nil).Once()

						doneCh := make(chan bool)
						r.OnContainerRemoved(func(*Container) {
							doneCh <- true
						})

						_, e := r.Run(ctx, logrus.StandardLogger(), defaultRuntime, "hash", env, "run", opts)
						So(e, ShouldResemble, expectedErr)
						cont2 := <-r.containerPool[defaultRuntime]
						So(cont2.ID, ShouldEqual, cID2)
						So(<-doneCh, ShouldBeTrue)
					})
				})
			})

			Convey("CreatePool manages pool of containers", func() {
				Convey("propagates ReserveMemory error", func() {
					checker.On("Reset").Once()
					checker.On("ReserveMemory", r.options.MemoryMargin).Return(err)
					_, e := r.CreatePool()
					So(e, ShouldEqual, err)
				})
				Convey("proceeds on succesful ReserveMemory", func() {
					checker.On("Reset").Once()
					checker.On("ReserveMemory", r.options.MemoryMargin).Return(nil).Once()
					_, filename, _, _ := runtime.Caller(1)
					os.Setenv(wrapperPathEnv, filename)

					Convey("propagates ReserveMemory error", func() {
						checker.On("ReserveMemory", uint64(r.options.Constraints.MemoryLimit)).Return(sys.ErrNotEnoughMemory{})
						_, e := r.CreatePool()
						_, ok := e.(sys.ErrNotEnoughMemory)
						So(ok, ShouldBeTrue)
					})
					Convey("propagates PermStore error", func() {
						repo.On("PermStore", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", err)
						checker.On("ReserveMemory", uint64(r.options.Constraints.MemoryLimit)).Return(nil)
						_, e := r.CreatePool()
						So(e, ShouldEqual, err)
					})
					Convey("propagates createFreshContainer error", func() {
						repo.On("PermStore", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", nil)
						checker.On("ReserveMemory", uint64(r.options.Constraints.MemoryLimit)).Return(nil)
						repo.On("CreateVolume").Return(volKey, volPath, err)
						_, e := r.CreatePool()
						So(e, ShouldEqual, err)
					})
					Convey("proceeds with no error", func() {
						repo.On("PermStore", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", nil)
						checker.On("ReserveMemory", uint64(r.options.Constraints.MemoryLimit)).Return(nil)
						repo.On("CreateVolume").Return(volKey, volPath, nil)
						repo.On("Link", volKey, mock.Anything, wrapperMount).Return(nil)
						repo.On("RelativePath", volPath).Return(volRelPath, nil)

						dockerMgr.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
							mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(cID, nil)
						dockerMgr.On("ContainerStart", mock.Anything, cID).Return(nil)
						dockerMgr.On("ContainerAttach", mock.Anything, cID).Return(cont.resp, nil)

						id, e := r.CreatePool()
						So(e, ShouldBeNil)
						So(id, ShouldNotBeBlank)
						So(r.IsRunning(), ShouldBeTrue)
						mock.AssertExpectationsForObjects(t, dockerMgr, checker, repo)

						// Trying to create again fails.
						_, e = r.CreatePool()
						So(e, ShouldEqual, ErrPoolAlreadyCreated)

						// StopPool stops everything.
						expectedCount := len(r.containerPool) * int(opts.Concurrency)
						dockerMgr.On("ContainerStop", mock.Anything, cID).Return(nil).Times(expectedCount)
						repo.On("DeleteVolume", volKey).Return(nil).Times(expectedCount)
						r.StopPool()
						So(r.IsRunning(), ShouldBeFalse)
					})
				})
			})

			SkipConvey("onEvictedHandler gets called on container removal from cache", func() {
				cont := &Container{ID: "someId", volumeKey: "someKey", SourceHash: "sourceHash", UserID: "userID"}
				cont.StopAcceptingConnections()
				r.containerCache.Set("hash", cont)
				dockerMgr.On("ContainerStop", mock.Anything, "someId").Return(nil).Once()
				repo.On("DeleteVolume", "someKey").Return(nil).Once()

				// Calls OnContainerRemoved handler.
				contCh := make(chan *Container)
				r.OnContainerRemoved(func(cont *Container) {
					contCh <- cont
				})

				r.containerCache.Flush()
				cont2 := <-contCh
				time.Sleep(5 * time.Millisecond)
				So(cont.SourceHash, ShouldEqual, cont2.SourceHash)
				So(cont.UserID, ShouldEqual, cont2.UserID)
				So(cont.Environment, ShouldEqual, cont2.Environment)
			})

			Convey("createFreshContainer given mocked dependencies", func() {

				Convey("propagates CreateVolume error", func() {
					repo.On("CreateVolume").Return(volKey, volPath, err)
					_, e := r.createFreshContainer(context.Background(), defaultRuntime)
					So(e, ShouldEqual, err)
				})
				Convey("proceeds after CreateVolume", func() {
					repo.On("CreateVolume").Return(volKey, volPath, nil)

					Convey("propagates Link error", func() {
						repo.On("Link", volKey, mock.Anything, wrapperMount).Return(err)
						_, e := r.createFreshContainer(context.Background(), defaultRuntime)
						So(e, ShouldEqual, err)
					})
					Convey("proceeds after Link", func() {
						repo.On("Link", volKey, mock.Anything, wrapperMount).Return(nil)
						repo.On("RelativePath", volPath).Return(volRelPath, nil)

						Convey("propagates ContainerCreate error", func() {
							dockerMgr.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
								mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", err)
							_, e := r.createFreshContainer(context.Background(), defaultRuntime)
							So(e, ShouldEqual, err)
						})
						Convey("proceeeds after ContainerCreate", func() {
							dockerMgr.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
								mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(cID, nil)

							Convey("propagates ContainerAttach error", func() {
								dockerMgr.On("ContainerAttach", mock.Anything, cID).Return(types.HijackedResponse{}, err)
								_, e := r.createFreshContainer(context.Background(), defaultRuntime)
								So(e, ShouldEqual, err)
							})
							Convey("propagates ContainerStart error", func() {
								dockerMgr.On("ContainerAttach", mock.Anything, cID).Return(types.HijackedResponse{
									Reader: bufio.NewReader(new(mockReader)),
								}, nil)
								dockerMgr.On("ContainerStart", mock.Anything, cID).Return(err)
								_, e := r.createFreshContainer(context.Background(), defaultRuntime)
								So(e, ShouldEqual, err)
							})
							Convey("propagates context deadline error", func() {
								dockerMgr.On("ContainerAttach", mock.Anything, cID).Return(types.HijackedResponse{
									Reader: bufio.NewReader(new(mockReader)),
								}, nil)
								dockerMgr.On("ContainerStart", mock.Anything, cID).Return(nil)
								ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
								_, e := r.createFreshContainer(ctx, defaultRuntime)
								cancel()
								So(e, ShouldResemble, context.DeadlineExceeded)
							})
						})

					})
				})
			})
		})
		Convey("propagates CheckFreeMemory error", func() {
			checker.On("CheckFreeMemory", mock.Anything).Return(err)

			r, e := NewRunner(opts, dockerMgr, checker, repo, redisCli)
			So(e, ShouldEqual, err)
			So(r, ShouldBeNil)
		})

		mock.AssertExpectationsForObjects(t, dockerMgr, checker, repo)
	})
}

func TestRunnerMethods(t *testing.T) {
	logrus.SetOutput(ioutil.Discard)
	initOnceRunner.Do(func() {
		freeCPUCounter = expvar.NewInt("cpu")
	})

	Convey("Given Runner with mocked docker manager, sys checker and filerepo", t, func() {
		dockerMgr := new(dockermock.Manager)
		checker := new(sysmock.SystemChecker)
		repo := new(repomock.Repo)
		opts := DefaultOptions
		opts.CreateRetrySleep = 1 * time.Millisecond
		r := DockerRunner{sys: checker,
			fileRepo:       repo,
			dockerMgr:      dockerMgr,
			options:        opts,
			containerCache: cache.NewLRUSetCache(cache.Options{}),
		}
		err := errors.New("some error")

		Convey("Options returns a copy of options struct", func() {
			So(r.Options(), ShouldNotEqual, r.options)
			So(r.Options(), ShouldResemble, r.options)
		})

		Convey("CleanupUnused cleans file repo and docker", func() {
			Convey("panics on container list error", func() {
				dockerMgr.On("ListContainersByLabel", mock.Anything, containerLabel).Return([]types.Container{}, err)
				So(r.CleanupUnused, ShouldPanicWith, err)
			})
			Convey("lists containers", func() {
				dockerMgr.On("ListContainersByLabel", mock.Anything, containerLabel).Return(
					[]types.Container{{ID: "someid", Labels: map[string]string{containerLabel: "abc"}}}, nil)

				// Proceeds on stop error.
				dockerMgr.On("ContainerStop", mock.Anything, "someid").Return(err).Once()
				repo.On("CleanupUnused").Once()

				Convey("respects prune config", func() {
					Convey("skips pruning if not enabled", func() {
						r.CleanupUnused()
					})
					Convey("if enabled", func() {
						r.options.PruneImages = true

						Convey("prunes", func() {
							dockerMgr.On("PruneImages", mock.Anything).Return(types.ImagesPruneReport{}, nil)
							r.CleanupUnused()
						})
						Convey("panics on error", func() {
							dockerMgr.On("PruneImages", mock.Anything).Return(types.ImagesPruneReport{}, err)
							So(r.CleanupUnused, ShouldPanicWith, err)
						})

					})
				})
			})
		})

		Convey("DownloadAllImages processes all supported images", func() {
			Convey("downloads properly", func() {
				dockerMgr.On("DownloadImage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				e := r.DownloadAllImages()
				So(e, ShouldBeNil)
			})
			Convey("propagates downloading error", func() {
				dockerMgr.On("DownloadImage", mock.Anything, mock.Anything, mock.Anything).Return(err)
				e := r.DownloadAllImages()
				So(e, ShouldEqual, err)
			})
		})

		Convey("afterRun panics if it cannot recreate container after retries", func() {
			t := time.Now()
			repo.On("CreateVolume").Return("", "", err).Times(r.options.CreateRetryCount)
			r.poolSemaphore = semaphore.NewWeighted(int64(opts.Concurrency))
			r.poolSemaphore.Acquire(context.Background(), 1)

			So(func() { r.afterRun("runtime", &Container{}, "", &RunOptions{}, true, err) }, ShouldPanicWith, err)
			So(time.Since(t), ShouldBeGreaterThan, time.Duration(r.options.CreateRetryCount-1)*r.options.CreateRetrySleep)
		})
		Convey("given an initialized container", func() {
			cont := &Container{ID: "someid", volumeKey: "volKey", connLimit: 2, conns: make(map[string]struct{})}
			r.containerPool = make(map[string]chan *Container)
			r.poolSemaphore = semaphore.NewWeighted(int64(opts.Concurrency))
			r.poolSemaphore.Acquire(context.Background(), 1)
			r.containerPool["runtime"] = make(chan *Container, 1)
			r.taskWaitGroup.Add(1)
			cont.Reserve("abc", func(int) error { return nil })

			Convey("afterRun returns container to cache if resource was missing", func() {
				repo.On("CleanupVolume", "volKey").Return(nil).Once()
				checker.On("CheckFreeMemory", mock.Anything).Return(nil)
				r.afterRun("runtime", cont, "abc", &RunOptions{Weight: 1, MCPU: 1}, false, filerepo.ErrResourceNotFound)
				So((<-r.containerPool["runtime"]).ID, ShouldEqual, cont.ID)
			})
			Convey("afterRun panics on full memory when delete lru fails", func() {
				checker.On("CheckFreeMemory", mock.Anything).Return(sys.ErrNotEnoughMemory{}).Once()
				repo.On("CleanupVolume", "volKey").Return(nil).Once()
				So(func() { r.afterRun("runtime", cont, "abc", &RunOptions{}, false, filerepo.ErrResourceNotFound) }, ShouldPanic)
			})
			Convey("afterRun tries to delete containers on full memory", func() {
				r.containerCache.Set("hash", cont)
				repo.On("CleanupVolume", "volKey").Return(nil).Once()
				checker.On("CheckFreeMemory", mock.Anything).Return(sys.ErrNotEnoughMemory{}).Once()
				checker.On("CheckFreeMemory", mock.Anything).Return(nil)
				r.afterRun("runtime", cont, "abc", &RunOptions{}, false, filerepo.ErrResourceNotFound)
				So((<-r.containerPool["runtime"]).ID, ShouldEqual, cont.ID)
			})
		})

		Convey("cleanupContainer silently proceeds on errors", func() {
			dockerMgr.On("ContainerStop", mock.Anything, "someId").Return(err).Once()
			repo.On("DeleteVolume", "volKey").Return(err).Once()
			r.cleanupContainer(&Container{ID: "someId", volumeKey: "volKey"})
		})

		r.Shutdown()
		mock.AssertExpectationsForObjects(t, dockerMgr, checker, repo)
	})
}
