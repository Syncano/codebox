package script

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	validator "gopkg.in/go-playground/validator.v9"

	"github.com/Syncano/codebox/pkg/util"
)

var (
	// ErrIncorrectCustomResponse signals that passed response was malformed
	ErrIncorrectCustomResponse = errors.New("incorrect custom response received")
	// LimitReachedText is used when stdout/stderr is too large or custom response could not be parsed.
	LimitReachedText = []byte("stdout and/or stderr max size exceeded.")
	// ResponseValidationErrorText is used when HTTP response validation failed.
	ResponseValidationErrorText = []byte("HTTP response validation failed.")
	// ErrLimitReached signals limit being reached while reading a stream.
	ErrLimitReached = errors.New("limit reached")
	// ErrMalformedHeader signals that mux from container has been malformed.
	ErrMalformedHeader = errors.New("malformed header")
)

// Mux enum.
const (
	MuxStdout = iota + 1
	MuxStderr
	MuxResponse
)

type muxBuf struct {
	mux byte
	buf []byte
}

const defaultBufferLength = 32 * 1024

func internalReadMux(r io.Reader, ch chan<- muxBuf, limit uint32, waitForMux byte) error {
	if limit == 0 {
		limit = math.MaxUint32
	}

	header := make([]byte, 5)
	var (
		len, bufLen uint32
		mux         byte
		err         error
	)

	for {
		if limit == 0 {
			return ErrLimitReached
		}

		_, err = io.ReadFull(r, header)
		if err != nil {
			return err
		}

		mux = header[0]
		len = binary.LittleEndian.Uint32(header[1:])

		if len > limit {
			len = limit
		}
		limit -= len

		// Read chunks.
		for {
			if len == 0 {
				break
			}

			bufLen = defaultBufferLength
			if bufLen > len {
				bufLen = len
			}
			len -= bufLen
			buf := make([]byte, bufLen)

			_, err = io.ReadFull(r, buf)
			if err != nil {
				return err
			}
			ch <- muxBuf{mux: mux, buf: buf}
		}

		if waitForMux == mux {
			if limit > 0 {
				return nil
			}
			return ErrLimitReached
		}
	}
}

// readMux processes stream mux.
func readMux(ctx context.Context, conn io.ReadCloser, limit uint32) (map[byte]*bytes.Buffer, error) {
	ret := map[byte]*bytes.Buffer{
		MuxStdout:   new(bytes.Buffer),
		MuxStderr:   new(bytes.Buffer),
		MuxResponse: new(bytes.Buffer),
	}
	ch := make(chan muxBuf)
	errCh := make(chan error)

	go func() {
		errCh <- internalReadMux(conn, ch, limit, MuxResponse)
	}()

	for {
		select {
		case buf := <-ch:
			if _, ok := ret[buf.mux]; !ok {
				conn.Close()
				return ret, ErrMalformedHeader
			}
			ret[buf.mux].Write(buf.buf)
		case err := <-errCh:
			return ret, err
		case <-ctx.Done():
			return ret, ctx.Err()
		}
	}
}

// Result is a response type for Runner.Run() method.
type Result struct {
	Code           int
	Stdout, Stderr []byte
	Response       *HTTPResponse
	Took           time.Duration
	Overhead       time.Duration
	Weight         uint
	Cached         bool
}

func (ret *Result) String() string {
	return fmt.Sprintf("{Code:%d, StdoutLen:%d, StderrLen:%d, Response:%v, Took:%v, Overhead:%v}",
		ret.Code, len(ret.Stdout), len(ret.Stderr), ret.Response, ret.Took, ret.Overhead)
}

// Parse parses stdout into custom response.
func (ret *Result) Parse(data []byte, streamMaxLength int, processErr error) error {
	// Process exit code.
	switch {
	case processErr == nil && len(data) > 0:
		ret.Code = int(data[0])
	case processErr == context.DeadlineExceeded:
		ret.Code = 124

	default:
		if processErr == ErrLimitReached {
			ret.Stderr = LimitReachedText
		}
		ret.Code = 1
	}

	// If streams are too long, trim them.
	if len(ret.Stdout)+len(ret.Stderr) > streamMaxLength {
		if len(ret.Stdout) > streamMaxLength {
			ret.Stdout = ret.Stdout[:streamMaxLength]
		}
		ret.Stderr = LimitReachedText
		ret.Code = 1
	}

	// If len(data) > 1 then we got custom response appended.
	if len(data) > 1 {
		data = data[1:]
		res := new(HTTPResponse)
		// Check for first length bytes (uint32).
		if len(data) < 4 {
			ret.Code = 1
			return ErrIncorrectCustomResponse
		}

		// Read json length.
		jsonLen := binary.LittleEndian.Uint32(data[:4])
		if len(data) < int(jsonLen+4) {
			ret.Code = 1
			return ErrIncorrectCustomResponse
		}

		// Process json and content.
		if err := json.Unmarshal(data[4:jsonLen+4], res); err != nil {
			ret.Stderr = LimitReachedText
			ret.Code = 1
			return ErrIncorrectCustomResponse
		}
		res.Content = data[jsonLen+4:]

		if err := validate.Struct(res); err != nil {
			ret.Stderr = []byte("HTTP response validation failed.")
			ret.Code = 1
			// Return nil as we already handled this error.
			return nil
		}
		ret.Response = res
	}

	return nil
}

var validate *validator.Validate

func init() {
	validate = validator.New()
	util.Must(validate.RegisterValidation("headermap", func(fl validator.FieldLevel) bool {
		m := fl.Field().Interface().(map[string]string)
		var totalLen int
		for k, v := range m {
			totalLen += 3 + len(k) + len(v)
			if totalLen > 8*1024 {
				return false
			}
			if validate.Var(k, "printascii,max=127") != nil || validate.Var(v, "printascii,max=4096") != nil {
				return false
			}
		}

		return true
	}))
}

// HTTPResponse describes custom HTTP response JSON from script wrapper.
type HTTPResponse struct {
	StatusCode  int               `json:"sc" validate:"min=100,max=599"`
	ContentType string            `json:"ct" validate:"max=255,printascii,containsrune=/"`
	Headers     map[string]string `json:"h" validate:"max=50,headermap"`
	Content     []byte            `json:"-"`
}

func (res *HTTPResponse) String() string {
	return fmt.Sprintf("{Code:%d, ContentType:%.25s, ContentLen:%d, HeadersLen:%d}", res.StatusCode, res.ContentType, len(res.Content), len(res.Headers))
}
