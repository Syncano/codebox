package lb

import (
	"context"
	"errors"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/mock"

	repomocks "github.com/Syncano/codebox/pkg/filerepo/mocks"
	repopb "github.com/Syncano/codebox/pkg/filerepo/proto"
	scriptmocks "github.com/Syncano/codebox/pkg/script/mocks"
	scriptpb "github.com/Syncano/codebox/pkg/script/proto"
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

	Convey("Given mocked worker", t, func() {
		repoCli := new(repomocks.RepoClient)
		scriptCli := new(scriptmocks.ScriptRunnerClient)
		worker := &Worker{
			ID:        "id",
			freeCPU:   2000,
			alive:     true,
			repoCli:   repoCli,
			scriptCli: scriptCli,
			scripts:   make(map[ScriptInfo]int),
		}

		Convey("Reserve returns false for dead worker", func() {
			worker.alive = false
			So(worker.Reserve(100, 100, false), ShouldBeFalse)
			So(worker.FreeCPU(), ShouldEqual, 2000)
		})
		Convey("Reserve returns false when requireSlots is true and there are < 0 slots", func() {
			So(worker.Reserve(2000, 100, true), ShouldBeTrue)
			So(worker.Reserve(2000, 100, true), ShouldBeFalse)
			So(worker.FreeCPU(), ShouldEqual, 0)
			So(worker.Reserve(2000, 100, false), ShouldBeTrue)
			So(worker.FreeCPU(), ShouldEqual, -2000)
		})

		Convey("given MemMapFs and mocked stream, Upload", func() {
			stream := new(repomocks.Repo_UploadClient)
			repoCli.On("Upload", mock.Anything).Return(stream, nil)
			fs := afero.NewMemMapFs()

			Convey("propagates Meta Send error", func() {
				stream.On("Send", mock.Anything).Return(err)
				repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil)
				e := worker.Upload(context.Background(), fs, "/path", "key")
				So(e, ShouldEqual, err)
			})
			Convey("propagates Recv error", func() {
				stream.On("Send", mock.Anything).Return(nil)
				stream.On("Recv", mock.Anything).Return(nil, err).Once()
				repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil)
				e := worker.Upload(context.Background(), fs, "/path", "key")
				So(e, ShouldEqual, err)
			})
			Convey("returns if not accepted", func() {
				stream.On("Send", mock.Anything).Return(nil).Once()
				stream.On("Recv", mock.Anything).Return(&repopb.UploadResponse{Accepted: false}, nil).Once()
				e := worker.Upload(context.Background(), fs, "/path", "key")
				So(e, ShouldBeNil)
			})
			Convey("given successful meta send", func() {
				stream.On("Send", mock.Anything).Return(nil).Once()
				stream.On("Recv", mock.Anything).Return(&repopb.UploadResponse{Accepted: true}, nil).Once()

				Convey("on Walk error, checks Exists", func() {
					repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: true}, nil)

					e := worker.Upload(context.Background(), fs, "/path", "key")
					So(e, ShouldBeNil)
				})
				Convey("propagates Walk error", func() {
					repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil)

					e := worker.Upload(context.Background(), fs, "/path", "key")
					So(e, ShouldNotBeNil)
				})
				Convey("given file in MemMapFs", func() {
					afero.WriteFile(fs, "/path/file.js", []byte("content"), os.ModePerm)
					repoCli.On("Exists", mock.Anything, mock.Anything).Return(&repopb.ExistsResponse{Ok: false}, nil)

					Convey("propagates Chunk Send error", func() {
						stream.On("Send", mock.Anything).Return(err).Once()
						e := worker.Upload(context.Background(), fs, "/path", "key")
						So(e, ShouldEqual, err)
					})
					Convey("propagates Done Send error", func() {
						stream.On("Send", mock.Anything).Return(nil).Times(1)
						stream.On("Send", mock.Anything).Return(err).Once()
						e := worker.Upload(context.Background(), fs, "/path", "key")
						So(e, ShouldEqual, err)
					})
					Convey("propagates last Recv error", func() {
						stream.On("Send", mock.Anything).Return(nil).Times(2)
						stream.On("Recv", mock.Anything).Return(nil, err).Once()
						e := worker.Upload(context.Background(), fs, "/path", "key")
						So(e, ShouldEqual, err)
					})
					Convey("given mocked fs, Upload propagates read error", func() {
						stream.On("Send", mock.Anything).Return(nil)
						mockfs := new(MockFs)
						afero.WriteFile(mockfs, "/path/file.js", []byte("content"), os.ModePerm)
						mockfs.On("Open", "/path").Return(fs.Open("/path"))
						mockfs.On("Open", "/path/file.js").Return(nil, err)

						e := worker.Upload(context.Background(), mockfs, "/path", "key")
						So(e, ShouldEqual, err)
						mockfs.AssertExpectations(t)
					})
				})
			})

			stream.AssertExpectations(t)
		})

		Convey("Run propagates gRPC error", func() {
			scriptCli.On("Run", mock.Anything).Return(nil, err)
			_, e := worker.Run(context.Background(), &scriptpb.RunRequest_MetaMessage{}, nil)
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

			_, e := worker.Run(context.Background(), &scriptpb.RunRequest_MetaMessage{}, []*scriptpb.RunRequest_ChunkMessage{
				{Name: "chunk1", Data: []byte("data")},
			})
			So(e, ShouldEqual, err)
		})

		mock.AssertExpectationsForObjects(t, repoCli, scriptCli)
	})
}
