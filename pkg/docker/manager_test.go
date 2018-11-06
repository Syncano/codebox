package docker

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	units "github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
)

func TestNewManager(t *testing.T) {
	logrus.SetOutput(ioutil.Discard)

	Convey("Given mocked docker client", t, func() {
		cli := new(MockClient)

		Convey("NewManager running checks on client", func() {
			Convey("propagates Info error", func() {
				cli.On("Info", mock.Anything).Return(
					types.Info{}, io.EOF,
				)
				_, e := NewManager(Options{}, cli)
				So(e, ShouldEqual, io.EOF)
			})

			Convey("detects storage and network capabilities", func() {
				cli.On("Info", mock.Anything).Return(
					types.Info{
						Driver:       "overlay2",
						DriverStatus: [][2]string{{"-", "xfs"}, {}},
					}, nil,
				)
				cli.On("NetworkInspect", mock.Anything, "isolated_nw", mock.Anything).Return(
					types.NetworkResource{}, nil,
				)
				m, e := NewManager(Options{}, cli)
				So(e, ShouldBeNil)
				So(m.storageLimitSupported, ShouldBeTrue)
			})

			Convey("detects missing network", func() {
				cli.On("Info", mock.Anything).Return(
					types.Info{
						Driver:       "overlay2",
						DriverStatus: [][2]string{{"-", "xfs"}, {}},
					}, nil,
				)
				cli.On("NetworkInspect", mock.Anything, "isolated_nw", mock.Anything).Return(
					types.NetworkResource{}, io.EOF,
				)

				Convey("and creates a new one as a result", func() {
					cli.On("NetworkCreate", mock.Anything, "isolated_nw", mock.Anything).Return(
						types.NetworkCreateResponse{}, nil,
					)
					m, e := NewManager(Options{}, cli)
					So(e, ShouldBeNil)
					So(m.storageLimitSupported, ShouldBeTrue)
				})
				Convey("propagates error during creation", func() {
					cli.On("NetworkCreate", mock.Anything, "isolated_nw", mock.Anything).Return(
						types.NetworkCreateResponse{}, io.EOF,
					)
					_, e := NewManager(Options{}, cli)
					So(e, ShouldEqual, io.EOF)
				})
			})

			Convey("detects lack of support for storage limit", func() {
				cli.On("Info", mock.Anything).Return(
					types.Info{
						Driver:       "ext4",
						DriverStatus: [][2]string{{"-", "xfs"}, {}},
					}, nil,
				)
				cli.On("NetworkInspect", mock.Anything, "other_nw", mock.Anything).Return(
					types.NetworkResource{}, nil,
				)
				m, e := NewManager(Options{Network: "other_nw"}, cli)
				So(e, ShouldBeNil)
				So(m.storageLimitSupported, ShouldBeFalse)
			})

			Convey("detects too high reserved cpu value", func() {
				cli.On("Info", mock.Anything).Return(
					types.Info{
						Driver:       "ext4",
						DriverStatus: [][2]string{{"-", "xfs"}, {}},
					}, nil,
				)
				m, e := NewManager(Options{Network: "other_nw", ReservedCPU: 5000}, cli)
				So(m, ShouldBeNil)
				So(e, ShouldEqual, ErrReservedCPUTooHigh)
			})

			cli.AssertExpectations(t)
		})
	})
}

type mockReadCloser struct {
	readErr  error
	closeErr error
}

func (mc mockReadCloser) Read(p []byte) (int, error) { return 0, mc.readErr }
func (mc mockReadCloser) Close() error               { return mc.closeErr }

