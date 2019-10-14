package script

import (
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Syncano/codebox/pkg/filerepo"
	pb "github.com/Syncano/codebox/pkg/script/proto"
	"github.com/Syncano/codebox/pkg/util"
)

const (
	// ChunkARGS is used to define script ARGS in chunk message.
	ChunkARGS = "__args__"
	chunkSize = 2 * 1024 * 1024
)

var (
	initOnceScript              sync.Once
	executionDurationsHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "codebox_execution_duration_seconds",
		Help:    "Codebox execution latency distributions.",
		Buckets: []float64{.1, .25, .5, 1, 2.5, 10, 30, 60, 120, 180},
	})
	overheadDurationsHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "codebox_overhead_duration_seconds",
		Help: "Codebox overhead latency distributions.",
	})
	executionCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codebox_executions_total",
			Help: "Codebox executions.",
		},
		[]string{"code"},
	)
)

// Server describes a Script Runner server.
//go:generate go run github.com/vektra/mockery/cmd/mockery -dir proto -all
type Server struct {
	Runner Runner
}

// Assert that Server is compatible with proto interface.
var _ pb.ScriptRunnerServer = (*Server)(nil)

// NewServer initializes new worker server.
func NewServer(runner Runner) *Server {
	// Register prometheus exports.
	initOnceScript.Do(func() {
		prometheus.MustRegister(
			executionDurationsHistogram,
			overheadDurationsHistogram,
			executionCounter,
		)
	})

	return &Server{Runner: runner}
}

// Run runs script in secure environment of worker.
func (s *Server) Run(stream pb.ScriptRunner_RunServer) error {
	var meta *pb.RunRequest_MetaMessage

	chunkData := make(map[string]File)

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		switch v := in.Value.(type) {
		case *pb.RunRequest_Meta:
			meta = v.Meta
		case *pb.RunRequest_Chunk:
			chunkData[v.Chunk.Name] = File{
				Filename:    v.Chunk.Filename,
				ContentType: v.Chunk.ContentType,
				Data:        v.Chunk.Data,
			}
		}
	}

	if meta == nil {
		return nil
	}

	if meta.RequestID == "" {
		meta.RequestID = util.GenerateKey()
	}

	peerAddr := util.PeerAddr(stream.Context())
	logger := logrus.WithField("peer", peerAddr)
	logger.WithFields(logrus.Fields{
		"runtime":    meta.Runtime,
		"sourceHash": meta.SourceHash,
		"userID":     meta.UserID,
	}).Debug("grpc:script:Run start")

	// Call Run with given options.
	opts := meta.GetOptions()

	// Use chunk ARGS, fallback to run args.
	argsData, ok := chunkData[ChunkARGS]

	var args []byte

	if ok {
		args = argsData.Data

		delete(chunkData, ChunkARGS)
	} else {
		args = opts.GetArgs()
	}

	ret, err := s.Runner.Run(stream.Context(), logger, meta.Runtime, meta.RequestID, meta.SourceHash, meta.Environment, meta.UserID,
		&RunOptions{
			EntryPoint:  opts.GetEntryPoint(),
			OutputLimit: opts.GetOutputLimit(),
			Timeout:     time.Duration(opts.GetTimeout()) * time.Millisecond,
			MCPU:        opts.GetMCPU(),
			Async:       opts.GetAsync(),

			Args:   args,
			Meta:   opts.GetMeta(),
			Config: opts.GetConfig(),
			Files:  chunkData,
		})

	if ret == nil && err != nil {
		return s.ParseError(err)
	}

	executionCounter.WithLabelValues(strconv.Itoa(ret.Code)).Inc()
	executionDurationsHistogram.Observe(ret.Took.Seconds())
	overheadDurationsHistogram.Observe(ret.Overhead.Seconds()) // docker overhead

	// Send response if we got any.
	if ret != nil {
		e := s.sendResponse(stream, ret)
		if e != nil {
			return e
		}
	}

	return nil
}

// sendResponse sends response back through grpc channel, chunking if needed.
func (s *Server) sendResponse(stream pb.ScriptRunner_RunServer, ret *Result) error {
	// Prepare response.
	var (
		httpResponse *pb.HTTPResponseMessage
		content      []byte
	)

	if ret.Response != nil {
		httpResponse = &pb.HTTPResponseMessage{
			StatusCode:  int32(ret.Response.StatusCode),
			ContentType: ret.Response.ContentType,
			Content:     ret.Response.Content,
			Headers:     ret.Response.Headers,
		}
		if len(ret.Response.Content) > chunkSize {
			httpResponse.Content = httpResponse.Content[:chunkSize]
			content = ret.Response.Content[chunkSize:]
		}
	}

	resTook := (int64(ret.Took) + 500000) / 1e6
	if resTook == 0 {
		resTook = 1
	}

	responses := []*pb.RunResponse{
		{
			ContainerID: ret.ContainerID,
			Code:        int32(ret.Code),
			Stdout:      ret.Stdout,
			Stderr:      ret.Stderr,
			Response:    httpResponse,
			Took:        resTook,
			Time:        time.Now().UnixNano(),
			Cached:      ret.Cached,
			Weight:      uint32(ret.Weight),
		},
	}

	// Chunk rest of http content if needed.
	for len(content) > 0 {
		cut := chunkSize
		if len(content) < chunkSize {
			cut = len(content)
		}

		responses = append(responses, &pb.RunResponse{
			Response: &pb.HTTPResponseMessage{Content: content[:cut]},
		})
		content = content[cut:]
	}

	for _, res := range responses {
		if err := stream.Send(res); err != nil {
			return err
		}
	}

	return nil
}

// ParseError converts standard error to gRPC error with detected code.
func (s *Server) ParseError(err error) error {
	code := codes.Internal

	switch err {
	case ErrPoolNotRunning:
		code = codes.ResourceExhausted
	case filerepo.ErrResourceNotFound:
		code = codes.FailedPrecondition
	}

	return status.Error(code, err.Error())
}
