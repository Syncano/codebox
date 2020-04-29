package broker

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	redis "github.com/go-redis/redis/v7"
	"github.com/sirupsen/logrus"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	"github.com/vmihailenco/msgpack/v4"
	"google.golang.org/grpc/grpclog"

	"github.com/Syncano/codebox/pkg/broker/mocks"
	brokerpb "github.com/Syncano/codebox/pkg/broker/proto"
	"github.com/Syncano/codebox/pkg/celery"
	celerymocks "github.com/Syncano/codebox/pkg/celery/mocks"
	repomocks "github.com/Syncano/codebox/pkg/filerepo/mocks"
	repopb "github.com/Syncano/codebox/pkg/filerepo/proto"
	lbmocks "github.com/Syncano/codebox/pkg/lb/mocks"
	lbpb "github.com/Syncano/codebox/pkg/lb/proto"
	"github.com/Syncano/codebox/pkg/script"
	scriptpb "github.com/Syncano/codebox/pkg/script/proto"
	"github.com/Syncano/codebox/pkg/util"
	utilmocks "github.com/Syncano/codebox/pkg/util/mocks"
)

var (
	amqpCh *celerymocks.AMQPChannel
)

func TestMain(m *testing.M) {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(ioutil.Discard, ioutil.Discard, ioutil.Discard))
	logrus.SetOutput(ioutil.Discard)

	amqpCh = new(celerymocks.AMQPChannel)
	celery.Init(amqpCh)

	os.Exit(m.Run())
}

