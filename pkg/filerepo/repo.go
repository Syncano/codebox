package filerepo

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"github.com/Syncano/codebox/pkg/cache"
	"github.com/Syncano/codebox/pkg/sys"
	"github.com/Syncano/codebox/pkg/util"
)

// FsRepo provides methods to store files and manipulate volumes.
type FsRepo struct {
	storeID   string
	fileCache *cache.LRUCache
	options   Options
	sys       sys.SystemChecker
	fs        Fs
	command   Commander

	muStore   sync.RWMutex
	permStore map[string]string

	muVol   sync.RWMutex
	volumes map[string]*Volume

	muStoreLocks sync.Mutex
	storeLocks   map[string]chan struct{}
}

// Options holds settable options for file repo.
type Options struct {
	BasePath         string
	MaxDiskUsage     float64
	TTL              time.Duration
	Capacity         int
	CleanupInterval  time.Duration
	StoreLockTimeout time.Duration
}

// DefaultOptions holds default options values for file repo.
var DefaultOptions = Options{
	BasePath:         "/home/codebox/storage",
	MaxDiskUsage:     90,
	TTL:              3 * time.Hour,
	Capacity:         300,
	CleanupInterval:  30 * time.Second,
	StoreLockTimeout: 5 * time.Second,
}

// Volume defines info about volume.
type Volume struct {
	Path   string
	mounts []string
}

// Resource defines info about cached script.
type Resource struct {
	Path string
}

var (
	// ErrResourceNotFound signals that resource key was not found and it was a must.
	ErrResourceNotFound = errors.New("resource not found")
	// ErrVolumeNotFound signals that volume key was not found and it was a must.
	ErrVolumeNotFound = errors.New("volume not found")
	// ErrNotEnoughDiskSpace signals that there is not enough disk space available on storage path.
	ErrNotEnoughDiskSpace = errors.New("not enough disk space")
)

const (
	fileStorageName   = "files"
	volumeStorageName = "volumes"
	fusermountCmd     = "fusermount"
	squashfuseCmd     = "squashfuse"
	squashfuseMount   = "squashfuse"
)

// New initializes a new file repo.
func New(options Options, checker sys.SystemChecker, fs Fs, command Commander) *FsRepo {
	mergo.Merge(&options, DefaultOptions) // nolint - error not possible
	util.Must(fs.MkdirAll(options.BasePath, os.ModePerm))
	r := FsRepo{
		storeID:    util.GenerateKey(),
		options:    options,
		sys:        checker,
		fs:         fs,
		command:    command,
		permStore:  make(map[string]string),
		volumes:    make(map[string]*Volume),
		storeLocks: make(map[string]chan struct{}),
	}
	r.fileCache = cache.NewLRUCache(cache.Options{
		TTL:             options.TTL,
		CleanupInterval: options.CleanupInterval,
		Capacity:        options.Capacity,
	}, cache.LRUOptions{})
	r.fileCache.OnValueEvicted(r.onValueEvictedHandler)
	return &r
}

// Options returns a copy of options struct.
func (r *FsRepo) Options() Options {
	return r.options
}

// GetFS returns FS used by repo.
func (r *FsRepo) GetFS() afero.Fs {
	return r.fs
}

// StoragePath returns joined basepath and storeID.
func (r *FsRepo) StoragePath() string {
	return filepath.Join(r.options.BasePath, r.storeID)
}

// Store stores a file in cache.
func (r *FsRepo) Store(key string, storeKey string, src io.Reader, filename string) (string, error) {
	path := filepath.Join(r.StoragePath(), fileStorageName, key, storeKey)
	return path, r.storeFile(path, src, filename)
}

// StoreLock starts storing procedure returning lock channel and storeKey if key doesn't exist, nil and "" otherwise.
func (r *FsRepo) StoreLock(key string) (chan struct{}, string) {
	var (
		ch chan struct{}
		ok bool
	)

	r.muStoreLocks.Lock()
	ch, ok = r.storeLocks[key]
	if !ok {
		ch = make(chan struct{})
		r.storeLocks[key] = ch
		r.muStoreLocks.Unlock()
		return ch, util.GenerateKey()
	}
	r.muStoreLocks.Unlock()

	select {
	case <-ch:
		if r.Get(key) != "" {
			return nil, ""
		}
	case <-time.After(r.options.StoreLockTimeout):
		logrus.WithField("key", key).Warn("Timed out while waiting for store lock")
	}

	r.removeStoreLock(key, ch)
	return r.StoreLock(key)

}

