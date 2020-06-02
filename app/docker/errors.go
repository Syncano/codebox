package docker

import "errors"

// ErrReservedCPUTooHigh signals that too we are trying to reserve more cpu than we got.
var ErrReservedCPUTooHigh = errors.New("value of reserved cpu is higher than available")
