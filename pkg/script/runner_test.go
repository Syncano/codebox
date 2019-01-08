package script

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"

	"github.com/Syncano/codebox/pkg/cache"
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

func (m *MockConn) Close() error {
	return m.Called().Error(0)
}

type MockReadWriteCloser struct {
	mock.Mock
	bytes.Buffer
}

func (m *MockReadWriteCloser) Write(b []byte) (int, error) {
	ret := m.Called(b)
	return ret.Get(0).(int), ret.Error(1)
}

func (m *MockReadWriteCloser) Close() error {
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
		opts := Options{Concurrency: 1, CreateRetrySleep: 1 * time.Millisecond}
		dockerMgr.On("SetLimits", opts.Concurrency, DefaultOptions.NodeIOPS).Once()

		err := errors.New("some error")
		defaultRuntime := "nodejs_v6"
		volKey := "volkey"
		volPath := "volpath"
		volRelPath := "volrelpath"
		envKey := "env"
		cID := "cid"

		mrwc := new(MockReadWriteCloser)
		mrwc.Buffer.Write([]byte{3, 1, 0, 0, 0, 0})
		mc := new(MockConn)
		mc.On("Close").Return(nil)

		cont := &Container{ID: cID,
			volumeKey: volKey,
			resp: types.HijackedResponse{
				Conn:   mc,
				Reader: bufio.NewReader(bytes.NewBuffer([]byte(`127.0.0.1:1000\n`))),
			},
			conn:        mrwc,
			Environment: envKey,
		}

		Convey("sets up everything", func() {
			checker.On("CheckFreeMemory", mock.Anything).Return(nil).Once()

			r, e := NewRunner(opts, dockerMgr, checker, repo)
			So(e, ShouldBeNil)
			So(r.IsRunning(), ShouldBeFalse)

			Convey("Run runs script in container", func() {
				Convey("fails when pool is not running", func() {
					_, e := r.Run(logrus.StandardLogger(), defaultRuntime, "hash", "", "user", RunOptions{})
					So(e, ShouldEqual, ErrPoolNotRunning)
				})
				Convey("fails on unsupported runtime", func() {
					_, e := r.Run(logrus.StandardLogger(), "runtime", "hash", "", "user", RunOptions{})
					So(e, ShouldEqual, ErrUnsupportedRuntime)
				})
				Convey("given fake running pool", func() {
					r.setRunning(true)
					r.taskPool = make(chan bool, opts.Concurrency)
					r.taskPool <- true
					r.containerPool = make(map[string]chan *Container)
					r.containerPool[defaultRuntime] = make(chan *Container, opts.Concurrency)
					mc.On("Close").Return(nil)

					Convey("runs script in container", func() {
						checker.On("CheckFreeMemory", uint64(0)).Return(nil).Once()

						Convey("without files", func() {
							// Expect 3 writes (total len, context len, context)
							mrwc.On("Write", mock.Anything).Return(0, nil).Times(3)

							Convey("from pool", func() {
								r.containerPool[defaultRuntime] <- cont
								repo.On("Link", volKey, mock.Anything, mock.Anything).Return(nil)
								repo.On("Mount", volKey, mock.Anything, environmentFileName, environmentMount).Return(nil)

								// mocks for afterRun's createFreshContainer.
								cID2 := "cID2"
								repo.On("CreateVolume").Return(volKey, volPath, nil).Once()
								repo.On("RelativePath", volPath).Return(volRelPath, nil).Once()
								dockerMgr.On("CreateContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
									mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(cID2, nil).Once()
								dockerMgr.On("StartContainer", mock.Anything, cID2).Return(nil).Once()
								dockerMgr.On("AttachContainer", mock.Anything, cID2).Return(cont.resp, nil).Once()

								_, e := r.Run(logrus.StandardLogger(), defaultRuntime, "hash", envKey, "user", RunOptions{})
								So(e, ShouldBeNil)
								cont2 := <-r.containerPool[defaultRuntime]
								So(cont2.ID, ShouldEqual, cID2)
							})
							Convey("from cache", func() {
								r.containerCache.Push(fmt.Sprintf("hash/user//%x", util.Hash("main.js")), cont)
								_, e := r.Run(logrus.StandardLogger(), defaultRuntime, "hash", "", "user", RunOptions{})
								So(e, ShouldBeNil)
							})
						})
						Convey("with files", func() {
							files := map[string]File{"file": {Data: []byte("content")}}
							// Expect 3 writes (total len, context len, context)
							mrwc.On("Write", mock.Anything).Return(0, nil).Times(3)
							r.containerCache.Push(fmt.Sprintf("hash/user//%x", util.Hash("main.js")), cont)
							// And then expect a file content.
							mrwc.On("Write", files["file"].Data).Return(0, nil).Once()
							_, e := r.Run(logrus.StandardLogger(), defaultRuntime, "hash", "", "user", RunOptions{Files: files})
							So(e, ShouldBeNil)
						})

						r.taskWaitGroup.Wait()
					})
					Convey("propagates and cleans up errors", func() {
						files := map[string]File{"file": {Data: []byte("content")}}
						r.containerPool[defaultRuntime] <- cont
						expectedErr := err
						env := ""

						Convey("Link source error", func() {
							repo.On("Link", volKey, mock.Anything, mock.Anything).Return(err).Once()
						})
						Convey("Link environment error", func() {
							env = "env"
							repo.On("Link", volKey, mock.Anything, mock.Anything).Return(nil).Once()
							repo.On("Mount", volKey, mock.Anything, mock.Anything, mock.Anything).Return(err).Once()
						})

						Convey("for conn Write", func() {
							mrwc.On("Write", mock.Anything).Return(0, err).Once()
						})
						Convey("for files conn Write", func() {
							mrwc.On("Write", mock.Anything).Return(0, nil).Times(3)
							mrwc.On("Write", mock.Anything).Return(0, err).Once()
						})
						Convey("for container conn dial", func() {
							cont.conn = nil
							cont.addr = "invalid"
							expectedErr = &net.OpError{Net: "tcp", Op: "dial",
								Err: &net.AddrError{Err: "missing port in address", Addr: cont.addr}}
						})
						Convey("gets error log from container on crash", func() {
							mrwc.On("Write", mock.Anything).Return(0, nil)
							mrwc.Buffer.Reset()
							mrwc.Buffer.Write([]byte{3, 1, 0, 0, 0})
							mrc := new(mockReadCloser)
							dockerMgr.On("ContainerErrorLog", mock.Anything, cID).Return(mrc, nil).Once()
							expectedErr = io.EOF
						})
						Convey("returns malformed header on non existing mux", func() {
							mrwc.On("Write", mock.Anything).Return(0, nil)
							mrwc.Buffer.Reset()
							mrwc.Buffer.Write([]byte{4, 1, 0, 0, 0, 0})
							mrwc.On("Close", mock.Anything).Return(nil)
							expectedErr = ErrMalformedHeader
						})

						// mocks for afterRun's cleanupContainer.
						dockerMgr.On("StopContainer", mock.Anything, cID).Return(nil).Once()
						repo.On("DeleteVolume", volKey).Return(nil).Once()

						// mocks for afterRun's createFreshContainer.
						cID2 := "cID2"
						checker.On("CheckFreeMemory", uint64(0)).Return(nil).Once()
						repo.On("CreateVolume").Return(volKey, volPath, nil).Once()
						repo.On("Link", volKey, mock.Anything, mock.Anything).Return(nil)
						repo.On("RelativePath", volPath).Return(volRelPath, nil).Once()
						dockerMgr.On("CreateContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
							mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(cID2, nil).Once()
						dockerMgr.On("StartContainer", mock.Anything, cID2).Return(nil).Once()
						dockerMgr.On("AttachContainer", mock.Anything, cID2).Return(cont.resp, nil).Once()

						slotCh := make(chan bool)
						r.OnSlotReady(func() {
							slotCh <- true
						})

						_, e := r.Run(logrus.StandardLogger(), defaultRuntime, "hash", env, "run", RunOptions{Files: files})
						So(e, ShouldResemble, expectedErr)
						cont2 := <-r.containerPool[defaultRuntime]
						So(cont2.ID, ShouldEqual, cID2)
						So(<-slotCh, ShouldBeTrue)
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

					Convey("propagates PermStore error", func() {
						repo.On("PermStore", mock.Anything, mock.Anything, mock.Anything).Return("", err)
						checker.On("ReserveMemory", r.options.MemoryLimit).Return(nil)
						_, e := r.CreatePool()
						So(e, ShouldEqual, err)
					})
					Convey("propagates ReserveMemory error", func() {
						checker.On("ReserveMemory", r.options.MemoryLimit).Return(sys.ErrNotEnoughMemory{})
						_, e := r.CreatePool()
						_, ok := e.(sys.ErrNotEnoughMemory)
						So(ok, ShouldBeTrue)
					})
					Convey("propagates createFreshContainer error", func() {
						repo.On("PermStore", mock.Anything, mock.Anything, mock.Anything).Return("", nil)
						checker.On("ReserveMemory", r.options.MemoryLimit).Return(nil)
						repo.On("CreateVolume").Return(volKey, volPath, err)
						_, e := r.CreatePool()
						So(e, ShouldEqual, err)
					})
					Convey("proceeds with no error", func() {
						repo.On("PermStore", mock.Anything, mock.Anything, mock.Anything).Return("", nil)
						checker.On("ReserveMemory", r.options.MemoryLimit).Return(nil)
						repo.On("CreateVolume").Return(volKey, volPath, nil)
						repo.On("Link", volKey, mock.Anything, wrapperMount).Return(nil)
						repo.On("RelativePath", volPath).Return(volRelPath, nil)

						dockerMgr.On("CreateContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
							mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(cID, nil)
						dockerMgr.On("StartContainer", mock.Anything, cID).Return(nil)
						dockerMgr.On("AttachContainer", mock.Anything, cID).Return(cont.resp, nil)

						id, e := r.CreatePool()
						So(e, ShouldBeNil)
						So(id, ShouldNotBeBlank)
						So(r.IsRunning(), ShouldBeTrue)
						mock.AssertExpectationsForObjects(t, dockerMgr, checker, repo)

						// Trying to create again fails.
						_, e = r.CreatePool()
						So(e, ShouldEqual, ErrPoolAlreadyCreated)

						// StopPool stops everything.
						expectedCount := len(r.containerPool)
						dockerMgr.On("StopContainer", mock.Anything, cID).Return(nil).Times(expectedCount)
						repo.On("DeleteVolume", volKey).Return(nil).Times(expectedCount)
						r.StopPool()
						So(r.IsRunning(), ShouldBeFalse)
					})
				})
			})

			Convey("onEvictedHandler gets called on container removal from cache", func() {
				r.containerCache.Push("hash",
					&Container{ID: "someId", volumeKey: "someKey", SourceHash: "sourceHash", UserID: "userID"})
				dockerMgr.On("StopContainer", mock.Anything, "someId").Return(nil).Once()
				repo.On("DeleteVolume", "someKey").Return(nil).Once()

				// Calls OnContainerRemoved handler.
				contCh := make(chan *Container)
				r.OnContainerRemoved(func(cont *Container) {
					contCh <- cont
				})

				r.containerCache.Flush()
				cont := <-contCh
				time.Sleep(5 * time.Millisecond)
				So(cont.SourceHash, ShouldEqual, "sourceHash")
				So(cont.UserID, ShouldEqual, "userID")
				So(cont.Environment, ShouldEqual, "")
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

						Convey("propagates CreateContainer error", func() {
							dockerMgr.On("CreateContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
								mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", err)
							_, e := r.createFreshContainer(context.Background(), defaultRuntime)
							So(e, ShouldEqual, err)
						})
						Convey("proceeeds after CreateContainer", func() {
							dockerMgr.On("CreateContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
								mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(cID, nil)

							Convey("propagates StartContainer error", func() {
								dockerMgr.On("StartContainer", mock.Anything, cID).Return(err)
								_, e := r.createFreshContainer(context.Background(), defaultRuntime)
								So(e, ShouldEqual, err)
							})
							Convey("proceeds after StartContainer", func() {
								dockerMgr.On("StartContainer", mock.Anything, cID).Return(nil)

								Convey("propagates AttachContainer error", func() {
									dockerMgr.On("AttachContainer", mock.Anything, cID).Return(types.HijackedResponse{}, err)
									_, e := r.createFreshContainer(context.Background(), defaultRuntime)
									So(e, ShouldEqual, err)
								})
								Convey("propagates context deadline error", func() {
									dockerMgr.On("AttachContainer", mock.Anything, cID).Return(types.HijackedResponse{
										Reader: bufio.NewReader(new(mockReader)),
									}, nil)
									ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
									_, e := r.createFreshContainer(ctx, defaultRuntime)
									cancel()
									So(e, ShouldResemble, context.DeadlineExceeded)
								})
							})
						})

					})
				})
			})
		})
		Convey("propagates CheckFreeMemory error", func() {
			checker.On("CheckFreeMemory", mock.Anything).Return(err)

			r, e := NewRunner(opts, dockerMgr, checker, repo)
			So(e, ShouldEqual, err)
			So(r, ShouldBeNil)
		})

		mock.AssertExpectationsForObjects(t, dockerMgr, checker, repo)
	})
}

func TestRunnerMethods(t *testing.T) {
	logrus.SetOutput(ioutil.Discard)

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
			containerCache: cache.NewStackCache(cache.Options{}),
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
				dockerMgr.On("StopContainer", mock.Anything, "someid").Return(err).Once()
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

			So(func() { r.afterRun("runtime", &Container{}, false, err) }, ShouldPanicWith, err)
			So(time.Since(t), ShouldBeGreaterThan, time.Duration(r.options.CreateRetryCount-1)*r.options.CreateRetrySleep)
		})
		Convey("afterRun returns container to cache if resource was missing", func() {
			cont := &Container{ID: "someid", volumeKey: "volKey"}
			repo.On("CleanupVolume", "volKey").Return(nil).Once()
			checker.On("CheckFreeMemory", mock.Anything).Return(nil)
			r.containerPool = make(map[string]chan *Container)
			r.taskPool = make(chan bool, 1)
			r.containerPool["runtime"] = make(chan *Container, 1)
			r.taskWaitGroup.Add(1)
			r.afterRun("runtime", cont, false, filerepo.ErrResourceNotFound)
			So((<-r.containerPool["runtime"]).ID, ShouldEqual, cont.ID)
		})
		Convey("afterRun panics on full memory when delete lru fails", func() {
			cont := &Container{ID: "someid", volumeKey: "volKey"}
			checker.On("CheckFreeMemory", mock.Anything).Return(sys.ErrNotEnoughMemory{}).Once()
			repo.On("CleanupVolume", "volKey").Return(nil).Once()
			r.containerPool = make(map[string]chan *Container)
			r.containerPool["runtime"] = make(chan *Container, 1)
			So(func() { r.afterRun("runtime", cont, false, filerepo.ErrResourceNotFound) }, ShouldPanic)
		})
		Convey("afterRun tries to delete containers on full memory", func() {
			r.containerCache.Push("hash", &Container{ID: "otherid", volumeKey: "volKey"})
			cont := &Container{ID: "someid", volumeKey: "volKey"}
			repo.On("CleanupVolume", "volKey").Return(nil).Once()
			checker.On("CheckFreeMemory", mock.Anything).Return(sys.ErrNotEnoughMemory{}).Once()
			checker.On("CheckFreeMemory", mock.Anything).Return(nil)
			r.containerPool = make(map[string]chan *Container)
			r.taskPool = make(chan bool, 1)
			r.containerPool["runtime"] = make(chan *Container, 1)
			r.taskWaitGroup.Add(1)
			r.afterRun("runtime", cont, false, filerepo.ErrResourceNotFound)
			So((<-r.containerPool["runtime"]).ID, ShouldEqual, cont.ID)
		})

		Convey("cleanupContainer silently proceeds on errors", func() {
			dockerMgr.On("StopContainer", mock.Anything, "someId").Return(err).Once()
			repo.On("DeleteVolume", "volKey").Return(err).Once()
			r.cleanupContainer(&Container{ID: "someId", volumeKey: "volKey"})
		})

		r.Shutdown()
		mock.AssertExpectationsForObjects(t, dockerMgr, checker, repo)
	})
}

func TestContainerMethods(t *testing.T) {
	Convey("Given net server,", t, func() {
		l, _ := net.Listen("tcp", "127.0.0.1:3000")
		defer l.Close()
		c := &Container{addr: "127.0.0.1:3000"}

		Convey("Conn() connects and returns conn", func() {
			go c.Conn()
			l.Accept()
			c, err := c.Conn()
			So(c, ShouldNotBeNil)
			So(err, ShouldBeNil)
		})
	})
}
