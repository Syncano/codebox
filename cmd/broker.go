package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-redis/redis/v7"
	uwsgi "github.com/mattn/go-uwsgi"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"

	"github.com/Syncano/codebox/cmd/amqp"
	"github.com/Syncano/codebox/pkg/broker"
	pb "github.com/Syncano/codebox/pkg/broker/proto"
	"github.com/Syncano/codebox/pkg/celery"
	"github.com/Syncano/codebox/pkg/sys"
	"github.com/Syncano/codebox/pkg/version"
)

var (
	brokerOptions = &broker.ServerOptions{}
)

var brokerCmd = &cli.Command{
	Name:  "broker",
	Usage: "Starts broker to serve as a front for load balancers.",
	Description: `Brokers pass workload in correct way to available load balancers.
As there is no authentication, always run it in a private network.`,
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name: "port", Aliases: []string{"p"}, Usage: "port for grpc server",
			EnvVars: []string{"PORT"}, Value: 8000,
		},
		&cli.IntFlag{
			Name: "uwsgi-port", Usage: "port for uwsgi server",
			EnvVars: []string{"UWSGI_PORT"}, Value: 8080,
		},

		// Redis options.
		&cli.StringFlag{
			Name: "redis-addr", Usage: "redis TCP address",
			EnvVars: []string{"REDIS_ADDR"}, Value: "redis:6379",
		},

		// Broker Server options.
		&cli.StringSliceFlag{
			Name: "lb-addrs", Usage: "load balancer TCP addresses",
			EnvVars: []string{"LB_ADDRS"}, Value: cli.NewStringSlice("127.0.0.1:8000"),
		},
		&cli.IntFlag{
			Name: "lb-retry", Usage: "number of retries on failed lb run",
			EnvVars: []string{"LB_RETRY"}, Value: broker.DefaultOptions.LBRetry, Destination: &brokerOptions.LBRetry,
		},
		&cli.StringFlag{
			Name: "broker-url", Usage: "amqp broker url",
			EnvVars: []string{"BROKER_URL"}, Value: "amqp://admin:mypass@rabbitmq//",
		},
		&cli.UintFlag{
			Name: "download-concurrency", Usage: "download concurrency",
			EnvVars: []string{"DOWNLOAD_CONCURRENCY"}, Value: broker.DefaultOptions.DownloadConcurrency, Destination: &brokerOptions.DownloadConcurrency,
		},
		&cli.Int64Flag{
			Name: "payload-size", Usage: "payload size",
			EnvVars: []string{"MAX_PAYLOAD_SIZE"}, Value: broker.DefaultOptions.MaxPayloadSize, Destination: &brokerOptions.MaxPayloadSize,
		},
	},
	Action: func(c *cli.Context) error {
		logrus.WithFields(logrus.Fields{
			"version":   App.Version,
			"gitsha":    version.GitSHA,
			"buildtime": App.Compiled,
		}).Info("Broker starting")

		// Initialize redis client.
		redisClient := redis.NewClient(&redis.Options{
			Addr:     c.String("redis-addr"),
			Password: "",
			DB:       0,
		})

		// Create new gRPC server.
		logrus.WithField("options", lbOptions).Debug("Initializing broker server")
		amqpChannel := new(amqp.Channel)
		if err := amqpChannel.Init(c.String("broker-url")); err != nil {
			return err
		}
		celery.Init(amqpChannel)
		brokerOptions.LBAddr = c.StringSlice("lb-addrs")
		brokerServer, err := broker.NewServer(redisClient, brokerOptions)
		if err != nil {
			return err
		}

		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", c.Int("port")))
		if err != nil {
			return err
		}
		grpcServer := grpc.NewServer(sys.DefaultGRPCServerOptions...)

		// Register all servers.
		pb.RegisterScriptRunnerServer(grpcServer, brokerServer)

		// Serve a new gRPC service.
		go func() {
			if err = grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
				logrus.WithError(err).Fatal("GRPC serve error")
			}
		}()
		logrus.WithField("port", c.Int("port")).Info("Serving gRPC")

		// Start uwsgi server.
		uwsgiListener, err := net.Listen("tcp", fmt.Sprintf(":%d", c.Int("uwsgi-port")))
		if err != nil {
			return err
		}
		uwsgiServer := &http.Server{Handler: http.HandlerFunc(brokerServer.RunHandler)}
		uwsgiProxy := &uwsgi.Listener{Listener: uwsgiListener}

		go func() {
			if err := uwsgiServer.Serve(uwsgiProxy); err != nil && err != http.ErrServerClosed {
				logrus.WithError(err).Fatal("uwsgi serve error")
			}
		}()
		logrus.WithField("port", c.Int("uwsgi-port")).Info("Serving uwsgi server")

		// Setup health check.
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		})

		// Handle SIGINT and SIGTERM.
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch

		// Graceful shutdown.
		logrus.Info("Shutting down")
		uwsgiServer.Shutdown(context.Background()) // nolint - ignore error
		grpcServer.GracefulStop()
		brokerServer.Shutdown()
		amqpChannel.Shutdown()
		redisClient.Close()
		return nil
	},
}

func init() {
	App.Commands = append(App.Commands, brokerCmd)
}
