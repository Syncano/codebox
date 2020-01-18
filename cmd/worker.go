package cmd

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/go-redis/redis"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Syncano/codebox/pkg/docker"
	"github.com/Syncano/codebox/pkg/filerepo"
	repopb "github.com/Syncano/codebox/pkg/filerepo/proto"
	lbpb "github.com/Syncano/codebox/pkg/lb/proto"
	"github.com/Syncano/codebox/pkg/script"
	scriptpb "github.com/Syncano/codebox/pkg/script/proto"
	"github.com/Syncano/codebox/pkg/sys"
	"github.com/Syncano/codebox/pkg/version"
)

const (
	heartbeatTimeout = 1 * time.Second
	heartbeatRetry   = 3
	defaultTimeout   = 3 * time.Second
)

var (
	dockerOptions = &docker.Options{}
	scriptOptions = &script.Options{Constraints: new(docker.Constraints)}
)

var workerCmd = cli.Command{
	Name:        "worker",
	Usage:       "Starts worker for Codebox that is being controlled by load balancer.",
	Description: "Worker runs user code in docker environment.",
	Flags: []cli.Flag{
		cli.DurationFlag{
			Name: "heartbeat, b", Usage: "heartbeat sent to load balancer",
			EnvVar: "HEARTBEAT", Value: 5 * time.Second,
		},

		// Redis options.
		cli.StringFlag{
			Name: "redis-addr", Usage: "redis TCP address",
			EnvVar: "REDIS_ADDR", Value: "redis:6379",
		},

		// Script Runner general options.
		cli.StringFlag{
			Name: "host-storage-path, hpath", Usage: "runner host storage path",
			EnvVar: "HOST_STORAGE_PATH", Value: script.DefaultOptions.HostStoragePath, Destination: &scriptOptions.HostStoragePath,
		},
		cli.BoolFlag{
			Name: "prune-images", Usage: "prune unused images on start if set",
			EnvVar: "PRUNE_IMAGES", Destination: &scriptOptions.PruneImages,
		},
		cli.BoolFlag{
			Name: "use-existing-images", Usage: "check if images exist and only download if they are missing",
			EnvVar: "USE_EXISTING_IMAGES", Destination: &scriptOptions.UseExistingImages,
		},

		// Script Runner limits.
		cli.UintFlag{
			Name: "concurrency, c", Usage: "script concurrency",
			EnvVar: "CONCURRENCY", Value: script.DefaultOptions.Concurrency, Destination: &scriptOptions.Concurrency,
		},
		cli.UintFlag{
			Name: "cpu", Usage: "runner total cpu limit (in millicpus)",
			EnvVar: "MCPU", Value: script.DefaultOptions.MCPU, Destination: &scriptOptions.MCPU,
		},
		cli.Uint64Flag{
			Name: "iops", Usage: "runner total iops limit",
			EnvVar: "IOPS", Value: script.DefaultOptions.NodeIOPS, Destination: &scriptOptions.NodeIOPS,
		},
		cli.Int64Flag{
			Name: "memory-limit, m", Usage: "script memory limit",
			EnvVar: "MEMORY", Value: script.DefaultOptions.Constraints.MemoryLimit, Destination: &scriptOptions.Constraints.MemoryLimit,
		},
		cli.Uint64Flag{
			Name: "memory-margin", Usage: "runner memory margin",
			EnvVar: "MEMORY_MARGIN", Value: script.DefaultOptions.MemoryMargin, Destination: &scriptOptions.MemoryMargin,
		},

		// Script Runner creation options.
		cli.DurationFlag{
			Name: "create-timeout", Usage: "container create timeout",
			EnvVar: "CREATE_TIMEOUT", Value: script.DefaultOptions.CreateTimeout, Destination: &scriptOptions.CreateTimeout,
		},
		cli.IntFlag{
			Name: "create-retry-count", Usage: "container create retry count",
			EnvVar: "CREATE_RETRY_COUNT", Value: script.DefaultOptions.CreateRetryCount, Destination: &scriptOptions.CreateRetryCount,
		},
		cli.DurationFlag{
			Name: "create-retry-sleep", Usage: "container create retry sleep",
			EnvVar: "CREATE_RETRY_SLEEP", Value: script.DefaultOptions.CreateRetrySleep, Destination: &scriptOptions.CreateRetrySleep,
		},

		// Script Runner container cache options.
		cli.DurationFlag{
			Name: "container-ttl, l", Usage: "container ttl",
			EnvVar: "CONTAINER_TTL", Value: script.DefaultOptions.ContainerTTL, Destination: &scriptOptions.ContainerTTL,
		},
		cli.IntFlag{
			Name: "containers-capacity, cap", Usage: "containers capacity",
			EnvVar: "CONTAINERS_CAPACITY", Value: script.DefaultOptions.ContainersCapacity, Destination: &scriptOptions.ContainersCapacity,
		},
		cli.IntFlag{
			Name: "stream-max-length", Usage: "stream (stdout/stderr) max length",
			EnvVar: "STREAM_MAX_LENGTH", Value: script.DefaultOptions.StreamMaxLength, Destination: &scriptOptions.StreamMaxLength,
		},

		// Docker options.
		cli.StringFlag{
			Name: "docker-network", Usage: "docker network to use",
			EnvVar: "DOCKER_NETWORK", Value: docker.DefaultOptions.Network, Destination: &dockerOptions.Network,
		},
		cli.StringFlag{
			Name: "docker-network-subnet", Usage: "docker network subnet to create (if network is missing)",
			EnvVar: "DOCKER_NETWORK_SUBNET", Value: docker.DefaultOptions.NetworkSubnet, Destination: &dockerOptions.NetworkSubnet,
		},
		cli.Float64Flag{
			Name: "docker-reserved-cpu", Usage: "reserved cpu for docker runner",
			EnvVar: "DOCKER_RESERVED_CPU", Value: docker.DefaultOptions.ReservedCPU, Destination: &dockerOptions.ReservedCPU,
		},
		cli.StringFlag{
			Name: "docker-blkio-device", Usage: "docker blkio device to limit iops",
			EnvVar: "DOCKER_DEVICE", Value: docker.DefaultOptions.BlkioDevice, Destination: &dockerOptions.BlkioDevice,
		},
		cli.StringSliceFlag{
			Name: "docker-extra-hosts", Usage: "docker extra hosts",
			EnvVar: "DOCKER_EXTRA_HOSTS",
		},

		// File Repo options.
		cli.StringFlag{
			Name: "repo-path, rpath", Usage: "path for file repo storage",
			EnvVar: "REPO_PATH", Value: filerepo.DefaultOptions.BasePath, Destination: &repoOptions.BasePath,
		},
		cli.Float64Flag{
			Name: "repo-max-disk-usage, rdu", Usage: "max allowed file repo max disk usage",
			EnvVar: "REPO_MAX_DISK_USAGE", Value: filerepo.DefaultOptions.MaxDiskUsage, Destination: &repoOptions.MaxDiskUsage,
		},
		cli.DurationFlag{
			Name: "repo-ttl, rttl", Usage: "ttl for file repo storage",
			EnvVar: "REPO_TTL", Value: filerepo.DefaultOptions.TTL, Destination: &repoOptions.TTL,
		},
		cli.IntFlag{
			Name: "repo-capacity, rcap", Usage: "max capacity for file repo storage",
			EnvVar: "REPO_CAPACITY", Value: filerepo.DefaultOptions.Capacity, Destination: &repoOptions.Capacity,
		},
		cli.StringFlag{
			Name: "lb-addr, lb", Usage: "load balancer TCP address",
			EnvVar: "LB_ADDR", Value: "127.0.0.1:9000",
		},
	},
	Action: func(c *cli.Context) error {
		logrus.WithFields(logrus.Fields{
			"version":   App.Version,
			"gitsha":    version.GitSHA,
			"buildtime": App.Compiled,
		}).Info("Worker starting")

		// Initialize redis client.
		redisClient := redis.NewClient(&redis.Options{
			Addr:     c.String("redis-addr"),
			Password: "",
			DB:       0,
		})

		// Initialize docker client.
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion(docker.DockerVersion))
		if err != nil {
			return err
		}

		// Initialize docker manager.
		logrus.WithField("options", dockerOptions).Debug("Initializing docker manager")
		dockerOptions.ExtraHosts = c.StringSlice("docker-extra-hosts")
		dockerMgr, err := docker.NewManager(dockerOptions, cli)
		if err != nil {
			return err
		}

		// Initialize system checker.
		syschecker := new(sys.SigarChecker)

		// Initialize file repo.
		logrus.WithField("options", repoOptions).Debug("Initializing file repo")
		repo := filerepo.New(repoOptions, syschecker, new(filerepo.LinkFs), new(filerepo.Command))

		// Initialize script runner.
		logrus.WithField("options", scriptOptions).Debug("Initializing script runner")
		runner, err := script.NewRunner(scriptOptions, dockerMgr, syschecker, repo, redisClient)
		if err != nil {
			return err
		}
		if err = runner.DownloadAllImages(); err != nil {
			return err
		}

		// Setup health check.
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			if runner.IsRunning() {
				w.WriteHeader(200)
			} else {
				w.WriteHeader(400)
			}
		})

		// Start RPC-LB connection.
		stopServer := make(chan struct{})
		serverDone := make(chan struct{})

		go func() {
			// Connect to LB and retry on error unless server is stopped.
			defer close(serverDone)
			var poolID string
			setupPool := true

			for {
				if setupPool {
					// Setup pool on start and whenever successfully registered to reset to clean state after error.
					runner.StopPool()
					// Create script runner pool.
					if poolID, err = runner.CreatePool(); err != nil {
						panic(err)
					}
					runner.CleanupUnused()
				}

				setupPool, err = startServer(poolID, c.String("lb-addr"), c.Duration("heartbeat"),
					repo, runner, syschecker, stopServer)
				if s, e := status.FromError(err); e && s.Code() == codes.DeadlineExceeded || err == context.DeadlineExceeded {
					logrus.WithError(err).Warn("Server error")
				} else if err != nil {
					logrus.WithError(err).Error("Server error")
				}

				select {
				case <-stopServer:
					return
				case <-time.After(1 * time.Second):
				}
			}
		}()

		// Handle SIGINT and SIGTERM.
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch

		// Graceful shutdown.
		logrus.Info("Shutting down")

		close(stopServer)
		<-serverDone

		runner.Shutdown()
		repo.Shutdown()
		redisClient.Close()
		return nil
	},
}

