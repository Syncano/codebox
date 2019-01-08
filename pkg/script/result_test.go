package script

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestResultParsing(t *testing.T) {
	Convey("Given no custom response, Parse sets it to null", t, func() {
		data := []byte{50}
		res := &Result{}
		e := res.Parse(data, 1024, nil)
		So(e, ShouldBeNil)
		So(res.Code, ShouldEqual, 50)
		So(res.Response, ShouldBeNil)
	})
	Convey("Given valid custom response, Parse parses struct correctly", t, func() {
		data := []byte("\x32" + "\x30\x00\x00\x00" + `{"sc":200,"ct":"text/some-html","h":{"abc":"1"}}abc`)
		res := &Result{}
		e := res.Parse(data, 1024, nil)
		So(e, ShouldBeNil)
		So(res.Code, ShouldEqual, 50)
		So(res.Response.Content, ShouldResemble, []byte("abc"))
		So(res.Response.ContentType, ShouldEqual, "text/some-html")
		So(res.Response.StatusCode, ShouldEqual, 200)
		So(res.Response.Headers, ShouldResemble, map[string]string{"abc": "1"})
	})
	Convey("Given valid custom response, Parse succeeds", t, func() {
		m := make(map[string]string)
		for i := 0; i < 50; i++ {
			m[strconv.Itoa(i)] = "v"
		}
		jsonMap, _ := json.Marshal(m)

		m2 := make(map[string]string)
		for i := 0; i < 31; i++ {
			m2[strconv.Itoa(i)] = strings.Repeat("a", 255)
		}
		jsonHugeMap, _ := json.Marshal(m2)

		for _, resp := range []string{
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":{"%s":"val"}}`, strings.Repeat("a", 127)),
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":{"key":"%s"}}`, strings.Repeat("a", 4096)),
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":%s}`, jsonMap),
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":%s}`, jsonHugeMap),
		} {
			buf := make([]byte, 4)
			binary.LittleEndian.PutUint32(buf, uint32(len(resp)))
			data := append(append([]byte("2"), buf...), resp...)
			res := &Result{}
			e := res.Parse(data, 1024, nil)
			So(e, ShouldBeNil)
		}
	})
	Convey("Given invalid custom response, Parse propagates error", t, func() {
		m := make(map[string]string)
		for i := 0; i < 51; i++ {
			m[strconv.Itoa(i)] = "v"
		}
		jsonMap, _ := json.Marshal(m)

		m2 := make(map[string]string)
		for i := 0; i < 32; i++ {
			m2[strconv.Itoa(i)] = strings.Repeat("a", 255)
		}
		jsonHugeMap, _ := json.Marshal(m2)

		for _, resp := range []string{
			// `{"sc":200,"c":"YWJj","ct":"text/some-html","h":{"abc":1}}`,
			`{"sc":600,"ct":"text/html"}`,
			`{"sc":200,"ct":"texthtml"}`,
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":{"%s":"val"}}`, strings.Repeat("a", 128)),
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":{"key":"%s"}}`, strings.Repeat("a", 4097)),
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":%s}`, jsonMap),
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":%s}`, jsonHugeMap),
		} {
			buf := make([]byte, 4)
			binary.LittleEndian.PutUint32(buf, uint32(len(resp)))
			data := append(append([]byte{1}, buf...), resp...)
			res := &Result{}
			e := res.Parse(data, 1024, nil)
			So(res.Code, ShouldEqual, 1)
			So(e, ShouldBeNil)
			So(res.Stderr, ShouldResemble, ResponseValidationErrorText)
		}
		for _, data := range [][]byte{
			[]byte(`2` + "\x30\x00\x00"),
			[]byte(`2` + "\x30\x00\x00\x00" + `{"sc":200}abc`),
			[]byte(`2` + "\x0f\x00\x00\x00" + `{"h":{"key":1}}`),
		} {
			res := &Result{}
			e := res.Parse(data, 1024, nil)
			So(res.Code, ShouldEqual, 1)
			So(e, ShouldEqual, ErrIncorrectCustomResponse)
		}
	})
	Convey("Given deadline exceeded process error, Parse sets code to 124", t, func() {
		data := []byte{1}
		res := &Result{}
		e := res.Parse(data, 1024, context.DeadlineExceeded)
		So(e, ShouldBeNil)
		So(res.Code, ShouldEqual, 124)
		So(res.Response, ShouldBeNil)
	})
	Convey("Given limit reached error, Parse sets code to 1", t, func() {
		data := []byte{1}
		res := &Result{}
		e := res.Parse(data, 1024, ErrLimitReached)
		So(e, ShouldBeNil)
		So(res.Code, ShouldEqual, 1)
		So(res.Stderr, ShouldResemble, LimitReachedText)
		So(res.Response, ShouldBeNil)
	})
	Convey("Given too large streams, Parse trims them and sets code to 1", t, func() {
		data := []byte{1}
		stdout := []byte(strings.Repeat("a", 2048))
		res := &Result{Stdout: stdout}
		e := res.Parse(data, 1024, nil)
		So(e, ShouldBeNil)
		So(res.Code, ShouldEqual, 1)
		So(res.Stdout, ShouldResemble, stdout[:1024])
		So(res.Stderr, ShouldResemble, LimitReachedText)
		So(res.Response, ShouldBeNil)
	})
}

func TestMuxParsing(t *testing.T) {
	Convey("Given net server,", t, func() {
		l, _ := net.Listen("tcp", "127.0.0.1:3000")
		defer l.Close()

		retCh := make(chan map[byte]*bytes.Buffer, 1)
		errCh := make(chan error, 1)
		readMuxConn := func(ctx context.Context, limit uint32) {
			conn, _ := net.Dial("tcp", "127.0.0.1:3000")
			ret, err := readMux(ctx, conn, limit)
			retCh <- ret
			errCh <- err
		}

		Convey("readMux respects context", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			go readMuxConn(ctx, 0)
			So(<-errCh, ShouldResemble, context.DeadlineExceeded)
			cancel()
		})
		Convey("readMux respects limit and trims the output", func() {
			go readMuxConn(context.Background(), 1)
			conn, _ := l.Accept()
			conn.Write([]byte{MuxStdout, 2, 0, 0, 0, 0, 0})
			So(<-errCh, ShouldResemble, ErrLimitReached)
			ret := <-retCh
			So(len(ret[MuxStdout].Bytes()), ShouldEqual, 1)
		})
		Convey("readMux propagates unexpected eof error", func() {
			go readMuxConn(context.Background(), 0)
			conn, _ := l.Accept()
			conn.Write([]byte{MuxStdout, 2, 0, 0, 0, 0})
			conn.Close()
			So(<-errCh, ShouldResemble, io.ErrUnexpectedEOF)
		})
	})
}
