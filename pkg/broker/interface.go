package broker

import (
	"time"

	"github.com/go-redis/redis/v7"

	pb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/broker/v1"
	scriptpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"
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

//go:generate go run github.com/vektra/mockery/cmd/mockery -name ScriptRunnerClient
type ScriptRunnerClient interface {
	pb.ScriptRunnerClient
}

//go:generate go run github.com/vektra/mockery/cmd/mockery -name ScriptRunner_RunServer
type ScriptRunner_RunServer interface { // nolint
	pb.ScriptRunner_RunServer
}

//go:generate go run github.com/vektra/mockery/cmd/mockery -name ScriptRunner_RunClient
type ScriptRunner_RunClient interface { // nolint
	pb.ScriptRunner_RunClient
}

type StreamReponder interface {
	Send(*scriptpb.RunResponse) error
}
