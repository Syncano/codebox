package script

import (
	"context"

	"github.com/sirupsen/logrus"
)

// Runner provides methods to use to run user scripts securely.
//go:generate mockery -name Runner
type Runner interface {
	Options() Options
	DownloadAllImages() error
	CleanupUnused()
	Run(ctx context.Context, logger logrus.FieldLogger, runtime, sourceHash, environment, userID string, options *RunOptions) (*Result, error)
	CreatePool() (string, error)
	StopPool()
	Shutdown()
	OnContainerRemoved(f ContainerRemovedHandler)
	OnRunDone(f RunDoneHandler)
	IsRunning() bool
}

// Assert that DockerRunner is compatible with our interface.
var _ Runner = (*DockerRunner)(nil)
