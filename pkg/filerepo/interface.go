package filerepo

import (
	"io"

	"github.com/spf13/afero"
)

// Repo provides methods to store files and manipulate volumes.
//go:generate mockery -name Repo
type Repo interface {
	Options() Options
	StoragePath() string
	Store(key, storeKey string, src io.Reader, filename string) (path string, err error)
	StoreLock(key string) (ch chan struct{}, storeKey string)
	StoreUnlock(key, storeKey string, ch chan struct{}, save bool)
	PermStore(key string, src io.Reader, filename string) (path string, err error)
	Get(key string) string
	Delete(key string)
	CreateVolume() (string, string, error)
	CleanupVolume(volKey string) error
	DeleteVolume(volKey string) error
	RelativePath(path string) (string, error)
	Link(volKey, resKey, destName string) error
	GetFS() afero.Fs
	CleanupUnused()
	Flush()
	Shutdown()
}

// Assert that FsRepo is compatible with our interface.
var _ Repo = (*FsRepo)(nil)

// Fs provides afero.Fs methods and extends it with Link.
//go:generate mockery -inpkg -testonly -name Fs
type Fs interface {
	afero.Fs
	Link(oldname, newname string) error
}

// Assert that LinkFs is compatible with our interface.
var _ Fs = (*LinkFs)(nil)
