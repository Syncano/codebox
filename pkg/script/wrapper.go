package script

import (
	"bytes"
	"encoding/json"
	"io"
	"time"

	"github.com/Syncano/codebox/assets"
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
}

type contextFile struct {
	Name        string `json:"name"`
	Filename    string `json:"fname"`
	ContentType string `json:"ct"`
	Length      int    `json:"length"`
}

type wrapperContext struct {
	Async       bool             `json:"_async"`
	MuxResponse int              `json:"_response"`
	EntryPoint  string           `json:"_entryPoint"`
	Timeout     time.Duration    `json:"_timeout"`
	Files       []contextFile    `json:"_files"`
	Args        *json.RawMessage `json:"ARGS"`
	Config      *json.RawMessage `json:"CONFIG"`
	Meta        *json.RawMessage `json:"META"`
}

// SupportedRuntimes defines info and constraints of all runtimes.
var SupportedRuntimes = map[string]*RuntimeInfo{
	"nodejs_v6": {
		FileName:  "node.js",
		AssetName: "wrappers/node.js",
		Command: []string{
			"node",
			"--max_old_space_size=256",
			"/app/wrapper/node.js",
		},
		Environment:       []string{"NODE_PATH=/app/env/node_modules:/app/code"},
		Image:             "node:6-stretch",
		User:              "node",
		DefaultEntryPoint: "main.js",
	},

	"nodejs_v8": {
		FileName:  "node.js",
		AssetName: "wrappers/node.js",
		Command: []string{
			"node",
			"--max_old_space_size=256",
			"/app/wrapper/node.js",
		},
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
