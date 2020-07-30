package docker

import (
	"context"
	"io"
	"io/ioutil"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/blkiodev"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	units "github.com/docker/go-units"
	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
)

// StdManager provides methods to use docker more easily.
type StdManager struct {
	client                Client
	storageLimitSupported bool
	options               *Options
	info                  types.Info
	runtime               string
}

// Options holds information about manager controlled docker limits to enforce.
type Options struct {
	BlkioDevice   string
	Network       string
	NetworkSubnet string
	ReservedMCPU  uint
	DNS           []string
	ExtraHosts    []string
}

// DefaultOptions holds default options values for docker manager.
var DefaultOptions = Options{
	BlkioDevice:   "/dev/sda",
	Network:       "isolated_nw",
	NetworkSubnet: "172.25.0.0/16",
	ReservedMCPU:  250,
	DNS:           []string{"208.67.222.222", "208.67.220.220"},
}

// Constraints defines limitations for docker container.
type Constraints struct {
	CPULimit  int64
	CPUPeriod int64
	CPUQuota  int64
	IOPSLimit uint64

	MemoryLimit     int64
	MemorySwapLimit int64
	PidLimit        int64
	NofileUlimit    int64
	StorageLimit    string
}

func (c *Constraints) MCPU() uint32 {
	return uint32(c.CPULimit / 1e6)
}

const (
	containerWorkdir = "/tmp"
	defaultTimeout   = 30 * time.Second
	gvisorRuntime    = "runsc"

	// DockerVersion specifies client API version to use.
	DockerVersion = "1.26"
)

// NewManager initializes a new manager for docker.
func NewManager(opts *Options, cli Client) (*StdManager, error) {
	options := DefaultOptions
	_ = mergo.Merge(&options, opts, mergo.WithOverride)

	// Get and save Info from docker.
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	info, err := cli.Info(ctx)
	if err != nil {
		return nil, err
	}

	m := StdManager{
		client:  cli,
		options: &options,
		info:    info,
		runtime: gvisorRuntime,
	}

	// Check capabilities and overall compatibility of the system.
	// Check storage.
	if info.Driver == "overlay2" && info.DriverStatus[0][1] == "xfs" {
		m.storageLimitSupported = true
	} else {
		logrus.WithFields(
			logrus.Fields{
				"driver": info.Driver,
				"fs":     info.DriverStatus[0][1],
			}).Warn("Docker storage does not support limiting quota (requires overlay2 on xfs), disabling")
	}

	if _, ok := info.Runtimes[gvisorRuntime]; !ok {
		logrus.Warn("Docker does not support gVisor runtime, disabling")

		m.runtime = ""
	}

	// Check reserved cpu option.
	if uint(info.NCPU*1e3) <= options.ReservedMCPU {
		return nil, ErrReservedCPUTooHigh
	}

	// Check network isolation.
	ctx, cancel = context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	_, err = cli.NetworkInspect(ctx, m.options.Network, types.NetworkInspectOptions{})
	if err != nil {
		logrus.Warn("Docker isolation network not configured, creating network")

		if _, err = cli.NetworkCreate(ctx, m.options.Network, types.NetworkCreate{
			IPAM: &network.IPAM{
				Config: []network.IPAMConfig{
					{Subnet: m.options.NetworkSubnet},
				},
			},
		}); err != nil {
			return nil, err
		}
	}

	return &m, nil
}

// Options returns a copy of manager options struct.
func (m *StdManager) Options() Options {
	return *m.options
}

// Info returns a copy of docker info struct.
func (m *StdManager) Info() types.Info {
	return m.info
}

// DownloadImage downloads docker image if it doesn't exist already.
func (m *StdManager) DownloadImage(ctx context.Context, image string, check bool) error {
	if check {
		if _, _, err := m.client.ImageInspectWithRaw(ctx, image); err == nil {
			return nil
		}
	}

	resp, err := m.client.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	// Process (discard) read response so that image is saved.
	if _, err := io.Copy(ioutil.Discard, resp); err != nil {
		return err
	}

	if err := resp.Close(); err != nil {
		return err
	}

	return nil
}

