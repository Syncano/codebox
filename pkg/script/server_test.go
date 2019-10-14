package script_test

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/Syncano/codebox/pkg/filerepo"

	"github.com/sirupsen/logrus"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	. "github.com/Syncano/codebox/pkg/script"
	"github.com/Syncano/codebox/pkg/script/mocks"
	pb "github.com/Syncano/codebox/pkg/script/proto"
)

func TestServer(t *testing.T) {
	logrus.SetOutput(ioutil.Discard)

	Convey("Given server with mocked runner", t, func() {
		runner := new(mocks.Runner)
		server := NewServer(runner)
		err := errors.New("some error")

		Convey("given mocked Run stream, Run", func() {
			stream := new(mocks.ScriptRunner_RunServer)
			stream.On("Context").Return(context.Background())

			Convey("given proper meta and chunk data", func() {
				r1 := pb.RunRequest{Value: &pb.RunRequest_Meta{
					Meta: &pb.RunRequest_MetaMessage{
						RequestID: "reqID",
						Runtime:   "runtime", SourceHash: "hash", UserID: "userID",
						Environment: "env"},
				}}
				r2 := pb.RunRequest{Value: &pb.RunRequest_Chunk{
					Chunk: &pb.RunRequest_ChunkMessage{
						Name: "someName",
						Data: []byte("someData"),
					},
				}}
				stream.On("Recv").Return(&r1, nil).Once()
				stream.On("Recv").Return(&r2, nil).Once()
				stream.On("Recv").Return(nil, io.EOF).Once()

				Convey("runs script and returns response", func() {
					runner.On("Run", mock.Anything, mock.Anything, "runtime", "reqID", "hash", "env", "userID", mock.Anything).Return(
						&Result{Code: 1, Took: 2 * time.Millisecond, Response: &HTTPResponse{StatusCode: 204}}, nil)
					stream.On("Send", mock.Anything).Return(nil).Once()
					e := server.Run(stream)
					So(e, ShouldBeNil)
					msg := stream.Calls[len(stream.Calls)-1].Arguments.Get(0).(*pb.RunResponse)
					So(msg.Code, ShouldEqual, 1)
					So(msg.Took, ShouldEqual, 2)
					So(msg.Response.StatusCode, ShouldEqual, 204)
				})
				Convey("runs script and returns response in chunks if needed", func() {
					chunkSize := 2 * 1024 * 1024
					runner.On("Run", mock.Anything, mock.Anything, "runtime", "reqID", "hash", "env", "userID", mock.Anything).Return(
						&Result{Code: 1, Took: 2 * time.Millisecond, Response: &HTTPResponse{StatusCode: 204, Content: []byte(strings.Repeat("a", 3*chunkSize/2))}}, nil)
					stream.On("Send", mock.Anything).Return(nil).Twice()
					e := server.Run(stream)
					So(e, ShouldBeNil)
					msg := stream.Calls[len(stream.Calls)-2].Arguments.Get(0).(*pb.RunResponse)
					So(msg.Code, ShouldEqual, 1)
					So(msg.Took, ShouldEqual, 2)
					So(msg.Response.StatusCode, ShouldEqual, 204)
					So(len(msg.Response.Content), ShouldEqual, chunkSize)

					msg2 := stream.Calls[len(stream.Calls)-1].Arguments.Get(0).(*pb.RunResponse)
					So(len(msg2.Response.Content), ShouldEqual, chunkSize/2)
				})
				Convey("ignores Run error when response is returned", func() {
					runner.On("Run", mock.Anything, mock.Anything, "runtime", "reqID", "hash", "env", "userID", mock.Anything).Return(&Result{}, err)
					stream.On("Send", mock.Anything).Return(nil).Once()
					e := server.Run(stream)
					So(e, ShouldBeNil)
				})
				Convey("propagates Run error when no response is returned", func() {
					runner.On("Run", mock.Anything, mock.Anything, "runtime", "reqID", "hash", "env", "userID", mock.Anything).Return(nil, err)
					e := server.Run(stream)
					So(e, ShouldResemble, status.Error(codes.Internal, err.Error()))
				})
				Convey("propagates Send error", func() {
					runner.On("Run", mock.Anything, mock.Anything, "runtime", "reqID", "hash", "env", "userID", mock.Anything).Return(&Result{}, nil)
					stream.On("Send", mock.Anything).Return(err).Once()
					e := server.Run(stream)
					So(e, ShouldEqual, err)
				})
			})
			Convey("given proper meta and args in chunk runs script", func() {
				r1 := pb.RunRequest{Value: &pb.RunRequest_Meta{
					Meta: &pb.RunRequest_MetaMessage{
						RequestID: "reqID",
						Runtime:   "runtime", SourceHash: "hash", UserID: "userID",
						Environment: "env"},
				}}
				r2 := pb.RunRequest{Value: &pb.RunRequest_Chunk{
					Chunk: &pb.RunRequest_ChunkMessage{
						Name: ChunkARGS,
						Data: []byte("someData"),
					},
				}}
				stream.On("Recv").Return(&r1, nil).Once()
				stream.On("Recv").Return(&r2, nil).Once()
				stream.On("Recv").Return(nil, io.EOF).Once()

				runner.On("Run", mock.Anything, mock.Anything, "runtime", "reqID", "hash", "env", "userID", mock.Anything).Return(
					&Result{Code: 1, Took: 2 * time.Millisecond, Response: &HTTPResponse{StatusCode: 204}}, nil)
				stream.On("Send", mock.Anything).Return(nil).Once()
				e := server.Run(stream)
				So(e, ShouldBeNil)
				msg := stream.Calls[len(stream.Calls)-1].Arguments.Get(0).(*pb.RunResponse)
				So(msg.Code, ShouldEqual, 1)
				So(msg.Took, ShouldEqual, 2)
				So(msg.Response.StatusCode, ShouldEqual, 204)
			})
			Convey("sends no response on missing Meta", func() {
				stream.On("Recv").Return(nil, io.EOF).Once()
				e := server.Run(stream)
				So(e, ShouldBeNil)
			})
			Convey("propagates Recv error", func() {
				stream.On("Recv").Return(nil, err).Once()
				e := server.Run(stream)
				So(e, ShouldEqual, err)
			})

		})
		Convey("ParseError maps error codes", func() {
			var testData = []struct {
				e    error
				code codes.Code
			}{
				{ErrPoolNotRunning, codes.ResourceExhausted},
				{filerepo.ErrResourceNotFound, codes.FailedPrecondition},
			}
			for _, t := range testData {
				parsed, ok := status.FromError(server.ParseError(t.e))
				So(ok, ShouldBeTrue)
				So(parsed.Code(), ShouldEqual, t.code)
			}
		})

		runner.AssertExpectations(t)
	})
}
