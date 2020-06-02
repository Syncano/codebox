package lb

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// ErrUnknownWorkerID signals that we received a message for unknown worker.
	ErrUnknownWorkerID = status.Error(codes.InvalidArgument, "unknown worker id")
	// ErrNoWorkersAvailable signals that there are no suitable workers at this moment.
	ErrNoWorkersAvailable = status.Error(codes.ResourceExhausted, "no workers available")
)
