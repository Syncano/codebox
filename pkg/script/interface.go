package script

import (
	"context"
	"net"

	"github.com/go-redis/redis"
	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"
)

// Runner provides methods to use to run user scripts securely.
//go:generate go run github.com/vektra/mockery/cmd/mockery -name Runner
type Runner interface {
	Options() Options
	DownloadAllImages() error
	CleanupUnused()
	Run(ctx context.Context, logger logrus.FieldLogger, runtime, sourceHash, environment, userID string, options *RunOptions) (*Result, error)
	CreatePool() (string, error)
	StopPool()
	Shutdown()
	OnContainerRemoved(f ContainerRemovedHandler)
	OnContainerReleased(f ContainerReleasedHandler)
	IsRunning() bool
}

// Assert that DockerRunner is compatible with our interface.
var _ Runner = (*DockerRunner)(nil)

// YamuxSession provides methods to open new session connections.
//go:generate go run github.com/vektra/mockery/cmd/mockery -inpkg -testonly -name YamuxSession
type YamuxSession interface {
	Open() (net.Conn, error)
	Close() error
}

// Assert that yamux.Session is compatible with our interface.
var _ YamuxSession = (*yamux.Session)(nil)

// RedisClient defines redis client methods we are using.
//go:generate go run github.com/vektra/mockery/cmd/mockery -inpkg -testonly -name RedisClient
type RedisClient interface {
	Publish(channel string, message interface{}) *redis.IntCmd
}

// Assert that redis client is compatible with our interface.
var _ RedisClient = (*redis.Client)(nil)