// PruneImages removes all unused images (dangling or without a container).
func (m *StdManager) PruneImages(ctx context.Context) (types.ImagesPruneReport, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("dangling", "false")

	return m.client.ImagesPrune(ctx, filterArgs)
}

// ListContainersByLabel returns containerIDs for docker containers containing given label.
func (m *StdManager) ListContainersByLabel(ctx context.Context, label string) ([]types.Container, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", label)

	return m.client.ContainerList(ctx, types.ContainerListOptions{Filters: filterArgs})
}

// ContainerCreate creates docker container for given image and command.
func (m *StdManager) ContainerCreate(ctx context.Context, image, user string, cmd, env []string,
	labels map[string]string, constraints *Constraints, binds []string) (string, error) {
	stopTimeout := 0
	containerConfig := container.Config{
		AttachStdout: true,
		Image:        image,
		User:         user,
		Labels:       labels,
		Cmd:          cmd,
		Env:          env,
		StopTimeout:  &stopTimeout,
		WorkingDir:   containerWorkdir,
	}

	var ulimits []*units.Ulimit
	if constraints.NofileUlimit != 0 {
		ulimits = []*units.Ulimit{{
			Name: "nofile",
			Soft: constraints.NofileUlimit,
			Hard: constraints.NofileUlimit,
		}}
	}

	blkioThrottle := []*blkiodev.ThrottleDevice{
		{
			Path: m.options.BlkioDevice,
			Rate: constraints.IOPSLimit,
		},
	}

	hostDNS := m.options.DNS
	if len(hostDNS) == 0 {
		hostDNS = nil
	}

	hostConfig := container.HostConfig{
		Binds:       binds,
		DNS:         hostDNS,
		ExtraHosts:  m.options.ExtraHosts,
		NetworkMode: container.NetworkMode(m.options.Network),
		Resources: container.Resources{
			Memory:               constraints.MemoryLimit,
			MemorySwap:           constraints.MemorySwapLimit,
			Ulimits:              ulimits,
			PidsLimit:            constraints.PidLimit,
			CPUPeriod:            constraints.CPUPeriod,
			CPUQuota:             constraints.CPUQuota,
			BlkioDeviceReadIOps:  blkioThrottle,
			BlkioDeviceWriteIOps: blkioThrottle,
		},
		Runtime: m.runtime,
	}

	if m.storageLimitSupported && constraints.StorageLimit != "" {
		hostConfig.StorageOpt = map[string]string{"size": constraints.StorageLimit}
	}

	resp, err := m.client.ContainerCreate(ctx, &containerConfig, &hostConfig, nil, "")
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

// ContainerAttach attaches to container's stdout and stderr.
func (m *StdManager) ContainerAttach(ctx context.Context, containerID string) (types.HijackedResponse, error) {
	return m.client.ContainerAttach(ctx, containerID, types.ContainerAttachOptions{
		Stream: true,
		Stdout: true,
	})
}

// ContainerStart starts given containerID.
func (m *StdManager) ContainerStart(ctx context.Context, containerID string) error {
	return m.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
}

// ContainerStop stops given containerID.
func (m *StdManager) ContainerStop(ctx context.Context, containerID string) error {
	m.client.ContainerStop(ctx, containerID, nil) // nolint: errcheck
	return m.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
}

// ContainerUpdate updates given containerID.
func (m *StdManager) ContainerUpdate(ctx context.Context, containerID string, constraints *Constraints) error {
	_, err := m.client.ContainerUpdate(ctx, containerID, container.UpdateConfig{
		Resources: container.Resources{
			CPUPeriod: constraints.CPUPeriod,
			CPUQuota:  constraints.CPUQuota,
			Memory:    constraints.MemoryLimit,
		},
	})

	return err
}
