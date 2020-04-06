package cmd

import (
	_ "expvar" // Register expvar default http handler.
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/doloopwhile/logrusltsv"
	"github.com/evalphobia/logrus_sentry"
	opentracing "github.com/opentracing/opentracing-go"
	zipkinot "github.com/openzipkin-contrib/zipkin-go-opentracing"
	"github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc/grpclog"

	"github.com/Syncano/codebox/cmd/stack"
	"github.com/Syncano/codebox/pkg/filerepo"
	"github.com/Syncano/codebox/pkg/version"
)

var (
	repoOptions = &filerepo.Options{}

	// App is the main structure of a cli application.
	App = cli.NewApp()
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
		&cli.StringFlag{
			Name: "zipkin-addr", Usage: "zipkin address",
			EnvVars: []string{"ZIPKIN_ADDR"}, Value: "zipkin",
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
		http.Handle("/metrics", promhttp.Handler())

		// Initialize tracing.
		reporter := zipkinhttp.NewReporter(fmt.Sprintf("http://%s:9411/api/v2/spans", c.String("zipkin-addr")))
		defer reporter.Close()

		endpoint, err := zipkin.NewEndpoint(c.String("service-name"), "")
		if err != nil {
			logrus.WithError(err).Fatal("Unable to create local endpoint error")
		}

		// Initialize tracer.
		nativeTracer, err := zipkin.NewTracer(reporter,
			zipkin.WithLocalEndpoint(endpoint),
			zipkin.WithSampler(zipkin.NewModuloSampler(uint64(1/c.Float64("tracing-sampling")))),
		)
		if err != nil {
			logrus.WithError(err).Fatal("Unable to create tracer")
		}

		// Use zipkin-go-opentracing to wrap our tracer.
		tracer := zipkinot.Wrap(nativeTracer)

		opentracing.SetGlobalTracer(tracer)

		return nil
	}
}
