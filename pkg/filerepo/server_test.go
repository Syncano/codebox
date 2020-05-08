package filerepo_test

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	. "github.com/Syncano/codebox/pkg/filerepo"
	"github.com/Syncano/codebox/pkg/filerepo/mocks"
	pb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/filerepo/v1"
)

func TestServer(t *testing.T) {
	logrus.SetOutput(ioutil.Discard)

	Convey("Given server with mocked repo", t, func() {
		repo := new(mocks.Repo)
		server := Server{Repo: repo}
		err := errors.New("some error")

		Convey("Exists returns true if file exists", func() {
			repo.On("Get", "abc").Return("value").Once()
			req := pb.ExistsRequest{Key: "abc"}

			res, e := server.Exists(context.Background(), &req)
			So(e, ShouldBeNil)
			So(res.Ok, ShouldBeTrue)
		})
		Convey("Exists returns false otherwise", func() {
			repo.On("Get", "abc").Return("").Once()
			req := pb.ExistsRequest{Key: "abc"}

			res, e := server.Exists(context.Background(), &req)
			So(e, ShouldBeNil)
			So(res.Ok, ShouldBeFalse)
		})

		Convey("given mocked Upload stream, Upload", func() {
			stream := new(mocks.Repo_UploadServer)
			stream.On("Context").Return(context.Background())

			Convey("given proper meta and chunk data", func() {
				r1 := pb.UploadRequest{Value: &pb.UploadRequest_Meta{
					Meta: &pb.UploadRequest_MetaMessage{Key: "someKey"},
				}}
				r2 := pb.UploadRequest{Value: &pb.UploadRequest_Chunk{
					Chunk: &pb.UploadRequest_ChunkMessage{
						Name: "someName",
						Data: []byte("someData"),
					},
				}}
				rDone := pb.UploadRequest{Value: &pb.UploadRequest_Done{
					Done: true,
				}}
				stream.On("Recv").Return(&r1, nil).Once()

				Convey("stores file in repo", func() {
					stream.On("Recv").Return(&r2, nil).Once()
					stream.On("Recv").Return(&rDone, nil).Once()
					ch := make(chan struct{})
					repo.On("StoreLock", "someKey").Return(ch, "storeKey").Once()
					repo.On("Store", "someKey", "storeKey", mock.Anything, "someName", os.ModePerm).Return("path", nil)
					repo.On("StoreUnlock", "someKey", "storeKey", ch, true).Once()
					stream.On("Send", mock.Anything).Return(nil).Twice()
					e := server.Upload(stream)
					So(e, ShouldBeNil)
				})
				Convey("stores multi-part file in repo", func() {
					bufCh := make(chan []byte, 1)
					stream.On("Recv").Return(&r2, nil).Once()
					stream.On("Recv").Return(&r2, nil).Once()
					stream.On("Recv").Return(&rDone, nil).Once()
					ch := make(chan struct{})
					repo.On("StoreLock", "someKey").Return(ch, "storeKey").Once()
					repo.On("Store", "someKey", "storeKey", mock.Anything, "someName", os.ModePerm).Return("path", nil).Run(func(args mock.Arguments) {
						buf := make([]byte, 16)
						args.Get(2).(io.Reader).Read(buf)
						bufCh <- buf
					})
					repo.On("StoreUnlock", "someKey", "storeKey", ch, true).Once()
					stream.On("Send", mock.Anything).Return(nil).Twice()
					e := server.Upload(stream)
					So(e, ShouldBeNil)
					So(string(<-bufCh), ShouldEqual, "someDatasomeData")
				})
				Convey("propagates multi-file store error", func() {
					stream.On("Send", mock.Anything).Return(nil).Once()
					r3 := pb.UploadRequest{Value: &pb.UploadRequest_Chunk{
						Chunk: &pb.UploadRequest_ChunkMessage{
							Name: "someName2",
							Data: []byte("someData"),
						},
					}}
					stream.On("Recv").Return(&r2, nil).Once()
					stream.On("Recv").Return(&r3, nil).Once()
					ch := make(chan struct{})
					repo.On("StoreLock", "someKey").Return(ch, "storeKey").Once()
					repo.On("Store", "someKey", "storeKey", mock.Anything, "someName", os.ModePerm).Return("path", err)
					repo.On("StoreUnlock", "someKey", "storeKey", ch, false).Once()
					repo.On("Delete", "someKey").Once()
					e := server.Upload(stream)
					So(e, ShouldResemble, status.Error(codes.Internal, err.Error()))
				})
				Convey("handles mid-upload error", func() {
					stream.On("Send", mock.Anything).Return(nil).Once()
					stream.On("Recv").Return(&r2, nil).Once()
					stream.On("Recv").Return(nil, err).Once().WaitUntil(time.After(5 * time.Millisecond))
					ch := make(chan struct{})
					repo.On("StoreLock", "someKey").Return(ch, "storeKey").Once()
					repo.On("Store", "someKey", "storeKey", mock.Anything, "someName", os.ModePerm).Return("path", nil)
					repo.On("StoreUnlock", "someKey", "storeKey", ch, false).Once()
					e := server.Upload(stream)
					So(e, ShouldEqual, err)
				})
				Convey("propagates store error", func() {
					stream.On("Send", mock.Anything).Return(nil).Once()
					stream.On("Recv").Return(&r2, nil).Once()
					stream.On("Recv").Return(&rDone, nil).Once()
					err = ErrNotEnoughDiskSpace
					repo.On("Delete", "someKey").Once()
					ch := make(chan struct{})
					repo.On("StoreLock", "someKey").Return(ch, "storeKey").Once()
					repo.On("Store", "someKey", "storeKey", mock.Anything, "someName", os.ModePerm).Return("path", err)
					repo.On("StoreUnlock", "someKey", "storeKey", ch, false).Once()
					e := server.Upload(stream)
					So(e, ShouldResemble, status.Error(codes.ResourceExhausted, err.Error()))
				})
				Convey("ends upload if key exists", func() {
					repo.On("StoreLock", "someKey").Return(nil, "storeKey").Once()
					stream.On("Send", mock.Anything).Return(nil).Once()
					e := server.Upload(stream)
					So(e, ShouldBeNil)
				})
			})
			Convey("sends error response on missing Meta (chunk)", func() {
				r := pb.UploadRequest{Value: &pb.UploadRequest_Chunk{
					Chunk: &pb.UploadRequest_ChunkMessage{
						Name: "someName",
						Data: []byte("someData"),
					},
				}}
				stream.On("Recv").Return(&r, nil).Once()
				e := server.Upload(stream)
				So(e, ShouldEqual, ErrMissingMeta)
			})
			Convey("sends error response on missing Meta (done)", func() {
				r := pb.UploadRequest{Value: &pb.UploadRequest_Done{Done: true}}
				stream.On("Recv").Return(&r, nil).Once()
				e := server.Upload(stream)
				So(e, ShouldEqual, ErrMissingMeta)
			})
			Convey("returns on EOF", func() {
				stream.On("Recv").Return(nil, io.EOF).Once()
				e := server.Upload(stream)
				So(e, ShouldBeNil)
			})
			Convey("propagates Recv error", func() {
				stream.On("Recv").Return(nil, err).Once()
				e := server.Upload(stream)
				So(e, ShouldEqual, err)
			})

			stream.AssertExpectations(t)
		})

		repo.AssertExpectations(t)
	})
}
