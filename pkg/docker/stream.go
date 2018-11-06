package docker

import (
	"encoding/binary"
	"errors"
	"io"
	"math"

	"github.com/docker/docker/pkg/stdcopy"
)

const (
	stdWriterPrefixLen = 8
	stdWriterFdIndex   = 0
	stdWriterSizeIndex = 4
	expectedReadyFd    = int(stdcopy.Stdout) | int(stdcopy.Stderr)
)

// ErrLimitReached signals limit being reached while reading a stream.
var ErrLimitReached = errors.New("limit reached")

type streamData struct {
	peek []byte
	out  chan<- []byte
}

// readHeader reads and parses docker stream header.
func readHeader(src io.Reader) (fd stdcopy.StdType, frameSize uint32, err error) {
	buf := make([]byte, stdWriterPrefixLen)
	_, err = io.ReadFull(src, buf)
	if err != nil {
		return
	}

	fd = stdcopy.StdType(buf[stdWriterFdIndex])
	frameSize = binary.BigEndian.Uint32(buf[stdWriterSizeIndex : stdWriterSizeIndex+4])
	return
}

// readStreamUntil reads through the src reader until it is finished with magicString or we surpass our limit.
func readStreamUntil(src io.Reader, stdout, stderr chan<- []byte, magicString string, limit uint32) error {
	var data *streamData
	magicStringLen := len(magicString)
	dataList := []streamData{{out: stdout}, {out: stderr}}

	if limit == 0 {
		limit = math.MaxUint32
	}

	readyFd := 0

	for {
		if limit == 0 {
			return ErrLimitReached
		}

		fd, frameSize, err := readHeader(src)
		if err != nil {
			return err
		}

		if frameSize > limit {
			frameSize = limit
		}
		limit -= frameSize

		switch fd {
		case stdcopy.Stdin:
			fallthrough
		case stdcopy.Stdout:
			// Write on stdout
			data = &dataList[0]
		case stdcopy.Stderr:
			// Write on stderr
			data = &dataList[1]
		}

		buf := make([]byte, frameSize)
		_, err = io.ReadFull(src, buf)
		if err != nil {
			return err
		}

		if int(frameSize) < magicStringLen {
			data.peek = append(data.peek, buf...)
			if len(data.peek) > magicStringLen {
				data.peek = data.peek[len(data.peek)-magicStringLen:]
			}
		} else {
			data.peek = buf[int(frameSize)-magicStringLen:]
		}

		data.out <- buf

		if string(data.peek) == magicString {
			readyFd |= int(fd)
			close(data.out)
		}
		if readyFd == expectedReadyFd {
			return nil
		}
	}
}