func TestManagerMethods(t *testing.T) {
	Convey("Given manager with mocked docker client", t, func() {
		cli := new(MockClient)
		m := StdManager{client: cli}

		Convey("SetLimits calculates cpus and iops limit", func() {
			m.info = types.Info{NCPU: 2}
			m.SetLimits(2, 1000)
			So(m.cpusLimit, ShouldEqual, 1*1e9)
			So(m.iopsLimit, ShouldEqual, 500)
		})
		Convey("SetLimits for calculations takes into account reserved cpu", func() {
			m.info = types.Info{NCPU: 2}
			m.options.ReservedCPU = 1
			m.SetLimits(2, 1000)
			So(m.cpusLimit, ShouldEqual, 0.5*1e9)
			So(m.iopsLimit, ShouldEqual, 500)
		})

		Convey("Options returns a copy of options struct", func() {
			So(m.Options(), ShouldNotEqual, m.options)
			So(m.Options(), ShouldResemble, m.options)
		})

		Convey("DownloadImage checks if image exists", func() {
			cli.On("ImageInspectWithRaw", context.Background(), "image").Return(types.ImageInspect{}, nil, nil)
			e := m.DownloadImage(context.Background(), "image", true)
			So(e, ShouldBeNil)
		})

		Convey("DownloadImage downloads image if it does not exists", func() {
			mrc := mockReadCloser{readErr: io.EOF}
			err := errors.New("e")

			Convey("with checking if it already exists", func() {
				cli.On("ImageInspectWithRaw", context.Background(), "image").Return(types.ImageInspect{}, nil, io.EOF)
				Convey("propagates imagepull error", func() {
					cli.On("ImagePull", context.Background(), "image", types.ImagePullOptions{}).Return(mrc, io.EOF)
					e := m.DownloadImage(context.Background(), "image", true)
					So(e, ShouldEqual, io.EOF)
				})
				Convey("propagates copy error", func() {
					mrc.readErr = err
					cli.On("ImagePull", context.Background(), "image", types.ImagePullOptions{}).Return(mrc, nil)
					e := m.DownloadImage(context.Background(), "image", true)
					So(e, ShouldEqual, err)
				})
				Convey("propagates close error", func() {
					mrc.closeErr = err
					cli.On("ImagePull", context.Background(), "image", types.ImagePullOptions{}).Return(mrc, nil)
					e := m.DownloadImage(context.Background(), "image", true)
					So(e, ShouldEqual, err)
				})
				Convey("succeeds if everything went fine", func() {
					cli.On("ImagePull", context.Background(), "image", types.ImagePullOptions{}).Return(mrc, nil)
					e := m.DownloadImage(context.Background(), "image", true)
					So(e, ShouldBeNil)
				})
			})

			Convey("without checking, always downloads", func() {
				cli.On("ImagePull", context.Background(), "image", types.ImagePullOptions{}).Return(mrc, nil)
				e := m.DownloadImage(context.Background(), "image", false)
				So(e, ShouldBeNil)
			})
		})

		Convey("CreateContainer parses given Constraints and calls CreateContainer", func() {
			Convey("on successful creation", func() {
				cli.On("ContainerCreate", context.Background(), mock.Anything, mock.Anything, mock.Anything, "").Return(
					container.ContainerCreateCreatedBody{ID: "someid"}, nil)

				create := func(constraints Constraints) (string, error, *container.HostConfig) {
					id, e := m.CreateContainer(context.Background(), "image", "user", []string{"cmd"}, []string{"env"},
						map[string]string{"label": "value"}, constraints, []string{"bind"})
					return id, e, cli.Calls[0].Arguments.Get(2).(*container.HostConfig)
				}
				Convey("returns ID", func() {
					id, e, hostConfig := create(Constraints{})
					So(e, ShouldBeNil)
					So(id, ShouldEqual, "someid")
					So(hostConfig.Ulimits, ShouldBeNil)
					So(hostConfig.StorageOpt, ShouldBeNil)
					So(hostConfig.NetworkMode, ShouldBeEmpty)
				})
				Convey("sets up nofile limit", func() {
					_, _, hostConfig := create(Constraints{NofileUlimit: 1000})
					So(hostConfig.Ulimits, ShouldResemble, []*units.Ulimit{{
						Name: "nofile",
						Soft: 1000,
						Hard: 1000,
					}})
				})
				Convey("sets up storageopt", func() {
					m.storageLimitSupported = true
					_, _, hostConfig := create(Constraints{StorageLimit: "100M"})
					So(hostConfig.StorageOpt, ShouldResemble, map[string]string{"size": "100M"})
				})
				Convey("sets up network isolation", func() {
					m.options.Network = "network"
					_, _, hostConfig := create(Constraints{})
					So(hostConfig.NetworkMode, ShouldEqual, "network")
				})
			})

			Convey("propagates error", func() {
				cli.On("ContainerCreate", context.Background(), mock.Anything, mock.Anything, mock.Anything, "").Return(
					container.ContainerCreateCreatedBody{}, io.EOF)
				_, e := m.CreateContainer(context.Background(), "image", "user", []string{"cmd"}, []string{"cmd"},
					map[string]string{"label": "value"}, Constraints{}, []string{"bind"})
				So(e, ShouldEqual, io.EOF)
			})
		})

		Convey("PruneImages calls ImagesPrune", func() {
			cli.On("ImagesPrune", context.Background(),
				mock.MatchedBy(func(f filters.Args) bool {
					return reflect.DeepEqual(f.Get("dangling"), []string{"false"})
				})).Return(types.ImagesPruneReport{}, nil)
			m.PruneImages(context.Background())
		})

		Convey("ListContainersByLabel calls ContainerList", func() {
			cli.On("ContainerList", context.Background(),
				mock.MatchedBy(func(o types.ContainerListOptions) bool {
					return reflect.DeepEqual(o.Filters.Get("label"), []string{"label"})
				})).Return([]types.Container{}, nil)
			m.ListContainersByLabel(context.Background(), "label")
		})

		Convey("AttachContainer calls ContainerAttach", func() {
			cli.On("ContainerAttach", context.Background(), "id", mock.Anything).Return(types.HijackedResponse{}, nil)
			m.AttachContainer(context.Background(), "id")
		})

		Convey("StartContainer calls ContainerStart", func() {
			cli.On("ContainerStart", context.Background(), "id", mock.Anything).Return(nil)
			m.StartContainer(context.Background(), "id")
		})

		Convey("StopContainer calls ContainerStop", func() {
			cli.On("ContainerStop", context.Background(), "id", mock.Anything).Return(nil)
			m.StopContainer(context.Background(), "id")
		})

		Convey("ProcessReader with valid stream", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reader := bufio.NewReader(bytes.NewReader([]byte{
				0x01, 0, 0, 0, 0, 0, 0, 0x02, 0x0A, 0x0B,
				0x02, 0, 0, 0, 0, 0, 0, 0x01, 0x0B,
			}))
			resp := types.HijackedResponse{Reader: reader}

			Convey("respects context", func() {
				cancel()
				_, _, e := m.ProcessResponse(ctx, resp, string(0x0B), 0)
				So(e, ShouldEqual, context.Canceled)
			})
			Convey("reads stream properly", func() {
				stdout, stderr, e := m.ProcessResponse(ctx, resp, string(0x0B), 0)
				So(stdout, ShouldResemble, []byte{0x0A})
				So(stderr, ShouldBeEmpty)
				So(e, ShouldBeNil)
			})
		})

		cli.AssertExpectations(t)
	})
}
