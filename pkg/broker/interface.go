package broker

import (
	"time"

	"github.com/go-redis/redis"
	opentracing "github.com/opentracing/opentracing-go"
)

// RedisClient defines redis client methods we are using.
//go:generate go run github.com/vektra/mockery/cmd/mockery -inpkg -testonly -name RedisClient
type RedisClient interface {
	Get(key string) *redis.StringCmd
	Del(keys ...string) *redis.IntCmd
	Set(key string, value interface{}, expiration time.Duration) *redis.StatusCmd
}

// Assert that redis client is compatible with our interface.
var _ RedisClient = (*redis.Client)(nil)

// Tracer defines opentracing tracer we are using.
//go:generate go run github.com/vektra/mockery/cmd/mockery -name Tracer
type Tracer interface {
	StartSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span
	Inject(sm opentracing.SpanContext, format interface{}, carrier interface{}) error
	Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error)
}

// Assert that opentracing NoopTracer is compatible with our interface.
var _ Tracer = (*opentracing.NoopTracer)(nil)
