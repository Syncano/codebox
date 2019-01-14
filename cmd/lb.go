package cmd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"google.golang.org/grpc"

	"github.com/Syncano/codebox/cmd/autoscaler"
	"github.com/Syncano/codebox/pkg/filerepo"
	repopb "github.com/Syncano/codebox/pkg/filerepo/proto"
	"github.com/Syncano/codebox/pkg/lb"
	lbpb "github.com/Syncano/codebox/pkg/lb/proto"
	"github.com/Syncano/codebox/pkg/sys"
	"github.com/Syncano/codebox/pkg/version"
)

var (
	lbOptions         = lb.ServerOptions{}
	autoscalerOptions = autoscaler.Options{}
)

var lbCmd = cli.Command{
	Name:  "lb",
	Usage: "Starts load balancer responsible for controlling workers.",
	Description: `Load balancer orchestrates workers and distributes work among them. 
As there is no authentication, always run it in a private network.`,
	Flags: []cli.Flag{
		cli.IntFlag{
			Name: "port, p", Usage: "port for grpc server",
			EnvVar: "PORT", Value: 9000,
		},

		// File Repo options.
		cli.StringFlag{
			Name: "repo-path, path", Usage: "path for file repo storage",
			EnvVar: "REPO_PATH", Value: filerepo.DefaultOptions.BasePath, Destination: &repoOptions.BasePath,
		},
		cli.Float64Flag{
			Name: "repo-max-disk-usage, u", Usage: "max allowed file repo max disk usage",
			EnvVar: "REPO_MAX_DISK_USAGE", Value: filerepo.DefaultOptions.MaxDiskUsage, Destination: &repoOptions.MaxDiskUsage,
		},
		cli.DurationFlag{
			Name: "repo-ttl, l", Usage: "ttl for file repo storage",
			EnvVar: "REPO_TTL", Value: filerepo.DefaultOptions.TTL, Destination: &repoOptions.TTL,
		},
		cli.IntFlag{
			Name: "repo-capacity, c", Usage: "max capacity for file repo storage",
			EnvVar: "REPO_CAPACITY", Value: filerepo.DefaultOptions.Capacity, Destination: &repoOptions.Capacity,
		},

		// LB Server options.
		cli.DurationFlag{
			Name: "worker-keepalive, k", Usage: "max allowed worker keepalive",
			EnvVar: "WORKER_KEEPALIVE", Value: lb.DefaultOptions.WorkerKeepalive, Destination: &lbOptions.WorkerKeepalive,
		},
		cli.IntFlag{
			Name: "worker-retry", Usage: "number of retries on failed worker run",
			EnvVar: "WORKER_RETRY", Value: lb.DefaultOptions.WorkerRetry, Destination: &lbOptions.WorkerRetry,
		},
		cli.IntFlag{
			Name: "worker-min-ready", Usage: "number of retries on failed worker run",
			EnvVar: "WORKER_MIN_READY", Destination: &lbOptions.WorkerMinReady,
		},

		// Autoscaler options.
		cli.BoolFlag{
			Name: "scaling-enabled", Usage: "enable scaling",
			EnvVar: "SCALING_ENABLED",
		},
		cli.StringFlag{
			Name: "scaling-deployment", Usage: "deployment to scale",
			EnvVar: "SCALING_DEPLOYMENT", Value: autoscaler.DefaultOptions.Deployment, Destination: &autoscalerOptions.Deployment,
		},
		cli.DurationFlag{
			Name: "scaling-cooldown", Usage: "max allowed worker keepalive",
			EnvVar: "SCALING_COOLDOWN", Value: autoscaler.DefaultOptions.Cooldown, Destination: &autoscalerOptions.Cooldown,
		},
		cli.IntFlag{
			Name: "scaling-min", Usage: "minimum number of worker replicas",
			EnvVar: "SCALING_MIN", Value: autoscaler.DefaultOptions.MinScale, Destination: &autoscalerOptions.MinScale,
		},
		cli.IntFlag{
			Name: "scaling-max", Usage: "maximum number of worker replicas",
			EnvVar: "SCALING_MAX", Value: autoscaler.DefaultOptions.MaxScale, Destination: &autoscalerOptions.MaxScale,
		},
		cli.UintFlag{
			Name: "scaling-freecpu-min", Usage: "minimum number of freecpu to maintain",
			EnvVar: "SCALING_FREECPU_MIN", Value: autoscaler.DefaultOptions.MinFreeCPU, Destination: &autoscalerOptions.MinFreeCPU,
		},
		cli.UintFlag{
			Name: "scaling-freecpu-max", Usage: "maximum number of freecpu to maintain",
			EnvVar: "SCALING_FREECPU_MAX", Value: autoscaler.DefaultOptions.MaxFreeCPU, Destination: &autoscalerOptions.MaxFreeCPU,
		},
	},
	Action: func(c *cli.Context) error {
		logrus.WithFields(logrus.Fields{
			"version":   App.Version,
			"gitsha":    version.GitSHA,
			"buildtime": App.Compiled,
		}).Info("Load balancer starting")

		// Initialize system checker.
		syschecker := new(sys.SigarChecker)

		// Initialize file repo.
		logrus.WithField("options", repoOptions).Debug("Initializing file repo")
		repo := filerepo.New(repoOptions, syschecker, new(filerepo.LinkFs), new(filerepo.Command))

		// Create new gRPC server.
		logrus.WithField("options", lbOptions).Debug("Initializing load balancer server")
		lbServer := lb.NewServer(repo, lbOptions)

		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", c.Int("port")))
		if err != nil {
			return err
		}
		grpcServer := grpc.NewServer(sys.DefaultGRPCServerOptions...)

		// Register all servers.
		lbpb.RegisterWorkerPlugServer(grpcServer, lbServer)
		lbpb.RegisterScriptRunnerServer(grpcServer, lbServer)
		repopb.RegisterRepoServer(grpcServer, &filerepo.Server{Repo: repo})

		// Serve a new gRPC service.
		go func() {
			if err = grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
				logrus.WithError(err).Fatal("GRPC serve error")
			}
		}()
		logrus.WithField("port", c.Int("port")).Info("Serving gRPC")

		// Setup healthcheck.
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		})
		http.HandleFunc("/ready", lbServer.ReadyHandler)

		// Start autoscaler.
		if c.Bool("scaling-enabled") {
			logrus.Info("Starting autoscaler")
			scaler, err := autoscaler.New(autoscalerOptions)
			if err != nil {
				return err
			}
			scalingStop := scaler.Start()
			defer close(scalingStop)
		}

		// Handle SIGINT and SIGTERM.
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch

		// Graceful shutdown.
		logrus.Info("Shutting down")
		grpcServer.GracefulStop()
		lbServer.Shutdown()
		repo.Shutdown()
		return nil
	},
}

func init() {
	App.Commands = append(App.Commands, lbCmd)
}
