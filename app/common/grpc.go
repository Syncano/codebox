package common

import "time"

const (
	// MaxGRPCMessageSize defines max send/recv grpc payload.
	MaxGRPCMessageSize = 20 << 20

	// KeepaliveParamsTime defines duration after which server will ping client to see if the transport is still alive.
	KeepaliveParamsTime = 10 * time.Second
	// KeepaliveParamsTimeout defines duration that server waits for client to respond.
	KeepaliveParamsTimeout = 5 * time.Second
)