func (r *FsRepo) removeStoreLock(key string, ch chan struct{}) {
	r.muStoreLocks.Lock()
	if r.storeLocks[key] == ch {
		delete(r.storeLocks, key)
	}
	r.muStoreLocks.Unlock()
}

// StoreUnlock finishes up storing procedure and depending on a flag - saves it or discards.
func (r *FsRepo) StoreUnlock(key, storeKey string, ch chan struct{}, save bool) {
	path := filepath.Join(r.StoragePath(), fileStorageName, key, storeKey)

	// Discard if save=false or key was set in meantime.
	if !save || !r.fileCache.Add(key, Resource{Path: path}) {
		r.fs.RemoveAll(path) // nolint - ignore error
		r.removeStoreLock(key, ch)
	}
	close(ch)
}

// PermStore stores a file in permanent storage.
func (r *FsRepo) PermStore(key string, src io.Reader, filename string) (path string, err error) {
	path = filepath.Join(r.StoragePath(), fileStorageName, key)
	if err = r.storeFile(path, src, filename); err != nil {
		return
	}
	r.muStore.Lock()
	r.permStore[key] = path
	r.muStore.Unlock()
	return
}

func (r *FsRepo) storeFile(path string, src io.Reader, filename string) error {
	// Check disk space.
	for {
		percent, _ := r.sys.GetDiskUsage(r.options.BasePath)
		if percent < r.options.MaxDiskUsage {
			break
		}
		removed := r.fileCache.DeleteLRU()
		if !removed {
			return ErrNotEnoughDiskSpace
		}
	}

	filePath := filepath.Join(path, filename)

	// Create dir.
	err := r.fs.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	if err != nil {
		return err
	}

	// Create file and write to it.
	file, err := r.fs.Create(filePath)
	if err != nil {
		return err
	}
	if _, err = io.Copy(file, src); err != nil {
		file.Close() // nolint - don't care about the error
		return err
	}
	return file.Close()
}

// Get returns path to file or empty string if it does not exist in both permanent and cache storage.
func (r *FsRepo) Get(key string) string {
	if res := r.getPermStore(key); res != "" {
		return res
	}
	if res := r.fileCache.Get(key); res != nil {
		return res.(Resource).Path
	}
	return ""
}

func (r *FsRepo) getPermStore(key string) string {
	r.muStore.RLock()
	defer r.muStore.RUnlock()
	val, ok := r.permStore[key]
	if ok {
		return val
	}
	return ""
}

// Delete removes all data related to key.
func (r *FsRepo) Delete(key string) {
	r.fileCache.Delete(key)
}

// CreateVolume creates a new volume. Returns volKey, real path to the volume and optionally an error that occurred.
func (r *FsRepo) CreateVolume() (volKey string, path string, err error) {
	r.muVol.Lock()
	defer r.muVol.Unlock()

	volKey = util.GenerateKey()
	path = filepath.Join(r.StoragePath(), volumeStorageName, volKey)
	if err = r.fs.MkdirAll(path, os.ModePerm); err != nil {
		return
	}
	r.volumes[volKey] = &Volume{Path: path}
	return
}

// DeleteVolume deletes a volume along with all links.
func (r *FsRepo) DeleteVolume(volKey string) error {
	r.muVol.Lock()
	defer r.muVol.Unlock()

	vol, ok := r.volumes[volKey]
	if !ok {
		return ErrVolumeNotFound
	}
	delete(r.volumes, volKey)
	return r.deleteVolume(vol)
}

// deleteVolume cleans up mounts and files but tries to finish cleanup even if error occurs. Returns first error.
func (r *FsRepo) deleteVolume(vol *Volume) error {
	var err error
	for _, m := range vol.mounts {
		if e := r.umount(filepath.Join(vol.Path, m)); e != nil && err == nil {
			err = e
		}
	}
	if e := r.fs.RemoveAll(vol.Path); e != nil && err == nil {
		return e
	}
	return err
}

func (r *FsRepo) umount(path string) error {
	return r.command.Run(fusermountCmd, "-u", path)
}

