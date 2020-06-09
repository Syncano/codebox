package script

import (
	"fmt"

	"github.com/mitchellh/hashstructure"

	lbpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/lb/v1"
	scriptpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"
)

// Index defines unique script information.
type Index struct { // nolint: golint
	Runtime    string
	SourceHash string
	UserID     string `hash:"ignore"`
}

func (i *Index) Hash() string {
	hash, err := hashstructure.Hash(i, nil)

	if err != nil {
		panic(err)
	}

	return fmt.Sprintf("%s:%x", i.UserID, hash)
}

// Definition defines unique script runtime information.
type Definition struct { // nolint: golint
	*Index
	Environment string
	Entrypoint  string
	MCPU        uint32
	Async       uint32
}

func (d *Definition) String() string {
	return fmt.Sprintf("{Runtime:%s, SourceHash:%s, Environment:%s, UserID:%s, Async:%d, MCPU:%d}", d.Runtime, d.SourceHash, d.Environment, d.UserID, d.Async, d.MCPU)
}

func (d *Definition) Hash() string {
	hash, err := hashstructure.Hash(d, nil)

	if err != nil {
		panic(err)
	}

	return fmt.Sprintf("%s:%x", d.UserID, hash)
}

func NewDefinition(runtime, sourceHash, environment, userID, entrypoint string, mcpu, async uint32) *Definition {
	if entrypoint == "" {
		entrypoint = SupportedRuntimes[runtime].DefaultEntryPoint
	}

	return &Definition{
		Index: &Index{
			Runtime:    runtime,
			SourceHash: sourceHash,
			UserID:     userID,
		},

		Environment: environment,
		Entrypoint:  entrypoint,
		MCPU:        mcpu,
		Async:       async,
	}
}

func CreateDefinitionFromScriptMeta(meta *scriptpb.RunMeta) *Definition {
	script := NewDefinition(
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

func CreateDefinitionFromContainerRemove(req *lbpb.ContainerRemovedRequest) *Definition {
	script := NewDefinition(
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
