package docker

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/pkg/stdcopy"
	. "github.com/smartystreets/goconvey/convey"
)

func TestStreamHeader(t *testing.T) {
	Convey("Given valid header, readHeader", t, func() {
		src := bytes.NewReader([]byte{0x01, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0x0A, 0xAB})

		Convey("reads properly frame info", func() {
			fd, frameSize, err := readHeader(src)
			So(fd, ShouldEqual, stdcopy.Stdout)
			So(frameSize, ShouldEqual, 0x0A)
			So(err, ShouldBeNil)
		})

		Convey("reads only 8 bytes", func() {
			readHeader(src)
			b, _ := src.ReadByte()
			So(b, ShouldEqual, 0xAB)
		})

		Convey("propagates error", func() {
			ioutil.ReadAll(src)
			_, _, err := readHeader(src)
			So(err, ShouldEqual, io.EOF)
		})
	})
}

func TestStreamReader(t *testing.T) {
	Convey("Given valid stream, readStreamUntil", t, func() {
		src := bytes.NewReader([]byte{
			0x00, 0, 0, 0, 0, 0, 0, 0x03, 0x0A, 0x0A, 0x0A,
			0x01, 0, 0, 0, 0, 0, 0, 0x03, 0x0B, 0x0B, 0x0C,
			0x02, 0, 0, 0, 0, 0, 0, 0x04, 0x0A, 0x0B, 0x0B, 0x0C})
		stdout, stderr := make(chan []byte, 2), make(chan []byte, 1)

		Convey("respects limit", func() {
			err := readStreamUntil(src, stdout, stderr, string([]byte{0x0C}), 1)
			So(err, ShouldEqual, ErrLimitReached)
			So(<-stdout, ShouldResemble, []byte{0x0A})
		})

		Convey("handles closing channels on magicString", func() {
			var err error
			Convey("returns properly split results", func() {
				err = readStreamUntil(src, stdout, stderr, string([]byte{0x0C}), 0)
			})
			Convey("handles split magicString", func() {
				err = readStreamUntil(src, stdout, stderr, string([]byte{0x0A, 0x0B, 0x0B, 0x0C}), 0)
			})
			So(err, ShouldBeNil)
			So(<-stdout, ShouldResemble, []byte{0x0A, 0x0A, 0x0A})
			So(<-stdout, ShouldResemble, []byte{0x0B, 0x0B, 0x0C})
			So(<-stdout, ShouldBeNil)
			So(<-stderr, ShouldResemble, []byte{0x0A, 0x0B, 0x0B, 0x0C})
			So(<-stderr, ShouldBeNil)
		})
	})

	Convey("Given too short frame, readStreamUntil propagates readHeader error", t, func() {
		src := bytes.NewReader([]byte{0x00, 0, 0, 0, 0, 0, 0})

		err := readStreamUntil(src, make(chan []byte), make(chan []byte), string([]byte{0x0C}), 0)
		So(err, ShouldEqual, io.ErrUnexpectedEOF)
	})

	Convey("Given too short stream, readStreamUntil propagates ReadFull error", t, func() {
		src := bytes.NewReader([]byte{0x00, 0, 0, 0, 0, 0, 0, 0x03, 0x0A})

		err := readStreamUntil(src, make(chan []byte), make(chan []byte), string([]byte{0x0C}), 0)
		So(err, ShouldEqual, io.ErrUnexpectedEOF)
	})
}
