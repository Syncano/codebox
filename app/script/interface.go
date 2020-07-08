package script

import (
	"context"
	"net"

	"github.com/go-redis/redis/v7"
	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"

	pb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"
)

// Runner provides methods to use to run user scripts securely.
//go:generate go run github.com/vektra/mockery/cmd/mockery -name Runner
type Runner interface {
	Options() Options
	DownloadAllImages() error
	CleanupUnused()
	Run(ctx context.Context, logger logrus.FieldLogger, requestID string, def *Definition, options *RunOptions) (*Result, error)
	CreatePool() (string, error)
	StopPool()
	Shutdown()
	OnContainerRemoved(f ContainerRemovedHandler)
	IsRunning() bool
	DeleteContainers(i *Index) []*Container
	DeleteContainersByID(i *Index, containerID string) []*Container
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
	redis.Cmdable
}

// Assert that redis client is compatible with our interface.
var _ RedisClient = (*redis.Client)(nil)

//go:generate go run github.com/vektra/mockery/cmd/mockery -name ScriptRunnerClient
type ScriptRunnerClient interface { // nolint
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
