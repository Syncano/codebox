package util

import (
	"context"
	"net"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func localConn(lis net.Listener) (net.Conn, net.Conn) {
	srvConnCh := make(chan net.Conn)
	go func() {
		srvConn, _ := lis.Accept()
		srvConnCh <- srvConn
	}()
	clientConn, _ := net.Dial(lis.Addr().Network(), lis.Addr().String())
	return clientConn, <-srvConnCh
}

func TestReadLimited(t *testing.T) {
	lis, _ := net.Listen("tcp", ":0") // nolint: gosec

	Convey("Given client-server connection", t, func() {
		clientConn, srvConn := localConn(lis)

		Convey("ReadLimited respects limit", func() {
			srvConn.Write([]byte{0, 1, 2, 3, 4, 5})
			data, err := ReadLimited(context.Background(), clientConn, 3)
			So(data, ShouldHaveLength, 3)
			So(data, ShouldResemble, []byte{0, 1, 2})
			So(err, ShouldEqual, ErrLimitReached)
		})
		Convey("ReadLimited respects context", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			data, err := ReadLimited(ctx, clientConn, 3)
			So(data, ShouldHaveLength, 0)
			So(err, ShouldResemble, context.DeadlineExceeded)
		})
		Convey("ReadLimited respects already cancelled context", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			data, err := ReadLimited(ctx, clientConn, 3)
			So(data, ShouldHaveLength, 0)
			So(err, ShouldResemble, context.Canceled)
		})
		Convey("ReadLimited handles setreadtimeout separately as well", func() {
			clientConn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
			data, err := ReadLimited(context.Background(), clientConn, 3)
			So(data, ShouldHaveLength, 0)
			So(err, ShouldResemble, context.DeadlineExceeded)
		})

		Convey("ReadLimitedUntil strips delimeter", func() {
			srvConn.Write([]byte{0, 1, 2, 3, 4, 5})
			srvConn.Write([]byte("delim"))
			data, err := ReadLimitedUntil(context.Background(), clientConn, "delim", 12)
			So(data, ShouldHaveLength, 6)
			So(data, ShouldResemble, []byte{0, 1, 2, 3, 4, 5})
			So(err, ShouldBeNil)
		})
		Convey("ReadLimitedUntil respects limit", func() {
			srvConn.Write([]byte{0, 1, 2, 3, 4, 5})
			data, err := ReadLimitedUntil(context.Background(), clientConn, "delim", 3)
			So(data, ShouldHaveLength, 3)
			So(data, ShouldResemble, []byte{0, 1, 2})
			So(err, ShouldEqual, ErrLimitReached)
		})
		Convey("ReadLimitedUntil respects context", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			data, err := ReadLimitedUntil(ctx, clientConn, "delim", 3)
			So(data, ShouldHaveLength, 0)
			So(err, ShouldResemble, context.DeadlineExceeded)
		})
		Convey("ReadLimitedUntil respects already cancelledcontext", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			data, err := ReadLimitedUntil(ctx, clientConn, "delim", 3)
			So(data, ShouldHaveLength, 0)
			So(err, ShouldResemble, context.Canceled)
		})
	})
}
