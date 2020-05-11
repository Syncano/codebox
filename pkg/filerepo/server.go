package filerepo

import (
	"context"
	"io"
	"os"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Syncano/codebox/pkg/util"
	pb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/filerepo/v1"
)

// Server defines a File Repo server.
type Server struct {
	Repo Repo
}

// Assert that Server is compatible with proto interface.
var _ pb.RepoServer = (*Server)(nil)

var (
	// ErrMissingMeta signals that there are upload format was invalid and was missing upload meta.
	ErrMissingMeta = status.Error(codes.FailedPrecondition, "missing upload meta")
)

// Exists checks if file was defined in file repo.
func (s *Server) Exists(ctx context.Context, in *pb.ExistsRequest) (*pb.ExistsResponse, error) {
	logrus.WithFields(logrus.Fields{"key": in.GetKey(), "peer": util.PeerAddr(ctx)}).Debug("grpc:filerepo:Exists")

	res := new(pb.ExistsResponse)
	res.Ok = s.Repo.Get(in.GetKey()) != ""

	return res, nil
}

// Upload streams file(s) to server.
func (s *Server) Upload(stream pb.Repo_UploadServer) error {
	peerAddr := util.PeerAddr(stream.Context())
	errCh := make(chan error, 1)
	logger := logrus.WithField("peer", peerAddr)

	var (
		meta      *pb.UploadRequest_MetaMessage
		chunkCh   chan []byte
		chunkName string
		lockCh    chan struct{}
		storeKey  string
	)

	defer func() {
		if chunkCh != nil {
			close(chunkCh)
		}

		if lockCh != nil {
			s.Repo.StoreUnlock(meta.GetKey(), storeKey, lockCh, false)
		}
	}()

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		switch in := in.Value.(type) {
		case *pb.UploadRequest_Meta:
			meta = in.Meta
			logger = logger.WithField("key", meta.GetKey())

			if lockCh, storeKey = s.Repo.StoreLock(meta.GetKey()); lockCh == nil {
				logger.Debug("grpc:filerepo:Upload Rejected")
				return stream.Send(&pb.UploadResponse{})
			}

			logger.Debug("grpc:filerepo:Upload Accepted")
			stream.Send(&pb.UploadResponse{Accepted: true}) // nolint - ignore error
		case *pb.UploadRequest_Chunk:
			if meta == nil {
				return ErrMissingMeta
			}

			logger.WithFields(logrus.Fields{
				"storeKey":  storeKey,
				"chunkName": in.Chunk.GetName(),
				"chunkSize": len(in.Chunk.GetData()),
			}).Debug("grpc:filerepo:Upload")

			chunkName, chunkCh, err = s.processChunkUpload(meta.GetKey(), storeKey, chunkName, chunkCh, in.Chunk, errCh)
			if err != nil {
				return err
			}
		case *pb.UploadRequest_Done:
			if chunkCh != nil {
				close(chunkCh)
				chunkCh = nil

				if err := <-errCh; err != nil {
					return err
				}
			}

			if meta != nil {
				logger.Info("grpc:filerepo:Upload Done")
				s.Repo.StoreUnlock(meta.GetKey(), storeKey, lockCh, true)
				lockCh = nil

				return stream.Send(&pb.UploadResponse{})
			}

			return ErrMissingMeta
		}
	}

	return nil
}

func (s *Server) processChunkUpload(key, storeKey, chunkName string, chunkCh chan []byte, chunk *pb.UploadRequest_ChunkMessage,
	errCh chan error) (newChunkName string, newChunkCh chan []byte, err error) {
	// If we are to start a new chunk, close previous chunk channel.
	if chunkCh != nil && chunk.GetName() != chunkName {
		close(chunkCh)
		chunkCh = nil

		if err := <-errCh; err != nil {
			return chunkName, chunkCh, err
		}
	}
	// When starting a new chunk, run it asynchronously.
	if chunkCh == nil {
		chunkName = chunk.GetName()
		chunkCh = make(chan []byte, 1)

		go func() {
			_, err := s.Repo.Store(key, storeKey, &util.ChannelReader{Channel: chunkCh}, chunk.GetName(), os.ModePerm)
			if err != nil {
				err = s.ParseError(err)
				s.Repo.Delete(key)
			}
			errCh <- err
		}()
	}

	chunkCh <- chunk.GetData()

	return chunkName, chunkCh, nil
}

// ParseError converts standard error to gRPC error with detected code.
func (s *Server) ParseError(err error) error {
	code := codes.Internal
	if err == ErrNotEnoughDiskSpace {
		code = codes.ResourceExhausted
	}

	return status.Error(code, err.Error())
}
