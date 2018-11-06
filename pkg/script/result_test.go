package script

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/Syncano/codebox/pkg/docker"
)

func TestResultParsing(t *testing.T) {
	sep := []byte("SEP")

	Convey("Given stdout without custom response, Parse sets it to null", t, func() {
		stderr := []byte(`out2`)
		res := &Result{Stderr: stderr}
		e := res.Parse(sep, 1024, nil)
		So(e, ShouldBeNil)
		So(res.Code, ShouldEqual, 50) // Last stderr character is parsed as exit code. 2 is 50 in ASCII.
		So(res.Stderr, ShouldResemble, stderr[:3])
		So(res.Response, ShouldBeNil)
	})
	Convey("Given stdout with valid custom response, Parse parses struct correctly", t, func() {
		stdout := []byte(`out2SEP` + "\x30\x00\x00\x00" + `{"sc":200,"ct":"text/some-html","h":{"abc":"1"}}abc`)
		res := &Result{Stdout: stdout, Stderr: []byte("out2")}
		e := res.Parse(sep, 1024, nil)
		So(e, ShouldBeNil)
		So(res.Code, ShouldEqual, 50)
		So(res.Response.Content, ShouldResemble, []byte("abc"))
		So(res.Response.ContentType, ShouldEqual, "text/some-html")
		So(res.Response.StatusCode, ShouldEqual, 200)
		So(res.Response.Headers, ShouldResemble, map[string]string{"abc": "1"})
	})
	Convey("Given stdout with valid custom response, Parse succeeds", t, func() {
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

		for _, out := range []string{
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":{"%s":"val"}}`, strings.Repeat("a", 127)),
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":{"key":"%s"}}`, strings.Repeat("a", 4096)),
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":%s}`, jsonMap),
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":%s}`, jsonHugeMap),
		} {
			buf := make([]byte, 4)
			binary.LittleEndian.PutUint32(buf, uint32(len(out)))
			stdout := append(append([]byte("out2SEP"), buf...), out...)
			res := &Result{Stdout: stdout}
			e := res.Parse(sep, 1024, nil)
			So(e, ShouldBeNil)
		}
	})
	Convey("Given stdout with invalid custom response, Parse propagates error", t, func() {
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

		for _, out := range []string{
			// `{"sc":200,"c":"YWJj","ct":"text/some-html","h":{"abc":1}}`,
			`{"sc":600,"ct":"text/html"}`,
			`{"sc":200,"ct":"texthtml"}`,
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":{"%s":"val"}}`, strings.Repeat("a", 128)),
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":{"key":"%s"}}`, strings.Repeat("a", 4097)),
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":%s}`, jsonMap),
			fmt.Sprintf(`{"sc":200,"ct":"text/html","h":%s}`, jsonHugeMap),
		} {
			buf := make([]byte, 4)
			binary.LittleEndian.PutUint32(buf, uint32(len(out)))
			stdout := append(append([]byte("out2SEP"), buf...), out...)
			res := &Result{Stdout: stdout, Stderr: []byte{0}}
			e := res.Parse(sep, 1024, nil)
			So(res.Code, ShouldEqual, 1)
			So(e, ShouldBeNil)
			So(res.Stderr, ShouldResemble, ResponseValidationErrorText)
		}
		for _, out := range [][]byte{
			[]byte(`out2SEP` + "\x30\x00\x00"),
			[]byte(`out2SEP` + "\x30\x00\x00\x00" + `{"sc":200}abc`),
			[]byte(`out2SEP` + "\x0f\x00\x00\x00" + `{"h":{"key":1}}`),
		} {
			res := &Result{Stdout: out, Stderr: []byte{0}}
			e := res.Parse(sep, 1024, nil)
			So(res.Code, ShouldEqual, 1)
			So(e, ShouldEqual, ErrIncorrectCustomResponse)
		}
	})
	Convey("Given deadline exceeded process error, Parse sets code to 124", t, func() {
		stdout := []byte(`out2`)
		res := &Result{Stdout: stdout}
		e := res.Parse(sep, 1024, context.DeadlineExceeded)
		So(e, ShouldBeNil)
		So(res.Code, ShouldEqual, 124)
		So(res.Stdout, ShouldResemble, stdout)
		So(res.Response, ShouldBeNil)
	})
	Convey("Given limit reached error, Parse sets code to 1", t, func() {
		stdout := []byte(`out2`)
		res := &Result{Stdout: stdout}
		e := res.Parse(sep, 1024, docker.ErrLimitReached)
		So(e, ShouldBeNil)
		So(res.Code, ShouldEqual, 1)
		So(res.Stdout, ShouldResemble, stdout)
		So(res.Stderr, ShouldResemble, LimitReachedText)
		So(res.Response, ShouldBeNil)
	})
	Convey("Given too large streams, Parse trims them and sets code to 1", t, func() {
		stdout := []byte(strings.Repeat("a", 2048))
		res := &Result{Stdout: stdout}
		e := res.Parse(sep, 1024, docker.ErrLimitReached)
		So(e, ShouldBeNil)
		So(res.Code, ShouldEqual, 1)
		So(res.Stdout, ShouldResemble, stdout[:1024])
		So(res.Stderr, ShouldResemble, LimitReachedText)
		So(res.Response, ShouldBeNil)
	})

}
