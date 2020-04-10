package broker

import (
	"time"

	"github.com/go-redis/redis/v7"
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
