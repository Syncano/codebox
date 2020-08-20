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
	"github.com/go-redis/redis/v7"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"go.opencensus.io/plugin/ocgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/Syncano/codebox/app/common"
	"github.com/Syncano/codebox/app/docker"
	"github.com/Syncano/codebox/app/filerepo"
	"github.com/Syncano/codebox/app/script"
	"github.com/Syncano/codebox/app/version"
	"github.com/Syncano/pkg-go/v2/sys"
	"github.com/Syncano/pkg-go/v2/util"
	repopb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/filerepo/v1"
	lbpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/lb/v1"
	scriptpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"
)

const (
	heartbeatTimeout = 1 * time.Second
	heartbeatRetry   = 3
	defaultTimeout   = 3 * time.Second
)

var (
	dockerOptions = &docker.Options{}
	scriptOptions = &script.Options{Constraints: new(docker.Constraints),
		UserCacheConstraints: new(script.UserCacheConstraints)}
)

var workerCmd = &cli.Command{
	Name:        "worker",
	Usage:       "Starts worker for Codebox that is being controlled by load balancer.",
	Description: "Worker runs user code in docker environment.",
	Flags: []cli.Flag{
		&cli.DurationFlag{
			Name: "heartbeat", Aliases: []string{"b"}, Usage: "heartbeat sent to load balancer",
			EnvVars: []string{"HEARTBEAT"}, Value: 5 * time.Second,
		},

		// Redis options.
		&cli.StringFlag{
			Name: "redis-addr", Usage: "redis TCP address",
			EnvVars: []string{"REDIS_ADDR"}, Value: "redis:6379",
		},

		// Script Runner general options.
		&cli.StringFlag{
			Name: "host-storage-path", Aliases: []string{"hpath"}, Usage: "runner host storage path",
			EnvVars: []string{"HOST_STORAGE_PATH"}, Value: script.DefaultOptions.HostStoragePath, Destination: &scriptOptions.HostStoragePath,
		},
		&cli.BoolFlag{
			Name: "prune-images", Usage: "prune unused images on start if set",
			EnvVars: []string{"PRUNE_IMAGES"}, Destination: &scriptOptions.PruneImages,
		},
		&cli.BoolFlag{
			Name: "use-existing-images", Usage: "check if images exist and only download if they are missing",
			EnvVars: []string{"USE_EXISTING_IMAGES"}, Destination: &scriptOptions.UseExistingImages,
		},

		// Script Runner limits.
		&cli.UintFlag{
			Name: "concurrency", Aliases: []string{"c"}, Usage: "script concurrency",
			EnvVars: []string{"CONCURRENCY"}, Value: script.DefaultOptions.Concurrency, Destination: &scriptOptions.Concurrency,
		},
		&cli.UintFlag{
			Name: "cpu", Usage: "runner total cpu limit (in millicpus)",
			EnvVars: []string{"MCPU"}, Value: script.DefaultOptions.MCPU, Destination: &scriptOptions.MCPU,
		},
		&cli.Uint64Flag{
			Name: "iops", Usage: "runner total iops limit",
			EnvVars: []string{"IOPS"}, Value: script.DefaultOptions.NodeIOPS, Destination: &scriptOptions.NodeIOPS,
		},
		&cli.Int64Flag{
			Name: "memory-limit", Aliases: []string{"m"}, Usage: "script memory limit",
			EnvVars: []string{"MEMORY"}, Value: script.DefaultOptions.Constraints.MemoryLimit, Destination: &scriptOptions.Constraints.MemoryLimit,
		},
		&cli.Uint64Flag{
			Name: "memory-margin", Usage: "runner memory margin",
			EnvVars: []string{"MEMORY_MARGIN"}, Value: script.DefaultOptions.MemoryMargin, Destination: &scriptOptions.MemoryMargin,
		},
		&cli.Int64Flag{
			Name: "ratelimit-capacity", Usage: "capacity of per container stdout/stderr ratelimit",
			EnvVars: []string{"RATELIMIT_CAPACITY"}, Value: script.DefaultOptions.StreamCapacityLimit, Destination: &scriptOptions.StreamCapacityLimit,
		},
		&cli.Int64Flag{
			Name: "ratelimit-quantum", Usage: "quantum (per minute) of per container stdout/stderr ratelimit",
			EnvVars: []string{"RATELIMIT_QUANTUM"}, Value: script.DefaultOptions.StreamPerMinuteQuantum, Destination: &scriptOptions.StreamPerMinuteQuantum,
		},

		// Script Runner creation options.
		&cli.DurationFlag{
			Name: "create-timeout", Usage: "container create timeout",
			EnvVars: []string{"CREATE_TIMEOUT"}, Value: script.DefaultOptions.CreateTimeout, Destination: &scriptOptions.CreateTimeout,
		},
		&cli.IntFlag{
			Name: "create-retry-count", Usage: "container create retry count",
			EnvVars: []string{"CREATE_RETRY_COUNT"}, Value: script.DefaultOptions.CreateRetryCount, Destination: &scriptOptions.CreateRetryCount,
		},
		&cli.DurationFlag{
			Name: "create-retry-sleep", Usage: "container create retry sleep",
			EnvVars: []string{"CREATE_RETRY_SLEEP"}, Value: script.DefaultOptions.CreateRetrySleep, Destination: &scriptOptions.CreateRetrySleep,
		},

		// Script Runner container cache options.
		&cli.DurationFlag{
			Name: "container-ttl", Aliases: []string{"l"}, Usage: "container ttl",
			EnvVars: []string{"CONTAINER_TTL"}, Value: script.DefaultOptions.ContainerTTL, Destination: &scriptOptions.ContainerTTL,
		},
		&cli.IntFlag{
			Name: "containers-capacity", Aliases: []string{"cap"}, Usage: "containers capacity",
			EnvVars: []string{"CONTAINERS_CAPACITY"}, Value: script.DefaultOptions.ContainersCapacity, Destination: &scriptOptions.ContainersCapacity,
		},
		&cli.IntFlag{
			Name: "stream-max-length", Usage: "stream (stdout/stderr) max length",
			EnvVars: []string{"STREAM_MAX_LENGTH"}, Value: script.DefaultOptions.StreamMaxLength, Destination: &scriptOptions.StreamMaxLength,
		},

		// Docker options.
		&cli.StringFlag{
			Name: "docker-network", Usage: "docker network to use",
			EnvVars: []string{"DOCKER_NETWORK"}, Value: docker.DefaultOptions.Network, Destination: &dockerOptions.Network,
		},
		&cli.StringFlag{
			Name: "docker-network-subnet", Usage: "docker network subnet to create (if network is missing)",
			EnvVars: []string{"DOCKER_NETWORK_SUBNET"}, Value: docker.DefaultOptions.NetworkSubnet, Destination: &dockerOptions.NetworkSubnet,
		},
		&cli.UintFlag{
			Name: "docker-reserved-cpu", Usage: "reserved cpu for docker runner (in millicpu)",
			EnvVars: []string{"DOCKER_RESERVED_CPU"}, Value: docker.DefaultOptions.ReservedMCPU, Destination: &dockerOptions.ReservedMCPU,
		},
		&cli.StringFlag{
			Name: "docker-blkio-device", Usage: "docker blkio device to limit iops",
			EnvVars: []string{"DOCKER_DEVICE"}, Value: docker.DefaultOptions.BlkioDevice, Destination: &dockerOptions.BlkioDevice,
		},
		&cli.StringSliceFlag{
			Name: "docker-dns", Usage: "docker DNS",
			EnvVars: []string{"DOCKER_DNS"}, Value: cli.NewStringSlice(docker.DefaultOptions.DNS...),
		},
		&cli.StringSliceFlag{
			Name: "docker-extra-hosts", Usage: "docker extra hosts",
			EnvVars: []string{"DOCKER_EXTRA_HOSTS"},
		},

		// File Repo options.
		&cli.StringFlag{
			Name: "repo-path", Aliases: []string{"rpath"}, Usage: "path for file repo storage",
			EnvVars: []string{"REPO_PATH"}, Value: filerepo.DefaultOptions.BasePath, Destination: &repoOptions.BasePath,
		},
		&cli.Float64Flag{
			Name: "repo-max-disk-usage", Aliases: []string{"rdu"}, Usage: "max allowed file repo max disk usage",
			EnvVars: []string{"REPO_MAX_DISK_USAGE"}, Value: filerepo.DefaultOptions.MaxDiskUsage, Destination: &repoOptions.MaxDiskUsage,
		},
		&cli.DurationFlag{
			Name: "repo-ttl", Aliases: []string{"rttl"}, Usage: "ttl for file repo storage",
			EnvVars: []string{"REPO_TTL"}, Value: filerepo.DefaultOptions.TTL, Destination: &repoOptions.TTL,
		},
		&cli.IntFlag{
			Name: "repo-capacity", Aliases: []string{"rcap"}, Usage: "max capacity for file repo storage",
			EnvVars: []string{"REPO_CAPACITY"}, Value: filerepo.DefaultOptions.Capacity, Destination: &repoOptions.Capacity,
		},
		&cli.StringFlag{
			Name: "lb-addr", Aliases: []string{"lb"}, Usage: "load balancer TCP address",
			EnvVars: []string{"LB_ADDR"}, Value: "127.0.0.1:9000",
		},

		// User Cache options.
		&cli.IntFlag{
			Name: "cache-max-key", Usage: "max allowed user cache key length",
			EnvVars: []string{"CACHE_MAX_KEY"}, Value: script.DefaultUserCacheConstraints.MaxKeyLen, Destination: &scriptOptions.UserCacheConstraints.MaxKeyLen,
		},
		&cli.IntFlag{
			Name: "cache-max-value", Usage: "max allowed user cache value length",
			EnvVars: []string{"CACHE_MAX_VALUE"}, Value: script.DefaultUserCacheConstraints.MaxValueLen, Destination: &scriptOptions.UserCacheConstraints.MaxValueLen,
		},
		&cli.IntFlag{
			Name: "cache-cardinality-limit", Usage: "max number of elements in user cache (per user)",
			EnvVars: []string{"CACHE_CARDINALITY_LIMIT"}, Value: script.DefaultUserCacheConstraints.CardinalityLimit, Destination: &scriptOptions.UserCacheConstraints.CardinalityLimit,
		},
		&cli.IntFlag{
			Name: "cache-size-limit", Usage: "max total size of elements in user cache (per user)",
			EnvVars: []string{"CACHE_SIZE_LIMIT"}, Value: script.DefaultUserCacheConstraints.SizeLimit, Destination: &scriptOptions.UserCacheConstraints.SizeLimit,
		},
		&cli.DurationFlag{
			Name: "cache-default-timeout", Usage: "default user cache timeout",
			EnvVars: []string{"CACHE_DEFAULT_TIMEOUT"}, Value: script.DefaultUserCacheConstraints.DefaultTimeout, Destination: &scriptOptions.UserCacheConstraints.DefaultTimeout,
		},
		&cli.Int64Flag{
			Name: "cache-ratelimit-capacity", Usage: "user cache capacity of per container ratelimit",
			EnvVars: []string{"CACHE_RATELIMIT_CAPACITY"}, Value: script.DefaultOptions.UserCacheStreamCapacityLimit, Destination: &scriptOptions.UserCacheStreamCapacityLimit,
		},
		&cli.Int64Flag{
			Name: "cache-ratelimit-quantum", Usage: "user cache quantum (per minute) of per container ratelimit",
			EnvVars: []string{"CACHE_RATELIMIT_QUANTUM"}, Value: script.DefaultOptions.UserCacheStreamPerMinuteQuantum, Destination: &scriptOptions.UserCacheStreamPerMinuteQuantum,
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
		dockerOptions.DNS = c.StringSlice("docker-dns")
		dockerOptions.ExtraHosts = c.StringSlice("docker-extra-hosts")

		logrus.WithField("options", dockerOptions).Debug("Initializing docker manager")
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
				if util.IsDeadlineExceeded(err) {
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
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(&ocgrpc.ServerHandler{}),
		grpc.MaxRecvMsgSize(common.MaxGRPCMessageSize),
		grpc.MaxSendMsgSize(common.MaxGRPCMessageSize),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    common.KeepaliveParamsTime,
			Timeout: common.KeepaliveParamsTimeout,
		}),
	)
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
	conn, err := grpc.Dial(lbAddr,
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(common.MaxGRPCMessageSize)),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
	)
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
				Runtime:     cont.Runtime,
				ContainerId: cont.ID,
				SourceHash:  cont.SourceHash,
				Environment: cont.Environment,
				UserId:      cont.UserID,
				Entrypoint:  cont.Entrypoint,
				Async:       cont.Async,
				Mcpu:        cont.MCPU,
			}); err != nil {
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
			Mcpu:        uint32(scriptOptions.MCPU - dockerOptions.ReservedMCPU),
			Memory:      syschecker.AvailableMemory(),
			DefaultMcpu: uint32(scriptOptions.Constraints.CPULimit / 1e6),
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