func init() {
	App.Commands = append(App.Commands, workerCmd)
}

// startServer returns two values. First one is a bool - true if registration to specified lbAddr was successful,
// second one is an error that occurred (if any).
func startServer(
	poolID, lbAddr string,
	heartbeat time.Duration,
	repo filerepo.Repo,
	runner script.Runner,
	syschecker sys.SystemChecker,
	stopCh chan struct{}) (bool, error) {
	logger := logrus.WithField("pool", poolID)

	// Create TCP socket on random port.
	lis, err := net.Listen("tcp", ":0") // nolint: gosec
	if err != nil {
		return false, err
	}

	// Serve a new gRPC server.
	grpcServer := grpc.NewServer(sys.DefaultGRPCServerOptions...)
	repopb.RegisterRepoServer(grpcServer, &filerepo.Server{Repo: repo})
	scriptpb.RegisterScriptRunnerServer(grpcServer, script.NewServer(runner))

	go func() {
		if err = grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			logger.WithError(err).Fatal("GRPC serve error")
		}
	}()

	defer grpcServer.GracefulStop()
	logger.WithField("port", lis.Addr().(*net.TCPAddr).Port).Info("Serving gRPC")

	errCh := make(chan error, 1)

	// Connect to load balancer.
	conn, err := grpc.Dial(lbAddr, sys.DefaultGRPCDialOptions...)
	if err != nil {
		return false, err
	}

	defer conn.Close()

	plugClient := lbpb.NewWorkerPlugClient(conn)

	// Setup script runner event handlers.
	runner.OnContainerRemoved(func(cont *script.Container) {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		if _, e := plugClient.ContainerRemoved(ctx,
			&lbpb.ContainerRemovedRequest{
				Id:          poolID,
				ContainerID: cont.ID,
				SourceHash:  cont.SourceHash,
				Environment: cont.Environment,
				UserID:      cont.UserID,
			}); err != nil {
			errCh <- e
		}
		cancel()
	})
	runner.OnContainerReleased(func(cont *script.Container, options *script.RunOptions) {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		if _, e := plugClient.ResourceRelease(ctx, &lbpb.ResourceReleaseRequest{Id: poolID, MCPU: options.MCPU}); err != nil {
			errCh <- e
		}
		cancel()
	})

	// Register with load balancer.
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	if _, err = plugClient.Register(ctx,
		&lbpb.RegisterRequest{
			Id:          poolID,
			Port:        uint32(lis.Addr().(*net.TCPAddr).Port),
			MCPU:        uint32(scriptOptions.MCPU - uint(dockerOptions.ReservedCPU*1000)),
			Memory:      syschecker.AvailableMemory(),
			DefaultMCPU: uint32(scriptOptions.Constraints.CPULimit / 1e6),
		}); err != nil {
		return false, err
	}

	logger.Info("Registered with load balancer")

	// On quit call disconnect.
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		plugClient.Disconnect(ctx, &lbpb.DisconnectRequest{Id: poolID}) // nolint - ignore error
		cancel()
	}()

	// Select loop.
	ticker := time.NewTicker(heartbeat)

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), heartbeatTimeout)
			_, err := plugClient.Heartbeat(ctx,
				&lbpb.HeartbeatRequest{
					Id:     poolID,
					Memory: syschecker.AvailableMemory(),
				},
				grpc_retry.WithMax(heartbeatRetry))

			cancel()

			if err != nil {
				logger.WithError(err).Warn("Heartbeat failed")
				return true, nil
			}
		case err := <-errCh:
			logger.WithError(err).Warn("RPC call failed")
			return true, err
		case <-stopCh:
			return true, nil
		}
	}
}