func TestServerMethods(t *testing.T) {
	err := errors.New("some error")
	queue := "codebox"

	Convey("Given server with mocked channel and redis client", t, func() {
		redisCli := new(MockRedisClient)
		lbCli := new(lbmocks.ScriptRunnerClient)
		repoCli := new(repomocks.RepoClient)
		downloader := new(utilmocks.Downloader)
		stream := new(mocks.ScriptRunner_RunServer)

		s, e := NewServer(redisCli, &ServerOptions{
			LBAddr:              []string{"127.0.0.1"},
			LBRetry:             0,
			DownloadConcurrency: 1,
		})
		So(e, ShouldBeNil)
		So(len(s.lbServers), ShouldEqual, 1)

		s.downloader = downloader
		s.lbServers[0].conn.Close()
		s.lbServers[0].lbCli = lbCli
		s.lbServers[0].repoCli = repoCli

		Convey("Options returns a copy of options struct", func() {
			So(s.Options(), ShouldNotEqual, s.options)
			So(s.Options(), ShouldResemble, s.options)
		})
		Convey("Run returns error on invalid request", func() {
			stream.On("Context").Return(context.Background())
			e := s.Run(new(brokerpb.RunRequest), stream)
			So(e, ShouldResemble, ErrInvalidArgument)
		})
		Convey("Run processes on valid request", func() {
			sourceHash := "abc"
			env := "cba"
			envURL := "http://google.com"

			runReq := brokerpb.RunRequest{
				LbMeta: &lbpb.RunRequest_MetaMessage{},
				Meta: &brokerpb.RunRequest_MetaMessage{
					EnvironmentURL: envURL,
					Trace:          []byte("\"trace\""),
					TraceID:        123,
				},
				Request: []*scriptpb.RunRequest{
					{
						Value: &scriptpb.RunRequest_Meta{
							Meta: &scriptpb.RunRequest_MetaMessage{
								Environment: env,
								SourceHash:  sourceHash,
							},
						},
					},
				},
			}

			Convey("when file does not exist, downloads it", func() {
				stream.On("Context").Return(context.Background())
				repoCli.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil).Once()
				uploadStream := new(repomocks.Repo_UploadClient)
				dlCh := make(chan *util.DownloadResult, 1)
				repoCli.On("Upload", mock.Anything).Return(uploadStream, nil)
				// Cast to receive only channel.
				var retCh <-chan *util.DownloadResult = dlCh
				downloader.On("Download", mock.Anything, mock.Anything).Return(retCh, nil)

				Convey("on upload Send error, propagates it", func() {
					uploadStream.On("Send", mock.Anything).Return(err)
					repoCli.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil).Once()
					e := s.Run(&runReq, stream)
					So(errors.Is(e, err), ShouldBeTrue)
				})
				Convey("given simple download result channel", func() {
					dlCh <- &util.DownloadResult{Data: []byte("abc")}
					close(dlCh)

					Convey("on Recv error, propagates it", func() {
						uploadStream.On("Send", mock.Anything).Return(nil).Once()
						uploadStream.On("Recv").Return(nil, err).Once()
						repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil).Once()
						e := s.Run(&runReq, stream)
						So(errors.Is(e, err), ShouldBeTrue)
					})
					Convey("on last Recv error, propagates it", func() {
						uploadStream.On("Send", mock.Anything).Return(nil)
						uploadStream.On("Recv").Return(&repopb.UploadResponse{Accepted: true}, nil).Once()
						uploadStream.On("Recv").Return(nil, err).Once()
						repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil).Once()
						e := s.Run(&runReq, stream)
						So(errors.Is(e, err), ShouldBeTrue)
					})
					Convey("when upload is not accepted, moves to next file", func() {
						uploadStream.On("Send", mock.Anything).Return(nil).Once()
						uploadStream.On("Recv").Return(&repopb.UploadResponse{Accepted: false}, nil).Once()
						repoCli.On("Exists", mock.Anything, &repopb.ExistsRequest{Key: env}, mock.Anything).Return(nil, err).Once()
						e := s.Run(&runReq, stream)
						So(errors.Is(e, err), ShouldBeTrue)
					})
					Convey("on uploadChunks error, rechecks Exists and moves to next file if succeeds", func() {
						uploadStream.On("Send", mock.Anything).Return(nil).Once()
						uploadStream.On("Recv").Return(nil, err).Once()
						repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: true}, nil).Once()
						repoCli.On("Exists", mock.Anything, &repopb.ExistsRequest{Key: env}, mock.Anything).Return(nil, err).Once()
						e := s.Run(&runReq, stream)
						So(errors.Is(e, err), ShouldBeTrue)
					})
					Convey("on chunk Send error, propagates it", func() {
						uploadStream.On("Send", mock.Anything).Return(nil).Once()
						uploadStream.On("Recv").Return(&repopb.UploadResponse{Accepted: true}, nil).Once()
						uploadStream.On("Send", mock.Anything).Return(err)
						repoCli.On("Exists", mock.Anything, mock.Anything).Return(nil, err).Once()
						e := s.Run(&runReq, stream)
						So(errors.Is(e, err), ShouldBeTrue)
					})
					Convey("on chunk upload Send error, propagates it", func() {
						uploadStream.On("Send", mock.Anything).Return(nil).Once()
						uploadStream.On("Recv").Return(&repopb.UploadResponse{Accepted: true}, nil).Once()
						uploadStream.On("Send", mock.Anything).Return(err)
						repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil).Once()
						e := s.Run(&runReq, stream)
						So(errors.Is(e, err), ShouldBeTrue)
					})
					Convey("on upload Done Send error, propagates it", func() {
						uploadStream.On("Send", mock.Anything).Return(nil).Once()
						uploadStream.On("Recv").Return(&repopb.UploadResponse{Accepted: true}, nil).Once()
						uploadStream.On("Send", mock.Anything).Return(nil).Once()
						uploadStream.On("Send", mock.Anything).Return(err)
						repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil).Once()
						e := s.Run(&runReq, stream)
						So(errors.Is(e, err), ShouldBeTrue)
					})
				})
				Convey("on successful upload Send", func() {
					repoCli.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil).Once()
					uploadStream.On("Send", mock.Anything).Return(nil)
					uploadStream.On("Recv").Return(&repopb.UploadResponse{Accepted: true}, nil)

					Convey("on successful download", func() {
						runStream := new(lbmocks.ScriptRunner_RunClient)
						lbCli.On("Run", mock.Anything).Return(runStream, nil)
						runStream.On("Send", mock.Anything).Return(nil)
						runStream.On("CloseSend", mock.Anything).Return(nil, nil)
						ch := make(chan bool)
						runStream.On("Recv", mock.Anything).Return(&scriptpb.RunResponse{
							Code:     100,
							Response: &scriptpb.HTTPResponseMessage{},
						}, err).Run(func(args mock.Arguments) {
							ch <- true
						})

						dlCh <- &util.DownloadResult{Data: []byte("abc")}
						close(dlCh)

						Convey("ignores json marshall error on celery", func() {
							runReq.Meta.Trace = []byte("trace")
							e := s.Run(&runReq, stream)
							So(e, ShouldBeNil)
						})
						Convey("runs request correctly and publishes results", func() {
							amqpCh.On("Publish", mock.Anything, queue, mock.Anything, mock.Anything, mock.Anything).Return(nil)
							e := s.Run(&runReq, stream)
							So(e, ShouldBeNil)
						})
						Convey("runs request and skips publish for empty trace", func() {
							runReq.Meta.Trace = nil
							e := s.Run(&runReq, stream)
							So(e, ShouldBeNil)
						})

						<-ch
						mock.AssertExpectationsForObjects(t, uploadStream, runStream)
					})
					Convey("on failed download, propagates error", func() {
						dlCh <- &util.DownloadResult{Error: err}
						close(dlCh)
						e := s.Run(&runReq, stream)
						So(errors.Is(e, err), ShouldBeTrue)
					})
				})
			})
			Convey("propagates Exists error", func() {
				repoCli.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(nil, err)
				stream.On("Context").Return(context.Background())
				e := s.Run(&runReq, stream)
				So(errors.Is(e, err), ShouldBeTrue)
			})
			Convey("propagates Exists error on environment upload", func() {
				repoCli.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: true}, nil).Once()
				repoCli.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(nil, err).Once()
				stream.On("Context").Return(context.Background())
				e := s.Run(&runReq, stream)
				So(errors.Is(e, err), ShouldBeTrue)
			})
			Convey("propagates Upload error", func() {
				repoCli.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil)
				repoCli.On("Upload", mock.Anything).Return(nil, err)
				stream.On("Context").Return(context.Background())
				e := s.Run(&runReq, stream)
				So(errors.Is(e, err), ShouldBeTrue)
			})
			Convey("if concurrent upload happens, wait for it to finish", func() {
				repoCli.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil)
				doneCh := make(chan struct{}, 1)
				s.uploads["127.0.0.1;abc"] = doneCh
				close(doneCh)
				e := s.uploadFiles(context.Background(), s.lbServers[0], "abc", nil)
				So(e, ShouldBeNil)
			})
			Convey("if file exists, proceeds", func() {
				repoCli.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: true}, nil)
				stream.On("Context").Return(context.Background())

				Convey("and propagates Run error", func() {
					lbCli.On("Run", mock.Anything).Return(nil, err)
					e := s.Run(&runReq, stream)
					So(errors.Is(e, err), ShouldBeTrue)
				})
				Convey("and propagates Send error", func() {
					runStream := new(lbmocks.ScriptRunner_RunClient)
					lbCli.On("Run", mock.Anything).Return(runStream, nil)
					runStream.On("Send", mock.Anything).Return(err)
					e := s.Run(&runReq, stream)
					So(errors.Is(e, err), ShouldBeTrue)
				})
				Convey("and propagates Send request error", func() {
					runStream := new(lbmocks.ScriptRunner_RunClient)
					lbCli.On("Run", mock.Anything).Return(runStream, nil)
					runStream.On("Send", mock.Anything).Return(nil).Once()
					runStream.On("Send", mock.Anything).Return(err)
					e := s.Run(&runReq, stream)
					So(errors.Is(e, err), ShouldBeTrue)
				})
			})
			Convey("runs in sync when sync flag is true", func() {
				repoCli.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: true}, nil)
				stream.On("Context").Return(context.Background())
				runStream := new(lbmocks.ScriptRunner_RunClient)
				lbCli.On("Run", mock.Anything).Return(runStream, nil)
				runStream.On("Send", mock.Anything).Return(nil)
				runStream.On("CloseSend", mock.Anything).Return(nil, nil)
				res := &scriptpb.RunResponse{}
				runStream.On("Recv", mock.Anything).Return(res, nil).Once()
				runStream.On("Recv", mock.Anything).Return(nil, io.EOF)

				runReq.Meta.Sync = true

				Convey("proceeds when no errors occur", func() {
					stream.On("Send", res).Return(nil).Once()
				})
				Convey("silently logs ret stream Send error", func() {
					stream.On("Send", res).Return(err).Once()
				})
				e := s.Run(&runReq, stream)
				So(e, ShouldBeNil)
			})

		})

		Convey("given test http environment", func() {
			instancePk := 1
			tracePk := 2
			payloadKey := "payload_key"
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(s.RunHandler)

			req, _ := http.NewRequest("GET", "/", nil)

			Convey("returns correct CORS headers", func() {
				req.Header.Set("HTTP_ORIGIN", "http://onet.pl")
				handler.ServeHTTP(rr, req)
				So(rr.Header().Get("Access-Control-Allow-Origin"), ShouldEqual, "*")
			})
			Convey("returns error on invalid request", func() {
				handler.ServeHTTP(rr, req)
				So(rr.Code, ShouldEqual, http.StatusInternalServerError)
			})
			Convey("returns error on invalid header", func() {
				req.Header.Set("TRACE_PK", "-1")
				req.Header.Set("PAYLOAD_KEY", payloadKey)
				redisCli.On("Get", payloadKey).Return(redis.NewStringResult("{}", nil))
				redisCli.On("Del", payloadKey).Return(redis.NewIntCmd(0, nil))
				handler.ServeHTTP(rr, req)
				So(rr.Code, ShouldEqual, http.StatusInternalServerError)
			})
			Convey("given valid headers", func() {
				req.Header.Set("INSTANCE_PK", strconv.Itoa(instancePk))
				req.Header.Set("TRACE_PK", strconv.Itoa(tracePk))
				req.Header.Set("PAYLOAD_KEY", payloadKey)
				req.Header.Set("PAYLOAD_PARSED", "0")

				Convey("propagates redis error", func() {
					redisCli.On("Get", payloadKey).Return(redis.NewStringResult("", err))
					handler.ServeHTTP(rr, req)
					So(rr.Code, ShouldEqual, http.StatusInternalServerError)
				})
				Convey("on redis success with invalid payload, propagates unmarshal error", func() {
					redisCli.On("Get", payloadKey).Return(redis.NewStringResult("", nil))
					redisCli.On("Del", payloadKey).Return(redis.NewIntCmd(0, nil))
					handler.ServeHTTP(rr, req)
				})
				Convey("on redis success, processes and runs request", func() {
					stdout := "stdout"
					stderr := "stderr"
					took := int64(50)
					payload := `{"name": "endpoint/name", "output_limit": 10, "files": {"a": "b"}, "source_hash": "sourcehash", "entrypoint": "entry", "environment": "env", "environment_url": "env_url", "trace": "trace_raw",
					"run": {"additional_args": "\"args\"", "config": "\"cfg\"", "meta": "\"meta\"", "runtime_name": "\"runtime\"", "timeout": 30.125}}`

					redisCli.On("Get", payloadKey).Return(redis.NewStringResult(payload, nil))
					redisCli.On("Del", payloadKey).Return(redis.NewIntCmd(0, nil))

					Convey("propagates prepareRequest json error", func() {
						req.Body = ioutil.NopCloser(bytes.NewBufferString("[123]"))
						req.Header.Add("Content-Type", "application/json")
						handler.ServeHTTP(rr, req)
						So(rr.Code, ShouldEqual, http.StatusBadRequest)
					})
					Convey("on socket files existing", func() {
						repoCli.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: true}, nil)

						Convey("propagates Run error", func() {
							lbCli.On("Run", mock.Anything).Return(nil, err)
							handler.ServeHTTP(rr, req)
							So(rr.Code, ShouldEqual, http.StatusBadGateway)
						})
						Convey("handles timeout correctly", func() {
							ch := make(chan int64, 1)
							runStream := new(lbmocks.ScriptRunner_RunClient)
							lbCli.On("Run", mock.Anything).Return(runStream, nil)
							runStream.On("Send", mock.Anything).Once().Return(nil)
							runStream.On("Send", mock.Anything).Once().Return(err).Run(func(args mock.Arguments) {
								ch <- args.Get(0).(*lbpb.RunRequest).GetRequest().GetMeta().GetOptions().Timeout
							})
							handler.ServeHTTP(rr, req)
							So(rr.Code, ShouldEqual, http.StatusBadGateway)
							So(<-ch, ShouldEqual, 30125)
						})

						Convey("on successful Run", func() {
							runStream := new(lbmocks.ScriptRunner_RunClient)
							lbCli.On("Run", mock.Anything).Return(runStream, nil)
							runStream.On("Send", mock.Anything).Return(nil)
							runStream.On("CloseSend", mock.Anything).Return(nil, nil)

							Convey("Recv error is treated as blocked script", func() {
								runStream.On("Recv", mock.Anything).Return(nil, err)
								amqpCh.On("Publish", mock.Anything, queue, mock.Anything, mock.Anything, mock.Anything).Return(nil)
								handler.ServeHTTP(rr, req)
								So(rr.Code, ShouldEqual, http.StatusTooManyRequests)
								var trace ScriptTrace
								e := json.Unmarshal(rr.Body.Bytes(), &trace)
								So(e, ShouldBeNil)
								So(trace.ID, ShouldEqual, tracePk)
								So(trace.Status, ShouldEqual, "blocked")
							})
							Convey("Chunk Recv error stops reading", func() {
								runStream.On("Recv", mock.Anything).Return(&scriptpb.RunResponse{
									Code:   0,
									Took:   took,
									Stdout: []byte(stdout),
									Stderr: []byte(stderr),
								}, nil).Once()
								runStream.On("Recv", mock.Anything).Return(nil, err)
								amqpCh.On("Publish", mock.Anything, queue, mock.Anything, mock.Anything, mock.Anything).Return(nil)

								handler.ServeHTTP(rr, req)
								So(rr.Code, ShouldEqual, http.StatusOK)
								var trace ScriptTrace
								e := json.Unmarshal(rr.Body.Bytes(), &trace)
								So(e, ShouldBeNil)
								So(trace.ID, ShouldEqual, tracePk)
								So(trace.Status, ShouldEqual, successStatus)
							})
							Convey("processes trace responses correctly", func() {
								runStream.On("Recv", mock.Anything).Return(&scriptpb.RunResponse{
									Code:   0,
									Took:   took,
									Stdout: []byte(stdout),
									Stderr: []byte(stderr),
								}, nil).Once()
								runStream.On("Recv", mock.Anything).Return(nil, io.EOF)
								amqpCh.On("Publish", mock.Anything, queue, mock.Anything, mock.Anything, mock.Anything).Return(nil)

								handler.ServeHTTP(rr, req)
								So(rr.Code, ShouldEqual, http.StatusOK)

								var trace ScriptTrace
								So(json.Unmarshal(rr.Body.Bytes(), &trace), ShouldBeNil)
								So(trace.ID, ShouldEqual, tracePk)
								So(trace.Status, ShouldEqual, successStatus)
								So(trace.Duration, ShouldEqual, took)
								So(trace.Result, ShouldNotBeNil)
								So(string(trace.Result.Stdout), ShouldEqual, strconv.QuoteToASCII(stdout))
								So(string(trace.Result.Stderr), ShouldEqual, strconv.QuoteToASCII(stderr))
								So(trace.Result.Response, ShouldBeNil)
							})
							Convey("processes trace failure", func() {
								runStream.On("Recv", mock.Anything).Return(&scriptpb.RunResponse{
									Code: 1,
									Took: took,
								}, nil).Once()
								runStream.On("Recv", mock.Anything).Return(nil, io.EOF)
								amqpCh.On("Publish", mock.Anything, queue, mock.Anything, mock.Anything, mock.Anything).Return(nil)

								handler.ServeHTTP(rr, req)
								So(rr.Code, ShouldEqual, http.StatusInternalServerError)

								var trace ScriptTrace
								So(json.Unmarshal(rr.Body.Bytes(), &trace), ShouldBeNil)
								So(trace.ID, ShouldEqual, tracePk)
								So(trace.Status, ShouldEqual, "failure")
							})
							Convey("processes trace timeout", func() {
								runStream.On("Recv", mock.Anything).Return(&scriptpb.RunResponse{
									Code: 124,
									Took: took,
								}, nil).Once()
								runStream.On("Recv", mock.Anything).Return(nil, io.EOF)
								amqpCh.On("Publish", mock.Anything, queue, mock.Anything, mock.Anything, mock.Anything).Return(nil)

								handler.ServeHTTP(rr, req)
								So(rr.Code, ShouldEqual, http.StatusRequestTimeout)

								var trace ScriptTrace
								So(json.Unmarshal(rr.Body.Bytes(), &trace), ShouldBeNil)
								So(trace.Status, ShouldEqual, "timeout")
							})
							Convey("processes http response correctly", func() {
								code := http.StatusConflict
								content := []byte("content")
								contentType := "content/type"
								headers := map[string]string{"header1": "val1", "header2": "val2"}

								runStream.On("Recv", mock.Anything).Return(&scriptpb.RunResponse{
									Code:   0,
									Took:   took,
									Stdout: []byte(stdout),
									Stderr: []byte(stderr),
									Response: &scriptpb.HTTPResponseMessage{
										StatusCode:  int32(code),
										Content:     content,
										ContentType: contentType,
										Headers:     headers,
									},
								}, nil).Once()
								runStream.On("Recv", mock.Anything).Return(nil, io.EOF)
								amqpCh.On("Publish", mock.Anything, queue, mock.Anything, mock.Anything, mock.Anything).Return(nil)

								handler.ServeHTTP(rr, req)
								So(rr.Code, ShouldEqual, code)
								So(rr.Body.Bytes(), ShouldResemble, content)
								So(rr.Header().Get("Content-Type"), ShouldEqual, contentType)
								for k, v := range headers {
									So(rr.Header().Get(k), ShouldEqual, v)
								}
							})
							Convey("processes chunked http response", func() {
								code := http.StatusConflict
								content := []byte("content")
								content2 := []byte("content2")

								runStream.On("Recv", mock.Anything).Return(&scriptpb.RunResponse{
									Took: took,
									Response: &scriptpb.HTTPResponseMessage{
										StatusCode: int32(code),
										Content:    content,
									},
								}, nil).Once()
								runStream.On("Recv", mock.Anything).Return(&scriptpb.RunResponse{
									Response: &scriptpb.HTTPResponseMessage{
										Content: content2,
									},
								}, nil).Once()
								runStream.On("Recv", mock.Anything).Return(nil, io.EOF)
								amqpCh.On("Publish", mock.Anything, queue, mock.Anything, mock.Anything, mock.Anything).Return(nil)

								handler.ServeHTTP(rr, req)
								So(rr.Code, ShouldEqual, code)
								So(rr.Body.Bytes(), ShouldResemble, append(content, content2...))
							})
						})
					})
				})
				Convey("given payload with cache, sets it correctly in redis", func() {
					stdout := "stdout"
					stderr := "stderr"
					t := int64(50)
					payload := `{"cache": 2.5, "name": "endpoint/name", "output_limit": 10, "files": {"a": "b"}, "source_hash": "sourcehash", "entrypoint": "entry", "environment": "env", "environment_url": "env_url", "trace": "trace_raw",
					"run": {"additional_args": "\"args\"", "config": "\"cfg\"", "meta": "\"meta\"", "runtime_name": "\"runtime\"", "timeout": 30}}`
					cacheKey := createCacheKey("1", "endpoint/name", "sourcehash")

					redisCli.On("Get", payloadKey).Return(redis.NewStringResult(payload, nil))
					redisCli.On("Del", payloadKey).Return(redis.NewIntCmd(0, nil))
					repoCli.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: true}, nil)

					redisCli.On("Get", cacheKey).Return(redis.NewStringResult("", err)).Once()
					cacheCh := make(chan []byte, 1)

					redisCli.On("Set", cacheKey, mock.Anything, 2500*time.Millisecond).Return(nil).Run(func(args mock.Arguments) {
						cacheCh <- args.Get(1).([]byte)
					})

					runStream := new(lbmocks.ScriptRunner_RunClient)
					lbCli.On("Run", mock.Anything).Return(runStream, nil)
					runStream.On("Send", mock.Anything).Return(nil)
					runStream.On("CloseSend", mock.Anything).Return(nil, nil)

					runStream.On("Recv", mock.Anything).Return(&scriptpb.RunResponse{
						Code:   0,
						Time:   t,
						Stdout: []byte(stdout),
						Stderr: []byte(stderr),
					}, nil).Once()
					runStream.On("Recv", mock.Anything).Return(nil, io.EOF)
					amqpCh.On("Publish", mock.Anything, queue, mock.Anything, mock.Anything, mock.Anything).Return(nil)

					handler.ServeHTTP(rr, req)
					So(rr.Code, ShouldEqual, http.StatusOK)

					var trace1 ScriptTrace
					var trace2 ScriptTrace
					cacheVal := <-cacheCh
					e := msgpack.Unmarshal(cacheVal, &trace1)
					So(e, ShouldBeNil)
					e = json.Unmarshal(rr.Body.Bytes(), &trace2)
					So(e, ShouldBeNil)

					So(trace1.ID, ShouldEqual, trace2.ID)
					So(string(trace1.Result.Stdout), ShouldEqual, string(trace2.Result.Stdout))
					So(string(trace1.Result.Stderr), ShouldEqual, string(trace2.Result.Stderr))
				})
				Convey("given cached response, returns it without running scripts", func() {
					stdout := strconv.QuoteToASCII("stdout")
					stderr := strconv.QuoteToASCII("stderr")
					var trace1 ScriptTrace
					trace1.ID = 13
					trace1.Result = &ScriptTraceResult{
						Stdout: json.RawMessage(stdout),
						Stderr: json.RawMessage(stderr),
					}
					traceBytes, e := msgpack.Marshal(&trace1)
					So(e, ShouldBeNil)

					payload := `{"cache": 2.5, "name": "endpoint/name", "output_limit": 10, "files": {"a": "b"}, "source_hash": "sourcehash", "entrypoint": "entry", "environment": "env", "environment_url": "env_url", "trace": "trace_raw",
					"run": {"additional_args": "\"args\"", "config": "\"cfg\"", "meta": "\"meta\"", "runtime_name": "\"runtime\"", "timeout": 30}}`
					cacheKey := createCacheKey("1", "endpoint/name", "sourcehash")

					redisCli.On("Get", payloadKey).Return(redis.NewStringResult(payload, nil))
					redisCli.On("Del", payloadKey).Return(redis.NewIntCmd(0, nil))
					redisCli.On("Get", cacheKey).Return(redis.NewStringResult(string(traceBytes), nil)).Once()

					handler.ServeHTTP(rr, req)
					So(rr.Code, ShouldEqual, http.StatusOK)

					var trace2 ScriptTrace
					e = json.Unmarshal(rr.Body.Bytes(), &trace2)
					So(e, ShouldBeNil)

					So(trace1.ID, ShouldEqual, trace2.ID)
					So(string(trace1.Result.Stdout), ShouldEqual, string(trace2.Result.Stdout))
					So(string(trace1.Result.Stderr), ShouldEqual, string(trace2.Result.Stderr))
				})
			})
		})
		Convey("multipart http request with files is correctly parsed", func() {
			expectedArgs := `{"field":"value"}`
			expectedFile := make([]byte, chunkSize*1.5)
			rand.Read(expectedFile)

			body := new(bytes.Buffer)
			writer := multipart.NewWriter(body)
			writer.WriteField("field", "value")
			w, _ := writer.CreateFormFile("file", "filename")
			w.Write(expectedFile)
			writer.Close()

			req, _ := http.NewRequest("POST", "/", body)
			req.Header.Add("Content-Type", writer.FormDataContentType())

			request := new(brokerpb.RunRequest)
			e := s.processRequestData(req, request)
			So(e, ShouldBeNil)
			So(len(request.Request), ShouldEqual, 2)

			So(request.Request[0].GetChunk().Name, ShouldEqual, script.ChunkARGS)
			So(request.Request[0].GetChunk().Data, ShouldResemble, []byte(expectedArgs))
			So(request.Request[1].GetChunk().Name, ShouldEqual, "file")
			So(request.Request[1].GetChunk().Data, ShouldResemble, expectedFile)
		})
		Convey("empty multipart http request is correctly parsed", func() {
			expectedArgs := `{}`

			body := new(bytes.Buffer)
			writer := multipart.NewWriter(body)
			writer.Close()

			req, _ := http.NewRequest("POST", "/", body)
			req.Header.Add("Content-Type", writer.FormDataContentType())

			request := new(brokerpb.RunRequest)
			e := s.processRequestData(req, request)
			So(e, ShouldBeNil)
			So(len(request.Request), ShouldEqual, 1)

			So(request.Request[0].GetChunk().Name, ShouldEqual, script.ChunkARGS)
			So(request.Request[0].GetChunk().Data, ShouldResemble, []byte(expectedArgs))
		})
		Convey("json http request is correctly parsed", func() {
			expectedArgs := `{"abc":[123,"a"]}`

			req, _ := http.NewRequest("POST", "/", bytes.NewBufferString(expectedArgs))
			req.Header.Add("Content-Type", "application/json;charset=utf-8")

			request := new(brokerpb.RunRequest)
			e := s.processRequestData(req, request)
			So(e, ShouldBeNil)
			So(len(request.Request), ShouldEqual, 1)

			So(request.Request[0].GetChunk().Name, ShouldEqual, script.ChunkARGS)
			So(request.Request[0].GetChunk().Data, ShouldResemble, []byte(expectedArgs))
		})
		s.Shutdown()
		mock.AssertExpectationsForObjects(t, amqpCh, redisCli, repoCli, lbCli)
	})
}
