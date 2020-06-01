package filerepo

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// ErrMissingMeta signals that there are upload format was invalid and was missing upload meta.
	ErrMissingMeta = status.Error(codes.FailedPrecondition, "missing upload meta")

	// ErrResourceNotFound signals that resource key was not found and it was a must.
	ErrResourceNotFound = errors.New("resource not found")
	// ErrVolumeNotFound signals that volume key was not found and it was a must.
	ErrVolumeNotFound = errors.New("volume not found")
	// ErrNotEnoughDiskSpace signals that there is not enough disk space available on storage path.
	ErrNotEnoughDiskSpace = errors.New("not enough disk space")
)