// CleanupVolume deletes all files/links and mounts from a volume.
func (r *FsRepo) CleanupVolume(volKey string) error {
	r.muVol.Lock()
	defer r.muVol.Unlock()

	vol, ok := r.volumes[volKey]
	if !ok {
		return ErrVolumeNotFound
	}
	if err := r.deleteVolume(vol); err != nil {
		return err
	}
	vol.mounts = []string{}
	return r.fs.MkdirAll(vol.Path, os.ModePerm)
}

// Link links a resource to a volume.
func (r *FsRepo) Link(volKey, resKey, destName string) error {
	resPath := r.Get(resKey)
	if resPath == "" {
		return ErrResourceNotFound
	}

	r.muVol.RLock()
	defer r.muVol.RUnlock()

	vol, ok := r.volumes[volKey]
	if !ok {
		return ErrVolumeNotFound
	}

	return afero.Walk(r.fs, resPath, func(path string, info os.FileInfo, err error) error {
		destRel, _ := filepath.Rel(resPath, path) // nolint - this error cannot happen as we walk the resPath.
		destPath := filepath.Join(vol.Path, destName, destRel)
		if info.IsDir() {
			return r.fs.Mkdir(destPath, os.ModePerm)
		}
		return r.fs.Link(path, destPath)
	})
}

// Mount mounts fileName to a volume dir through squashfuse.
func (r *FsRepo) Mount(volKey, resKey, fileName, destName string) error {
	resPath := r.Get(resKey)
	if resPath == "" {
		return ErrResourceNotFound
	}

	r.muVol.RLock()
	defer r.muVol.RUnlock()

	vol, ok := r.volumes[volKey]
	if !ok {
		return ErrVolumeNotFound
	}

	destPath := filepath.Join(vol.Path, destName)
	if err := r.fs.Mkdir(destPath, os.ModePerm); err != nil {
		return err
	}

	err := r.command.Run(squashfuseCmd, filepath.Join(resPath, fileName), destPath, "-o", "allow_other")
	if err == nil {
		vol.mounts = append(vol.mounts, destName)
	}
	return err
}

// CleanupUnused removes all directories from storage path that do not match our StorageID.
func (r *FsRepo) CleanupUnused() {
	r.cleanupMounts()

	files, err := afero.ReadDir(r.fs, r.options.BasePath)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if file.Name() == r.storeID {
			continue
		}
		path := filepath.Join(r.options.BasePath, file.Name())

		if err := r.fs.RemoveAll(path); err != nil {
			panic(err)
		}
	}
}

// Remove unused mounts.
func (r *FsRepo) cleanupMounts() {
	if f, err := r.fs.OpenFile("/proc/mounts", os.O_RDONLY, os.ModePerm); err != nil {
		logrus.WithError(err).Warn("/proc/mounts missing, skipping mounts cleanup")
	} else {
		defer f.Close()

		rd := bufio.NewReader(f)
		for {
			line, e := rd.ReadString('\n')
			if e == io.EOF {
				break
			}
			util.Must(e)

			p := strings.SplitN(line, " ", 3)
			if p[0] == squashfuseMount {
				util.Must(r.umount(p[1]))
			}
		}
	}
}

// Flush removes all volumes and permanent/cached resources.
func (r *FsRepo) Flush() {
	r.muStore.Lock()
	r.permStore = make(map[string]string)
	r.muStore.Unlock()

	r.muVol.Lock()
	for _, vol := range r.volumes {
		r.deleteVolume(vol) // nolint - ignore error
	}
	r.volumes = make(map[string]*Volume)
	r.muVol.Unlock()

	r.fileCache.Flush()
	if err := r.fs.RemoveAll(r.StoragePath()); err != nil {
		panic(err)
	}
}

// Shutdown removes all volumes, permanent/cached resources and stops janitor.
func (r *FsRepo) Shutdown() {
	r.fileCache.StopJanitor()
	r.Flush()
}

func (r *FsRepo) onValueEvictedHandler(key string, val interface{}) {
	r.muStoreLocks.Lock()
	delete(r.storeLocks, key)
	r.muStoreLocks.Unlock()

	path := val.(Resource).Path
	logrus.WithFields(logrus.Fields{"key": key, "path": path}).Info("FileRepo path removed")
	if err := r.fs.RemoveAll(path); err != nil {
		panic(err)
	}
}

// RelativePath returns relative to base path.
func (r *FsRepo) RelativePath(path string) (string, error) {
	return filepath.Rel(r.options.BasePath, path)
}
