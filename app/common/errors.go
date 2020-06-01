package common

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// ErrSourceNotAvailable signals that specified source hash was not found.
	ErrSourceNotAvailable = status.Error(codes.FailedPrecondition, "source not available")

	ErrInvalidArgument = status.Error(codes.InvalidArgument, "invalid argument")
)
