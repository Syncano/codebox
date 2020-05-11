package lb

import pb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/lb/v1"

//go:generate go run github.com/vektra/mockery/cmd/mockery -name ScriptRunnerClient
type ScriptRunnerClient interface {
	pb.ScriptRunnerClient
}

//go:generate go run github.com/vektra/mockery/cmd/mockery -name ScriptRunner_RunServer
type ScriptRunner_RunServer interface { // nolint
	pb.ScriptRunner_RunServer
}

//go:generate go run github.com/vektra/mockery/cmd/mockery -name ScriptRunner_RunClient
type ScriptRunner_RunClient interface { // nolint
	pb.ScriptRunner_RunClient
}
