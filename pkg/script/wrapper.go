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
	Command           []string
	Environment       []string
	Image             string
	User              string
	DefaultEntryPoint string
	Constraints       docker.Constraints
}

type contextFile struct {
	Name        string `json:"name"`
	Filename    string `json:"fname"`
	ContentType string `json:"ct"`
	Length      int    `json:"length"`
}

type wrapperContext struct {
	EntryPoint      string           `json:"_entryPoint"`
	MagicString     string           `json:"_magicString"`
	OutputSeparator string           `json:"_outputSeparator"`
	Timeout         time.Duration    `json:"_timeout"`
	Files           []contextFile    `json:"_files"`
	Args            *json.RawMessage `json:"ARGS"`
	Config          *json.RawMessage `json:"CONFIG"`
	Meta            *json.RawMessage `json:"META"`
}

var defaultConstraints = docker.Constraints{
	MemoryLimit:     256 * 1024 * 1024,
	MemorySwapLimit: 0,
	PidLimit:        32,
	NofileUlimit:    1024,
	StorageLimit:    "500M",
}

// SupportedRuntimes defines info and constraints of all runtimes.
var SupportedRuntimes = map[string]*RuntimeInfo{
	"nodejs_v6": {
		FileName:  "node.js",
		AssetName: "wrappers/node.js",
		Command: []string{
			"node",
			fmt.Sprintf("--max_old_space_size=%d", defaultConstraints.MemoryLimit/1024/1024),
			"/app/wrapper/node.js",
		},
		Environment:       []string{"NODE_PATH=/app/env/node_modules:/app/code"},
		Image:             "node:6-stretch",
		User:              "node",
		DefaultEntryPoint: "main.js",
		Constraints:       defaultConstraints,
	},

	"nodejs_v8": {
		FileName:  "node.js",
		AssetName: "wrappers/node.js",
		Command: []string{
			"node",
			fmt.Sprintf("--max_old_space_size=%d", defaultConstraints.MemoryLimit/1024/1024),
			"/app/wrapper/node.js",
		},
		Environment:       []string{"NODE_PATH=/app/env/node_modules:/app/code"},
		Image:             "node:8-stretch",
		User:              "node",
		DefaultEntryPoint: "main.js",
		Constraints:       defaultConstraints,
	},
}

// Wrapper returns io.Reader with contents of wrapper.
func (ri *RuntimeInfo) Wrapper() io.Reader {
	return bytes.NewReader(assets.MustAsset(ri.AssetName))
}
