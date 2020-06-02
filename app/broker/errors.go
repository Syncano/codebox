package broker

import (
	"errors"
)

var (
	// ErrMissingPayloadKey means that payload was missing from request headers.
	ErrMissingPayloadKey = errors.New("missing payload")
	// ErrJSONParsingFailed is used to mark any JSON unmarshal error during payload parsing.
	ErrJSONParsingFailed = errors.New("json parsing error, expected an object")
)
