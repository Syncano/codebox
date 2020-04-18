package lb

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/status"

	repomocks "github.com/Syncano/codebox/pkg/filerepo/mocks"
	repopb "github.com/Syncano/codebox/pkg/filerepo/proto"
	"github.com/Syncano/codebox/pkg/lb/mocks"
	pb "github.com/Syncano/codebox/pkg/lb/proto"
	scriptmocks "github.com/Syncano/codebox/pkg/script/mocks"
	scriptpb "github.com/Syncano/codebox/pkg/script/proto"
)

func TestMain(m *testing.M) {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(ioutil.Discard, ioutil.Discard, ioutil.Discard))
	logrus.SetOutput(ioutil.Discard)

	os.Exit(m.Run())
}

func TestServerMethods(t *testing.T) {
	err := errors.New("some error")

	Convey("Given server with mocked repo", t, func() {
		repo := new(repomocks.Repo)
		s := NewServer(repo, &ServerOptions{WorkerRetry: 1})

		Convey("given mocked Run stream, Run", func() {
			stream := new(mocks.ScriptRunner_RunServer)

			Convey("does nothing when meta is not received", func() {
				stream.On("Recv").Return(nil, io.EOF).Once()
				e := s.Run(stream)
				So(e, ShouldBeNil)
			})
			Convey("propagates error on Recv", func() {
				stream.On("Recv").Return(nil, err).Once()
				e := s.Run(stream)
				So(e, ShouldEqual, err)
			})
			Convey("given valid request", func() {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				stream.On("Context").Return(ctx)
				validReq1 := &pb.RunRequest{
					Value: &pb.RunRequest_Meta{
						Meta: &pb.RunRequest_MetaMessage{
							RequestID: "reqID",
						},
					},
				}
				validReq2 := &pb.RunRequest{
					Value: &pb.RunRequest_Request{
						Request: &scriptpb.RunRequest{
							Value: &scriptpb.RunRequest_Meta{
								Meta: &scriptpb.RunRequest_MetaMessage{
									RequestID:  "reqID",
									Runtime:    "runtime",
									SourceHash: "hash",
								},
							},
						},
					},
				}
				validReq3 := &pb.RunRequest{
					Value: &pb.RunRequest_Request{
						Request: &scriptpb.RunRequest{
							Value: &scriptpb.RunRequest_Chunk{
								Chunk: &scriptpb.RunRequest_ChunkMessage{
									Name: "key",
									Data: []byte("value"),
								},
							},
						},
					},
				}

				Convey("returns error when no workers are available", func() {
					stream.On("Recv").Return(validReq1, nil).Once()
					stream.On("Recv").Return(validReq2, nil).Once()
					stream.On("Recv").Return(validReq3, nil).Once()
					stream.On("Recv").Return(nil, io.EOF).Once()
					repo.On("Get", "hash").Return("/path")

					e := s.Run(stream)
					So(e, ShouldEqual, ErrNoWorkersAvailable)
				})
				Convey("given mocked worker in server", func() {
					repoCli := new(repomocks.RepoClient)
					scriptCli := new(scriptmocks.ScriptRunnerClient)
					conn, _ := grpc.Dial("localhost", grpc.WithInsecure())
					worker := Worker{
						ID:         "id",
						mCPU:       1,
						alive:      true,
						repoCli:    repoCli,
						scriptCli:  scriptCli,
						containers: make(map[string]*WorkerContainer),
						scripts:    make(map[ScriptInfo]int),
						conn:       conn,
						metrics:    Metrics(),
					}
					stdout := []byte("stdout")
					s.workers.Set("id", &worker)

					Convey("given meta with drained concurrency and cancelled context", func() {
						cancel()
						s.limiter.Lock(context.Background(), "ckey", 1)
						stream.On("Recv").Return(&pb.RunRequest{
							Value: &pb.RunRequest_Meta{
								Meta: &pb.RunRequest_MetaMessage{
									RequestID:        "reqID",
									ConcurrencyKey:   "ckey",
									ConcurrencyLimit: 1,
								},
							},
						}, nil).Once()
						stream.On("Recv").Return(validReq2, nil).Once()
						stream.On("Recv").Return(validReq3, nil).Once()
						stream.On("Recv").Return(nil, io.EOF).Once()
						repo.On("Get", "hash").Return("/path")
						e := s.Run(stream)
						So(e, ShouldResemble, status.Error(codes.ResourceExhausted, context.Canceled.Error()))
					})
					Convey("given meta, unlocks when done", func() {
						stream.On("Recv").Return(&pb.RunRequest{
							Value: &pb.RunRequest_Meta{
								Meta: &pb.RunRequest_MetaMessage{
									ConcurrencyKey:   "ckey",
									ConcurrencyLimit: 1,
								},
							},
						}, nil).Once()
						stream.On("Recv").Return(validReq2, nil).Once()
						stream.On("Recv").Return(validReq3, nil).Once()
						stream.On("Recv").Return(nil, io.EOF).Once()
						repo.On("Get", "hash").Return("/path")
						repoCli.On("Exists", mock.Anything, mock.Anything).Return(nil, err)
						e := s.Run(stream)
						So(e, ShouldEqual, err)
						So(s.limiter.Lock(ctx, "ckey", 1), ShouldBeNil)
					})
					Convey("given simple valid request", func() {
						stream.On("Recv").Return(validReq1, nil).Once()
						stream.On("Recv").Return(validReq2, nil).Once()
						stream.On("Recv").Return(validReq3, nil).Once()
						stream.On("Recv").Return(nil, io.EOF).Once()

						Convey("given successful repo Get", func() {
							repo.On("Get", "hash").Return("/path")
							runStream := new(scriptmocks.ScriptRunner_RunClient)

							Convey("propagates and handles source exists error", func() {
								repoCli.On("Exists", mock.Anything, mock.Anything).Return(nil, err)

								e := s.Run(stream)
								So(e, ShouldEqual, err)
							})
							Convey("given source that exists on worker", func() {
								repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: true}, nil)

								scriptCli.On("Run", mock.Anything).Return(runStream, nil).Once()
								runStream.On("Send", mock.Anything).Return(nil).Twice()
								runStream.On("CloseSend").Return(nil).Once()

								Convey("propagates worker.Run error", func() {
									runStream.On("Recv").Return(nil, err).Once()
									e := s.Run(stream)
									So(e, ShouldEqual, err)
									So(s.workers.Contains(worker.ID), ShouldBeTrue)
								})
								Convey("removes worker if error threshold has been exceeded", func() {
									s.options.WorkerErrorThreshold = 1
									runStream.On("Recv").Return(nil, err).Once()
									e := s.Run(stream)
									So(e, ShouldEqual, err)
									So(s.workers.Contains(worker.ID), ShouldBeFalse)
								})
								Convey("doesn't remove worker on context.canceled error", func() {
									s.options.WorkerErrorThreshold = 1
									runStream.On("Recv").Return(nil, context.Canceled)
									e := s.Run(stream)
									So(e, ShouldEqual, context.Canceled)
									So(s.workers.Contains(worker.ID), ShouldBeTrue)
								})
								Convey("proceeds on successful worker.Run", func() {
									runStream.On("Recv").Return(&scriptpb.RunResponse{Stdout: stdout, Cached: true}, nil).Once()
									runStream.On("Recv").Return(nil, io.EOF).Once()

									Convey("propagates client Send error", func() {
										stream.On("Send", mock.Anything).Return(err)
										e := s.Run(stream)
										So(e, ShouldEqual, err)
									})
									Convey("proceeds with sending", func() {
										stream.On("Send", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
											So(args.Get(0).(*scriptpb.RunResponse).Stdout, ShouldResemble, stdout)
										})
										e := s.Run(stream)
										So(e, ShouldBeNil)
										So(worker.FreeCPU(), ShouldEqual, 0)
										So(len(worker.scripts), ShouldEqual, 1) // Cached was true.
									})
								})
							})
							Convey("given not existing source, uploads it", func() {
								repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil)
								memfs := afero.NewMemMapFs()
								afero.WriteFile(memfs, "/path/file.js", []byte("content"), os.ModePerm)
								repo.On("GetFS").Return(memfs)

								Convey("propagates upload source error", func() {
									repoCli.On("Upload", mock.Anything).Return(nil, err)
									e := s.Run(stream)
									So(e, ShouldEqual, err)
								})
								Convey("proceeds on successful upload source", func() {
									uploadStream := new(repomocks.Repo_UploadClient)
									repoCli.On("Upload", mock.Anything).Return(uploadStream, nil)
									uploadStream.On("Send", mock.Anything).Return(nil).Times(3)
									uploadStream.On("Recv", mock.Anything).Return(&repopb.UploadResponse{Accepted: true}, nil).Twice()

									scriptCli.On("Run", mock.Anything).Return(runStream, nil).Once()
									runStream.On("Send", mock.Anything).Return(nil).Twice()
									runStream.On("CloseSend").Return(nil).Once()
									runStream.On("Recv").Return(&scriptpb.RunResponse{Stdout: stdout}, nil).Once()
									runStream.On("Recv").Return(nil, io.EOF).Once()

									stream.On("Send", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
										So(args.Get(0).(*scriptpb.RunResponse).Stdout, ShouldResemble, stdout)
									})

									e := s.Run(stream)
									So(e, ShouldBeNil)
									So(worker.FreeCPU(), ShouldEqual, 0)
									So(len(worker.scripts), ShouldEqual, 0) // Cached was false.
								})
							})

							runStream.AssertExpectations(t)
						})
						Convey("returns error when hash is not in filerepo", func() {
							repo.On("Get", "hash").Return("").Once()
							e := s.Run(stream)
							So(e, ShouldEqual, ErrSourceNotAvailable)
						})
					})

					mock.AssertExpectationsForObjects(t, repoCli, scriptCli)
				})

				stream.AssertExpectations(t)
			})
		})

		Convey("Options returns a copy of options struct", func() {
			So(s.Options(), ShouldNotEqual, s.options)
			So(s.Options(), ShouldResemble, s.options)
		})

		Convey("given some Container, grabWorker", func() {
			ci := ScriptInfo{SourceHash: "hash", UserID: "user"}

			Convey("returns worker with max free slots", func() {
				s.workers.Set("id1", NewWorker("id1", net.TCPAddr{}, 1, 1, 128, s.metrics))
				s.workers.Set("id2", NewWorker("id2", net.TCPAddr{}, 2, 2, 128, s.metrics))
				s.workers.Set("id3", NewWorker("id2", net.TCPAddr{}, 1, 2, 128, s.metrics))

				wi, conns, fromCache := s.grabWorker(ci)
				So(wi.Worker.ID, ShouldEqual, "id2")
				So(conns, ShouldEqual, 1)
				So(fromCache, ShouldBeFalse)
			})
			Convey("prefers worker from container cache if available and slots>0", func() {
				w2 := NewWorker("id2", net.TCPAddr{}, 1, 1, 128, s.metrics)
				s.workers.Set("id1", NewWorker("id1", net.TCPAddr{}, 2, 2, 128, s.metrics))
				s.workers.Set("id2", w2)

				w2.AddCache(s.workerContainerCache, ci, "id2", &WorkerContainer{Worker: w2})
				w, conns, fromCache := s.grabWorker(ci)
				So(w.Worker.ID, ShouldEqual, "id2")
				So(conns, ShouldEqual, 1)
				So(fromCache, ShouldBeTrue)
			})
			Convey("skips worker from container cache if it's missing from cache", func() {
				w2 := NewWorker("id2", net.TCPAddr{}, 1, 1, 128, s.metrics)
				s.workers.Set("id1", NewWorker("id1", net.TCPAddr{}, 2, 2, 128, s.metrics))

				w2.AddCache(s.workerContainerCache, ci, "id1", &WorkerContainer{Worker: w2})
				w, conns, fromCache := s.grabWorker(ci)
				So(w.Worker.ID, ShouldEqual, "id1")
				So(conns, ShouldEqual, 1)
				So(fromCache, ShouldBeFalse)
			})
			Convey("prefers worker from container cache with higher free cpu/conns", func() {
				w1 := NewWorker("id1", net.TCPAddr{}, 2, 2, 128, s.metrics)
				w2 := NewWorker("id2", net.TCPAddr{}, 1, 2, 128, s.metrics)
				s.workers.Set("id1", w1)
				s.workers.Set("id2", w2)

				w1.AddCache(s.workerContainerCache, ci, "id1", &WorkerContainer{Worker: w1})
				w2.AddCache(s.workerContainerCache, ci, "id2", &WorkerContainer{Worker: w2})
				w, conns, fromCache := s.grabWorker(ci)
				So(w.Worker.ID, ShouldEqual, "id1")
				So(conns, ShouldEqual, 1)
				So(fromCache, ShouldBeTrue)
				w1.RemoveCache(s.workerContainerCache, ci, "id1")
				So(s.workerContainerCache[ci], ShouldHaveLength, 1)
			})
			Convey("returns nil if there are no workers", func() {
				w, _, _ := s.grabWorker(ci)
				So(w, ShouldBeNil)
			})
		})

		Convey("findWorkerWithMaxFreeCPU returns nil if there are no workers", func() {
			wi := s.findWorkerWithMaxFreeCPU()
			So(wi, ShouldBeNil)
		})
		Convey("findWorkerWithMaxFreeCPU returns worker even if free CPU is negative", func() {
			wi := NewWorker("id1", net.TCPAddr{}, 1, 1, 128, s.metrics)
			wi.freeCPU = -10
			s.workers.Set("id1", wi)

			wi = s.findWorkerWithMaxFreeCPU()
			So(wi.ID, ShouldEqual, "id1")
		})
		Convey("findWorkerWithMaxFreeCPU finds a worker with highest free CPU", func() {
			s.workers.Set("id1", NewWorker("id1", net.TCPAddr{}, 1, 1, 128, s.metrics))
			s.workers.Set("id2", NewWorker("id2", net.TCPAddr{}, 2, 2, 128, s.metrics))
			s.workers.Set("id3", NewWorker("id3", net.TCPAddr{}, 1, 1, 128, s.metrics))

			wi := s.findWorkerWithMaxFreeCPU()
			So(wi.ID, ShouldEqual, "id2")
		})

		Convey("ContainerRemoved returns unregistered error on unknown id", func() {
			_, e := s.ContainerRemoved(context.Background(), &pb.ContainerRemovedRequest{Id: "id1"})
			So(e, ShouldEqual, ErrUnknownWorkerID)
		})
		Convey("Heartbeat returns unregistered error on unknown id", func() {
			_, e := s.Heartbeat(context.Background(), &pb.HeartbeatRequest{Id: "id1"})
			So(e, ShouldEqual, ErrUnknownWorkerID)
		})
		Convey("Disconnect silently succeeds on unknown id", func() {
			_, e := s.Disconnect(context.Background(), &pb.DisconnectRequest{Id: "id1"})
			So(e, ShouldBeNil)
		})

		Convey("Register adds a new worker", func() {
			_, e := s.Register(context.Background(), &pb.RegisterRequest{Id: "id1", Port: 123, MCPU: 2000, Memory: 1000})
			So(e, ShouldBeNil)

			wi := s.workers.Get("id1").(*Worker)
			So(wi, ShouldNotBeNil)
			So(wi.Addr.Port, ShouldEqual, 123)

			Convey("Heartbeat succeeds", func() {
				_, e := s.Heartbeat(context.Background(), &pb.HeartbeatRequest{Id: "id1"})
				So(e, ShouldBeNil)
			})
			Convey("given container in cache", func() {
				ci := ScriptInfo{SourceHash: "hash", UserID: "user"}
				wc := &WorkerContainer{Worker: wi}
				wi.AddCache(s.workerContainerCache, ci, "id1", wc)
				So(wi.scripts, ShouldNotBeEmpty)
				So(s.workerContainerCache, ShouldNotBeEmpty)

				Convey("ContainerRemoved removes container from cache if refcount gets to 0", func() {
					_, e := s.ContainerRemoved(context.Background(),
						&pb.ContainerRemovedRequest{Id: "id1", SourceHash: ci.SourceHash, UserID: ci.UserID})
					So(e, ShouldBeNil)
					So(wi.scripts, ShouldBeEmpty)
					So(s.workerContainerCache, ShouldBeEmpty)
				})
				Convey("ContainerRemoved keeps container in cache if refcount > 1", func() {
					wc := &WorkerContainer{Worker: wi}
					wi.AddCache(s.workerContainerCache, ci, "id1", wc)
					So(len(wi.scripts), ShouldEqual, 1)

					_, e := s.ContainerRemoved(context.Background(),
						&pb.ContainerRemovedRequest{Id: "id1", SourceHash: ci.SourceHash, UserID: ci.UserID})
					So(e, ShouldBeNil)

					So(len(wi.scripts), ShouldEqual, 1)
					So(s.workerContainerCache, ShouldNotBeEmpty)
					So(s.workerContainerCache[ci], ShouldContainKey, "id1")
				})
				Convey("Disconnect removes worker and all containers from cache", func() {
					_, e := s.Disconnect(context.Background(), &pb.DisconnectRequest{Id: "id1"})
					So(e, ShouldBeNil)
					So(s.workers.Get("id1"), ShouldBeNil)
					So(s.workerContainerCache, ShouldBeEmpty)
				})

			})
		})

		Convey("given mocked ResponseWriter, ReadyHandler", func() {
			req, _ := http.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(s.ReadyHandler)

			Convey("writes 200 if worker count is satisfied", func() {
				handler.ServeHTTP(rr, req)
				So(rr.Code, ShouldEqual, http.StatusOK)
			})
			Convey("writes 400 otherwise", func() {
				s.options.WorkerMinReady = 1
				handler.ServeHTTP(rr, req)
				So(rr.Code, ShouldEqual, http.StatusBadRequest)
			})
		})

		s.Shutdown()
		repo.AssertExpectations(t)
	})
}
