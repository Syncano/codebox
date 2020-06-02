package script

import (
	"fmt"

	"github.com/mitchellh/hashstructure"

	lbpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/lb/v1"
	scriptpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"
)

// ScriptInfo defines unique container information.
type ScriptInfo struct { // nolint: golint
	Runtime     string
	SourceHash  string
	Environment string
	UserID      string
	Entrypoint  string
	MCPU        uint32
	Async       uint32
}

func (si *ScriptInfo) String() string {
	return fmt.Sprintf("{Runtime:%s, SourceHash:%s, Environment:%s, UserID:%s, Async:%d, MCPU:%d}", si.Runtime, si.SourceHash, si.Environment, si.UserID, si.Async, si.MCPU)
}

func (si *ScriptInfo) Hash() string {
	hash, err := hashstructure.Hash(si, nil)

	if err != nil {
		panic(err)
	}

	return fmt.Sprintf("%x", hash)
}

func NewScriptInfo(runtime, sourceHash, environment, userID, entrypoint string, mcpu, async uint32) *ScriptInfo {
	if entrypoint == "" {
		entrypoint = SupportedRuntimes[runtime].DefaultEntryPoint
	}

	return &ScriptInfo{
		Runtime:     runtime,
		SourceHash:  sourceHash,
		Environment: environment,
		UserID:      userID,
		Entrypoint:  entrypoint,
		MCPU:        mcpu,
		Async:       async,
	}
}

func CreateScriptInfoFromScriptMeta(meta *scriptpb.RunMeta) *ScriptInfo {
	script := NewScriptInfo(
		meta.Runtime,
		meta.SourceHash,
		meta.Environment,
		meta.UserId,
		meta.GetOptions().GetEntrypoint(),
		meta.GetOptions().GetMcpu(),
		meta.GetOptions().GetAsync(),
	)

	return script
}

func CreateScriptInfoFromContainerRemove(req *lbpb.ContainerRemovedRequest) *ScriptInfo {
	script := NewScriptInfo(
		req.Runtime,
		req.SourceHash,
		req.Environment,
		req.UserId,
		req.Entrypoint,
		req.Mcpu,
		req.Async,
	)

	return script
}
