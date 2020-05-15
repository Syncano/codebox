package lb

import (
	"context"
	"errors"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/mock"

	repopb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/filerepo/v1"
	scriptpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"

	"github.com/Syncano/codebox/pkg/common"
	repomocks "github.com/Syncano/codebox/pkg/filerepo/mocks"
	scriptmocks "github.com/Syncano/codebox/pkg/script/mocks"
)

type MockFs struct {
	mock.Mock
	afero.MemMapFs
}

func (m *MockFs) Open(name string) (afero.File, error) {
	ret := m.Called(name)
	r1 := ret.Get(0)
	if r1 != nil {
		return r1.(afero.File), ret.Error(1)
	}
	return nil, ret.Error(1)
}

func TestWorker(t *testing.T) {
	err := errors.New("some error")
	NewServer(nil, &ServerOptions{})

	Convey("Given mocked worker", t, func() {
		repoCli := new(repomocks.RepoClient)
		scriptCli := new(scriptmocks.ScriptRunnerClient)
		worker := &Worker{
			ID:         "id",
			freeCPU:    2000,
			alive:      true,
			repoCli:    repoCli,
			scriptCli:  scriptCli,
			containers: make(map[string]*WorkerContainer),
			scripts:    make(map[ScriptInfo]int),
			metrics:    Metrics(),
		}
		cont := &WorkerContainer{Worker: worker}

		Convey("Reserve returns false for dead worker", func() {
			worker.alive = false
			So(worker.reserve(100, 1, false), ShouldBeFalse)
			So(cont.Reserve(), ShouldEqual, -1)
			So(worker.FreeCPU(), ShouldEqual, 2000)
		})
		Convey("Reserve returns false when requireSlots is true and there are < 0 slots", func() {
			So(worker.reserve(2000, 1, true), ShouldBeTrue)
			So(worker.reserve(2000, 1, true), ShouldBeFalse)
			So(worker.FreeCPU(), ShouldEqual, 0)
			So(worker.reserve(2000, 1, false), ShouldBeTrue)
			So(worker.FreeCPU(), ShouldEqual, -2000)
		})

		Convey("given MemMapFs and mocked stream, Upload", func() {
			stream := new(repomocks.Repo_UploadClient)
			repoCli.On("Upload", mock.Anything).Return(stream, nil)
			fs := afero.NewMemMapFs()

			Convey("propagates Meta Send error", func() {
				stream.On("Send", mock.Anything).Return(err)
				repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil)
				e := cont.Upload(context.Background(), fs, "/path", "key")
				So(e, ShouldEqual, err)
			})
			Convey("propagates Recv error", func() {
				stream.On("Send", mock.Anything).Return(nil)
				stream.On("Recv", mock.Anything).Return(nil, err).Once()
				repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil)
				e := cont.Upload(context.Background(), fs, "/path", "key")
				So(e, ShouldEqual, err)
			})
			Convey("returns if not accepted", func() {
				stream.On("Send", mock.Anything).Return(nil).Once()
				stream.On("Recv", mock.Anything).Return(&repopb.UploadResponse{Accepted: false}, nil).Once()
				e := cont.Upload(context.Background(), fs, "/path", "key")
				So(e, ShouldBeNil)
			})
			Convey("given successful meta send", func() {
				stream.On("Send", mock.Anything).Return(nil).Once()
				stream.On("Recv", mock.Anything).Return(&repopb.UploadResponse{Accepted: true}, nil).Once()

				Convey("on Walk error, checks Exists", func() {
					repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: true}, nil)

					e := cont.Upload(context.Background(), fs, "/path", "key")
					So(e, ShouldBeNil)
				})
				Convey("propagates Walk error", func() {
					repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil)

					e := cont.Upload(context.Background(), fs, "/path", "key")
					So(e, ShouldNotBeNil)
				})
				Convey("given file in MemMapFs", func() {
					afero.WriteFile(fs, "/path/file.js", []byte("content"), os.ModePerm)
					repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil)

					Convey("propagates Chunk Send error", func() {
						stream.On("Send", mock.Anything).Return(err).Once()
						e := cont.Upload(context.Background(), fs, "/path", "key")
						So(e, ShouldEqual, err)
					})
					Convey("propagates Done Send error", func() {
						stream.On("Send", mock.Anything).Return(nil).Times(1)
						stream.On("Send", mock.Anything).Return(err).Once()
						e := cont.Upload(context.Background(), fs, "/path", "key")
						So(e, ShouldEqual, err)
					})
					Convey("propagates last Recv error", func() {
						stream.On("Send", mock.Anything).Return(nil).Times(2)
						stream.On("Recv", mock.Anything).Return(nil, err).Once()
						e := cont.Upload(context.Background(), fs, "/path", "key")
						So(e, ShouldEqual, err)
					})
					Convey("given mocked fs, Upload propagates read error", func() {
						stream.On("Send", mock.Anything).Return(nil)
						mockfs := new(MockFs)
						afero.WriteFile(mockfs, "/path/file.js", []byte("content"), os.ModePerm)
						mockfs.On("Open", "/path").Return(fs.Open("/path"))
						mockfs.On("Open", "/path/file.js").Return(nil, err)

						e := cont.Upload(context.Background(), mockfs, "/path", "key")
						So(e, ShouldEqual, err)
						mockfs.AssertExpectations(t)
					})
				})
			})

			stream.AssertExpectations(t)
		})

		Convey("Run propagates gRPC error", func() {
			scriptCli.On("Run", mock.Anything).Return(nil, err)
			_, e := cont.Run(context.Background(), &scriptpb.RunMeta{}, nil)
			So(e, ShouldEqual, err)
		})
		Convey("given mocked stream, Run", func() {
			stream := new(scriptmocks.ScriptRunner_RunClient)
			scriptCli.On("Run", mock.Anything).Return(stream, nil)

			Convey("propagates Meta Send error", func() {
				stream.On("Send", mock.Anything).Return(err)
			})
			Convey("propagates Chunk Send error", func() {
				stream.On("Send", mock.Anything).Return(nil).Once()
				stream.On("Send", mock.Anything).Return(err)
			})
			Convey("propagates Meta CloseSend error", func() {
				stream.On("Send", mock.Anything).Return(nil)
				stream.On("CloseSend").Return(err)
			})

			_, e := cont.Run(context.Background(), &scriptpb.RunMeta{}, common.NewArrayChunkReader([]*scriptpb.RunChunk{
				{Name: "chunk1", Data: []byte("data")},
			}))
			So(errors.Is(e, err), ShouldBeTrue)
		})

		mock.AssertExpectationsForObjects(t, repoCli, scriptCli)
	})
}
