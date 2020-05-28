package cmd

import (
	_ "expvar" // Register expvar default http handler.
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"time"

	"contrib.go.opencensus.io/exporter/jaeger"
	"contrib.go.opencensus.io/exporter/prometheus"
	"github.com/doloopwhile/logrusltsv"
	"github.com/evalphobia/logrus_sentry"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc/grpclog"

	"github.com/Syncano/codebox/app/filerepo"
	"github.com/Syncano/codebox/app/version"
	"github.com/Syncano/codebox/cmd/stack"
)

var (
	repoOptions = &filerepo.Options{}

	// App is the main structure of a cli application.
	App = cli.NewApp()

	jaegerExporter *jaeger.Exporter
)

type logrusWrapper struct {
	*logrus.Logger
}

// V provides the functionality that returns whether a particular log level is at
// least l - this is needed to meet the LoggerV2 interface.  GRPC's logging levels
// are: https://github.com/grpc/grpc-go/blob/master/grpclog/loggerv2.go#L71
// 0=info, 1=warning, 2=error, 3=fatal
// logrus's are: https://github.com/sirupsen/logrus/blob/master/logrus.go
// 0=panic, 1=fatal, 2=error, 3=warn, 4=info, 5=debug
func (lw logrusWrapper) V(l int) bool {
	// translate to logrus level
	logrusLevel := 4 - l
	return int(lw.Logger.Level) <= logrusLevel
}

func initializeLogging(c *cli.Context) error {
	level := logrus.InfoLevel
	if c.Bool("debug") {
		level = logrus.DebugLevel
	}

	logrus.SetLevel(level)

	// Use JSON formatter if there is no terminal detected.
	if os.Getenv("FORCE_COLORS") == "1" {
		logrus.StandardLogger().Formatter.(*logrus.TextFormatter).ForceColors = true
	} else if !terminal.IsTerminal(int(os.Stdout.Fd())) {
		logrus.SetFormatter(new(logrusltsv.Formatter))
	}

	// Setup logrus stack trace hook.
	hook := stack.NewHook(
		"github.com/Syncano/codebox",
		[]logrus.Level{logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel, logrus.WarnLevel},
		[]logrus.Level{logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel},
	)
	logrus.AddHook(hook)

	// Setup sentry hook if DSN is defined.
	if c.IsSet("dsn") {
		hook, err := logrus_sentry.NewSentryHook(
			c.String("dsn"),
			[]logrus.Level{
				logrus.PanicLevel,
				logrus.FatalLevel,
				logrus.ErrorLevel,
			})
		if err != nil {
			return err
		}

		hook.StacktraceConfiguration.Enable = true
		hook.StacktraceConfiguration.Context = 7
		hook.StacktraceConfiguration.InAppPrefixes = []string{"github.com/Syncano/codebox"}
		hook.Timeout = 5 * time.Second

		logrus.AddHook(hook)
	}

	// Set grpc logger.
	log := logrus.StandardLogger()
	grpclog.SetLoggerV2(logrusWrapper{log})

	return nil
}

func init() {
	App.Name = "Codebox"
	App.Usage = "Application that enables running user provided unsecure code in a secure docker environment."
	App.Compiled = version.Buildtime
	App.Version = version.Current.String()
	App.Authors = []*cli.Author{
		{
			Name:  "Robert Kopaczewski",
			Email: "rk@23doors.com",
		},
	}
	App.Copyright = "(c) 2017-2020 Syncano"
	App.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name: "debug", Usage: "enable debug mode",
			EnvVars: []string{"DEBUG"},
		},
		&cli.StringFlag{
			Name: "dsn", Usage: "enable sentry logging",
			EnvVars: []string{"SENTRY_DSN"},
		},
		&cli.IntFlag{
			Name: "metric-port", Aliases: []string{"mp"}, Usage: "port for expvar server",
			EnvVars: []string{"METRIC_PORT"}, Value: 9080,
		},

		// Tracing options.
		&cli.StringFlag{
			Name: "jaeger-collector-endpoint", Usage: "jaeger collector endpoint",
			EnvVars: []string{"JAEGER_COLLECTOR_ENDPOINT"}, Value: "http://jaeger:14268/api/traces",
		},
		&cli.Float64Flag{
			Name: "tracing-sampling", Usage: "tracing sampling probability value",
			EnvVars: []string{"TRACING_SAMPLING"}, Value: 0,
		},
		&cli.StringFlag{
			Name: "service-name", Aliases: []string{"n"}, Usage: "service name",
			EnvVars: []string{"SERVICE_NAME"}, Value: "codebox",
		},
	}
	App.Before = func(c *cli.Context) error {
		// Initialize random seed.
		rand.Seed(time.Now().UTC().UnixNano())

		numCPUs := runtime.NumCPU()
		runtime.GOMAXPROCS(numCPUs + 1) // numCPUs hot threads + one for async tasks.

		// Initialize logging.
		if err := initializeLogging(c); err != nil {
			return err
		}

		// Serve expvar and checks.
		logrus.WithField("port", c.Int("metric-port")).Info("Serving http for expvar and checks")

		go func() {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", c.Int("metric-port")), nil); err != nil && err != http.ErrServerClosed {
				logrus.WithError(err).Fatal("Serve error")
			}
		}()

		// Setup prometheus handler.
		exporter, err := prometheus.NewExporter(prometheus.Options{})
		if err != nil {
			logrus.WithError(err).Fatal("Prometheus exporter misconfiguration")
		}

		var views []*view.View
		views = append(views, ochttp.DefaultClientViews...)
		views = append(views, ochttp.DefaultServerViews...)
		views = append(views, ocgrpc.DefaultClientViews...)
		views = append(views, ocgrpc.DefaultServerViews...)

		if err := view.Register(views...); err != nil {
			logrus.WithError(err).Fatal("Opencensus views registration failed")
		}

		// Serve prometheus metrics.
		http.Handle("/metrics", exporter)

		// Initialize tracing.
		jaegerExporter, err = jaeger.NewExporter(jaeger.Options{
			CollectorEndpoint: c.String("jaeger-collector-endpoint"),
			Process: jaeger.Process{
				ServiceName: c.String("service-name"),
			},
			OnError: func(err error) {
				logrus.WithError(err).Warn("Jaeger tracing error")
			},
		})
		if err != nil {
			logrus.WithError(err).Fatal("Jaeger exporter misconfiguration")
		}

		trace.RegisterExporter(jaegerExporter)
		trace.ApplyConfig(trace.Config{
			DefaultSampler: trace.ProbabilitySampler(c.Float64("tracing-sampling")),
		})

		return nil
	}
	App.After = func(c *cli.Context) error {
		// Close tracing reporter.
		jaegerExporter.Flush()

		return nil
	}
}
