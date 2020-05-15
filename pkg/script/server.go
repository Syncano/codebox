package script

import (
	"io"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"

	"github.com/Syncano/codebox/pkg/filerepo"
	"github.com/Syncano/codebox/pkg/util"
)

const (
	// ChunkARGS is used to define script ARGS in chunk message.
	chunkSize = 2 * 1024 * 1024
)

var (
	initOnceScript   sync.Once
	overheadDuration = stats.Float64(
		"codebox/overhead/duration/seconds",
		"Codebox overhead duration.",
		stats.UnitSeconds)

	overheadDurationView = &view.View{
		Name:        "codebox/overhead/duration/seconds",
		Description: "Codebox overhead distribution.",
		Measure:     overheadDuration,
		Aggregation: view.Distribution(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10),
	}
)

// Server describes a Script Runner server.
//go:generate go run github.com/vektra/mockery/cmd/mockery -dir ../../proto/gen/go/syncano/codebox/script -all
type Server struct {
	Runner Runner
}

// Assert that Server is compatible with proto interface.
var _ pb.ScriptRunnerServer = (*Server)(nil)

// NewServer initializes new worker server.
func NewServer(runner Runner) *Server {
	// Register prometheus exports.
	initOnceScript.Do(func() {
		util.Must(view.Register(overheadDurationView))
	})

	return &Server{Runner: runner}
}

// Run runs script in secure environment of worker.
func (s *Server) Run(stream pb.ScriptRunner_RunServer) error {
	var (
		meta     *pb.RunMeta
		argsData *File
	)

	chunkData := make(map[string]*File)

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
			f := File{
				Filename:    v.Chunk.Filename,
				ContentType: v.Chunk.ContentType,
				Data:        v.Chunk.Data,
			}

			switch v.Chunk.Type {
			case pb.RunChunk_GENERIC:
				chunkData[v.Chunk.Name] = &f
			case pb.RunChunk_ARGS:
				argsData = &f
			}
		}
	}

	if meta == nil {
		return nil
	}

	ctx, reqID := util.AddDefaultRequestID(stream.Context())
	peerAddr := util.PeerAddr(ctx)
	logger := logrus.WithFields(logrus.Fields{
		"peer":       peerAddr,
		"reqID":      reqID,
		"runtime":    meta.Runtime,
		"sourceHash": meta.SourceHash,
		"userID":     meta.UserId,
	})

	logger.Debug("grpc:script:Run start")

	// Call Run with given options.
	opts := meta.GetOptions()

	// Use chunk ARGS, fallback to run args.
	var args []byte

	if argsData != nil {
		args = argsData.Data
	} else {
		args = opts.GetArgs()
	}

	ret, err := s.Runner.Run(ctx, logger, meta.Runtime, reqID, meta.SourceHash, meta.Environment, meta.UserId,
		&RunOptions{
			EntryPoint:  opts.GetEntrypoint(),
			OutputLimit: opts.GetOutputLimit(),
			Timeout:     time.Duration(opts.GetTimeout()) * time.Millisecond,
			MCPU:        opts.GetMcpu(),
			Async:       opts.GetAsync(),

			Args:   args,
			Meta:   opts.GetMeta(),
			Config: opts.GetConfig(),
			Files:  chunkData,
		})

	if ret == nil && err != nil {
		logger.WithError(err).Warn("grpc:script:Run error")
		return s.ParseError(err)
	}

	// Send response if we got any.
	if ret != nil {
		stats.Record(ctx, overheadDuration.M(ret.Overhead.Seconds())) //  docker overhead

		logger.WithField("ret", ret).Info("grpc:script:Run")

		e := s.sendResponse(stream, ret)
		if e != nil {
			logger.WithError(err).Warn("grpc:script:Run send response error")
			return e
		}
	}

	return nil
}

// sendResponse sends response back through grpc channel, chunking if needed.
func (s *Server) sendResponse(stream pb.ScriptRunner_RunServer, ret *Result) error {
	// Prepare response.
	var (
		httpResponse *pb.HTTPResponse
		content      []byte
	)

	if ret.Response != nil {
		httpResponse = &pb.HTTPResponse{
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
			ContainerId: ret.ContainerID,
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
			Response: &pb.HTTPResponse{Content: content[:cut]},
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
	case ErrUnsupportedRuntime:
		code = codes.Canceled
	}

	return status.Error(code, err.Error())
}
