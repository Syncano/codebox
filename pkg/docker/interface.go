package docker

import (
	"context"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// Manager provides methods to use docker more easily.
//go:generate mockery -name Manager
type Manager interface {
	SetLimits(concurrency uint, nodeIOPS uint64)
	Options() Options
	DownloadImage(ctx context.Context, image string, check bool) error
	PruneImages(ctx context.Context) (types.ImagesPruneReport, error)

	ListContainersByLabel(ctx context.Context, label string) ([]types.Container, error)
	CreateContainer(ctx context.Context, image, user string, cmd []string, env []string, labels map[string]string, constraints Constraints, binds []string) (string, error)
	AttachContainer(ctx context.Context, containerID string) (types.HijackedResponse, error)
	ContainerErrorLog(ctx context.Context, containerID string) (io.ReadCloser, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string) error
}

// Assert that StdManager is compatible with our interface.
var _ Manager = (*StdManager)(nil)

// Client defines docker client methods we are using
//go:generate mockery -inpkg -testonly -name Client
type Client interface {
	Info(ctx context.Context) (types.Info, error)
	NetworkInspect(ctx context.Context, networkID string, options types.NetworkInspectOptions) (types.NetworkResource, error)
	NetworkCreate(ctx context.Context, name string, options types.NetworkCreate) (types.NetworkCreateResponse, error)

	ContainerAttach(ctx context.Context, container string, options types.ContainerAttachOptions) (types.HijackedResponse, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.ContainerCreateCreatedBody, error)
	ContainerStart(ctx context.Context, container string, options types.ContainerStartOptions) error
	ContainerStop(ctx context.Context, container string, timeout *time.Duration) error
	ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error
	ContainerPause(ctx context.Context, container string) error
	ContainerUnpause(ctx context.Context, container string) error
	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error)

	ImagePull(ctx context.Context, ref string, options types.ImagePullOptions) (io.ReadCloser, error)
	ImageInspectWithRaw(ctx context.Context, image string) (types.ImageInspect, []byte, error)
	ImagesPrune(ctx context.Context, pruneFilter filters.Args) (types.ImagesPruneReport, error)
}

// Assert that docker client is compatible with our interface.
var _ Client = (*client.Client)(nil)
