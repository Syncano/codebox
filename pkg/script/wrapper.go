package script

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/Syncano/codebox/assets"
	"github.com/Syncano/codebox/pkg/docker"
)

// RuntimeInfo describes a supported runtime.
type RuntimeInfo struct {
	FileName          string
	AssetName         string
	Command           func(*docker.Constraints) []string
	Environment       []string
	Image             string
	User              string
	DefaultEntryPoint string
}

type contextFile struct {
	Name        string `json:"name"`
	Filename    string `json:"fname"`
	ContentType string `json:"ct"`
	Length      int    `json:"length"`
}

type scriptSetup struct {
	Async      bool          `json:"async"`
	EntryPoint string        `json:"entryPoint"`
	Timeout    time.Duration `json:"timeout"`
}

type scriptContext struct {
	Delim  string           `json:"_delim"`
	Files  []contextFile    `json:"_files"`
	Args   *json.RawMessage `json:"ARGS"`
	Config *json.RawMessage `json:"CONFIG"`
	Meta   *json.RawMessage `json:"META"`
}

func nodeCommand(constraints *docker.Constraints) []string {
	return []string{
		"node",
		fmt.Sprintf("--max_old_space_size=%d", constraints.MemoryLimit/1024/1024),
		"/app/wrapper/node.js",
	}
}

// SupportedRuntimes defines info and constraints of all runtimes.
var SupportedRuntimes = map[string]*RuntimeInfo{
	"nodejs_v6": {
		FileName:          "node.js",
		AssetName:         "wrappers/node.js",
		Command:           nodeCommand,
		Environment:       []string{"NODE_PATH=/app/env/node_modules:/app/code"},
		Image:             "node:6-stretch",
		User:              "node",
		DefaultEntryPoint: "main.js",
	},

	"nodejs_v8": {
		FileName:          "node.js",
		AssetName:         "wrappers/node.js",
		Command:           nodeCommand,
		Environment:       []string{"NODE_PATH=/app/env/node_modules:/app/code"},
		Image:             "node:8-stretch",
		User:              "node",
		DefaultEntryPoint: "main.js",
	},
}

// Wrapper returns io.Reader with contents of wrapper.
func (ri *RuntimeInfo) Wrapper() io.Reader {
	return bytes.NewReader(assets.MustAsset(ri.AssetName))
}
