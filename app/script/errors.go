package script

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// gRPC errors.

	// ErrSourceNotAvailable signals that specified source hash was not found.
	ErrSourceNotAvailable = status.Error(codes.FailedPrecondition, "source not available")

	// Result errors.

	// ErrIncorrectData signals when output structure is malformed.
	ErrIncorrectData = errors.New("malformed or too large output received")

	// Runner critical errors.

	// ErrUnsupportedRuntime signals prohibited usage of unknown runtime.
	ErrUnsupportedRuntime = errors.New("unsupported runtime")
	// ErrPoolNotRunning signals pool is not yet running.
	ErrPoolNotRunning = errors.New("pool not running")
	// ErrPoolAlreadyCreated signals pool being already created.
	ErrPoolAlreadyCreated = errors.New("pool already created")
	// ErrSemaphoreNotAcquired signals a non critical error occurring for context timeouts and similar cases.
	ErrSemaphoreNotAcquired = errors.New("semaphore was not acquired successfully")

	// User Cache errors.

	ErrCacheCommandUnrecognized = errors.New("cache command unrecognized")
	ErrCacheCommandMalformed    = errors.New("cache command malformed")
	ErrCacheKeyLengthExceeded   = errors.New("cache key length exceeded")
	ErrCacheValueLengthExceeded = errors.New("cache value length exceeded")
)
