package script

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	validator "gopkg.in/go-playground/validator.v9"

	"github.com/Syncano/pkg-go/v2/util"
)

var (
	// IncorrectDataText is used when output structure is malformed.
	IncorrectDataText = []byte("malformed or too large output received")
	// LimitReachedText is used when stdout/stderr is too large or custom response could not be parsed.
	LimitReachedText = []byte("stdout and/or stderr max size exceeded.")
	// ResponseValidationErrorText is used when HTTP response validation failed.
	ResponseValidationErrorText = []byte("HTTP response validation failed.")
)

const (
	StreamStdout = iota
	StreamStderr
	StreamResponse
)

// Result is a response type for Runner.Run() method.
type Result struct {
	ContainerID    string
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
	case errors.Is(processErr, context.DeadlineExceeded):
		ret.Code = 124
	default:
		if errors.Is(processErr, util.ErrLimitReached) {
			ret.Stderr = LimitReachedText
		}

		ret.Code = 1
	}

	if len(data) > 0 {
		data = data[1:]
	}

	var err error

	for len(data) > 0 && err == nil {
		data, err = ret.parseData(data)
		if err == ErrIncorrectData {
			ret.Stderr = IncorrectDataText
			ret.Code = 1
		}
	}

	// If streams are too long, trim them.
	if len(ret.Stdout)+len(ret.Stderr) > streamMaxLength {
		if len(ret.Stdout) > streamMaxLength {
			ret.Stdout = ret.Stdout[:streamMaxLength]
		}

		ret.Stderr = LimitReachedText
		ret.Code = 1
	}

	return err
}

func (ret *Result) parseData(data []byte) ([]byte, error) {
	if len(data) < 5 {
		return nil, ErrIncorrectData
	}

	// Structure is: 1 byte of stream type, 4 bytes of length, X bytes of content
	typ := data[0]
	dataLen := binary.LittleEndian.Uint32(data[1:5])

	if len(data) < int(dataLen+5) {
		return nil, ErrIncorrectData
	}

	output := data[5 : dataLen+5]
	newData := data[5+dataLen:]

	switch typ {
	case StreamStdout:
		ret.Stdout = output
	case StreamStderr:
		ret.Stderr = output
	case StreamResponse:
		res := new(HTTPResponse)

		// Process json and content.
		if err := json.Unmarshal(output, res); err != nil {
			return nil, ErrIncorrectData
		}

		res.Content = newData

		if err := validate.Struct(res); err != nil {
			ret.Stderr = ResponseValidationErrorText
			ret.Code = 1
			// Return nil as we already handled this error.
			return newData, nil
		}

		ret.Response = res
		newData = nil

	default:
		return nil, ErrIncorrectData
	}

	return newData, nil
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
