package docker

import (
	"bytes"
	"context"
	"errors"
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
	cpusLimit             int64
	iopsLimit             uint64
	options               Options
	info                  types.Info
}

// Options holds information about manager controlled docker limits to enforce.
type Options struct {
	BlkioDevice   string
	Network       string
	NetworkSubnet string
	ReservedCPU   float64
	ExtraHosts    []string
}

// DefaultOptions holds default options values for docker manager.
var DefaultOptions = Options{
	BlkioDevice:   "/dev/sda",
	Network:       "isolated_nw",
	NetworkSubnet: "172.25.0.0/16",
	ReservedCPU:   0.25,
}

// ErrReservedCPUTooHigh signals that too we are trying to reserve more cpu than we got.
var ErrReservedCPUTooHigh = errors.New("value of reserved cpu is higher than available")

// Constraints defines limitations for docker container.
type Constraints struct {
	MemoryLimit     int64
	MemorySwapLimit int64
	PidLimit        int64
	NofileUlimit    int64
	StorageLimit    string
}

const (
	containerWorkdir = "/tmp"
	defaultTimeout   = 30 * time.Second
	devFuseMount     = "/dev/fuse"

	// DockerVersion specifies client API version to use.
	DockerVersion = "1.26"
)

// NewManager initializes a new manager for docker.
func NewManager(options Options, cli Client) (*StdManager, error) {
	mergo.Merge(&options, DefaultOptions) // nolint - error not possible

	// Get and save Info from docker.
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	info, err := cli.Info(ctx)
	if err != nil {
		return nil, err
	}

	m := StdManager{
		client:  cli,
		options: options,
		info:    info,
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

	// Check reserved cpu option.
	if float64(info.NCPU) < options.ReservedCPU {
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

// SetLimits sets per container CPU and IOPS limit based on available IOPS and desired concurrency.
func (m *StdManager) SetLimits(concurrency uint, nodeIOPS uint64) {
	// Calculate CPUs limit.
	m.cpusLimit = int64((float64(m.info.NCPU)-m.options.ReservedCPU)*1e9) / int64(concurrency)
	m.iopsLimit = nodeIOPS / uint64(concurrency)
}

// Options returns a copy of manager options struct.
func (m *StdManager) Options() Options {
	return m.options
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

// CreateContainer creates docker container for given image and command.
func (m *StdManager) CreateContainer(ctx context.Context, image, user string, cmd []string, env []string,
	labels map[string]string, constraints Constraints, binds []string) (string, error) {

	stopTimeout := 0
	containerConfig := container.Config{
		Image:        image,
		User:         user,
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
		OpenStdin:    true,
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
			Rate: m.iopsLimit,
		},
	}
	init := true
	hostConfig := container.HostConfig{
		Init:        &init,
		AutoRemove:  true,
		Binds:       binds,
		DNS:         []string{"208.67.222.222", "208.67.220.220"},
		CapAdd:      []string{"SYS_ADMIN"},
		SecurityOpt: []string{"apparmor=unconfined"},
		ExtraHosts:  m.options.ExtraHosts,
		NetworkMode: container.NetworkMode(m.options.Network),
		Resources: container.Resources{
			Devices: []container.DeviceMapping{{
				PathOnHost:        devFuseMount,
				PathInContainer:   devFuseMount,
				CgroupPermissions: "rw"}},
			Memory:               constraints.MemoryLimit,
			MemorySwap:           constraints.MemorySwapLimit,
			Ulimits:              ulimits,
			PidsLimit:            constraints.PidLimit,
			NanoCPUs:             m.cpusLimit,
			BlkioDeviceReadIOps:  blkioThrottle,
			BlkioDeviceWriteIOps: blkioThrottle,
		},
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

// AttachContainer attaches to container's stdout and stderr.
func (m *StdManager) AttachContainer(ctx context.Context, containerID string) (types.HijackedResponse, error) {
	return m.client.ContainerAttach(ctx, containerID, types.ContainerAttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
		Stdin:  true,
	})
}

// StartContainer starts given containerID.
func (m *StdManager) StartContainer(ctx context.Context, containerID string) error {
	return m.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
}

// StopContainer stops given containerID.
func (m *StdManager) StopContainer(ctx context.Context, containerID string) error {
	return m.client.ContainerStop(ctx, containerID, nil)
}

// ProcessResponse processes response stream from docker container and returns stdout+stderr.
func (m *StdManager) ProcessResponse(ctx context.Context, resp types.HijackedResponse, magicString string, limit uint32) ([]byte, []byte, error) {
	stdoutCh := make(chan []byte)
	stderrCh := make(chan []byte)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	errCh := make(chan error)

	go func() {
		errCh <- readStreamUntil(resp.Reader, stdoutCh, stderrCh, magicString, limit)
	}()
	for {
		select {
		case b := <-stdoutCh:
			stdout.Write(b)
		case b := <-stderrCh:
			stderr.Write(b)
		case <-ctx.Done():
			return stdout.Bytes(), stderr.Bytes(), ctx.Err()
		case err := <-errCh:
			if err == nil {
				stdout.Truncate(stdout.Len() - len(magicString))
				stderr.Truncate(stderr.Len() - len(magicString))
			}
			return stdout.Bytes(), stderr.Bytes(), err
		}
	}
}
