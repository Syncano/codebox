package filerepo

import (
	"io"
	"os"

	"github.com/spf13/afero"
)

// Repo provides methods to store files and manipulate volumes.
//go:generate go run github.com/vektra/mockery/cmd/mockery -name Repo
type Repo interface {
	Options() Options
	StoragePath() string
	Store(key, storeKey string, src io.Reader, filename string, mode os.FileMode) (path string, err error)
	StoreLock(key string) (ch chan struct{}, storeKey string)
	StoreUnlock(key, storeKey string, ch chan struct{}, save bool)
	PermStore(key string, src io.Reader, filename string, mode os.FileMode) (path string, err error)
	Get(key string) string
	Delete(key string)
	CreateVolume() (string, string, error)
	CleanupVolume(volKey string) error
	DeleteVolume(volKey string) error
	RelativePath(path string) (string, error)
	Link(volKey, resKey, destName string) error
	Mount(volKey, resKey, fileName, destName string) error
	GetFS() afero.Fs
	CleanupUnused()
	Flush()
	Shutdown()
}

// Assert that FsRepo is compatible with our interface.
var _ Repo = (*FsRepo)(nil)

// Fs provides afero.Fs methods and extends it with Link.
//go:generate go run github.com/vektra/mockery/cmd/mockery -inpkg -testonly -name Fs
type Fs interface {
	afero.Fs
	Link(oldname, newname string) error
}

// Assert that LinkFs is compatible with our interface.
var _ Fs = (*LinkFs)(nil)

// Commander provides methods for running subprocesses.
//go:generate go run github.com/vektra/mockery/cmd/mockery -inpkg -testonly -name Commander
type Commander interface {
	Run(string, ...string) error
}

// Assert that Command is compatible with our interface.
var _ Commander = (*Command)(nil)
