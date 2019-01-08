package filerepo

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"

	"github.com/Syncano/codebox/pkg/sys/mocks"
)

func verifyFile(filepath, content string) {
	stat, _ := os.Stat(filepath)
	So(stat.IsDir(), ShouldBeFalse)
	f, _ := ioutil.ReadFile(filepath)
	So(string(f), ShouldEqual, content)
}

func TestRepo(t *testing.T) {
	Convey("Given file repo with mocked system checker", t, func() {
		sc := new(mocks.SystemChecker)
		dir, _ := ioutil.TempDir("", "test")
		opts := Options{CleanupInterval: time.Minute, Capacity: 12, TTL: time.Second, BasePath: dir}
		repo := New(opts, sc, new(LinkFs), new(Command))

		Convey("related options are properly set up", func() {
			cacheOpts := repo.fileCache.Options()

			So(cacheOpts.CleanupInterval, ShouldEqual, opts.CleanupInterval)
			So(cacheOpts.Capacity, ShouldEqual, opts.Capacity)
			So(cacheOpts.TTL, ShouldEqual, opts.TTL)

			So(repo.storeID, ShouldNotBeBlank)
			storagePath := repo.StoragePath()
			So(storagePath, ShouldStartWith, dir)
			So(storagePath, ShouldEndWith, repo.storeID)
		})

		Convey("given successful disk check", func() {
			sc.On("GetDiskUsage", dir).Return(float64(0), uint64(math.MaxUint64))
			content := "abc"

			Convey("Store creates file(s) on successful disk check and entry in cache", func() {
				lockCh, storeKey := repo.StoreLock("key")
				So(lockCh, ShouldNotBeNil)
				path, e := repo.Store("key", storeKey, bytes.NewReader([]byte(content)), "name1")
				So(path, ShouldStartWith, dir)
				So(e, ShouldBeNil)
				stat, _ := os.Stat(path)
				So(stat.IsDir(), ShouldBeTrue)
				_, e = repo.Store("key", storeKey, bytes.NewReader([]byte(content)), "name2")
				So(e, ShouldBeNil)
				repo.StoreUnlock("key", storeKey, lockCh, true)

				// Verify that correct files exists.
				filepath1 := filepath.Join(path, "name1")
				filepath2 := filepath.Join(path, "name2")
				verifyFile(filepath1, content)
				verifyFile(filepath2, content)

				// Verify that entry is created in cache.
				ok := repo.fileCache.Get("key")
				So(ok, ShouldNotBeNil)

				// Verify that repo.Get() properly returns correct path.
				So(repo.Get("key"), ShouldEqual, path)

				Convey("which are deleted on shutdown", func() {
					repo.Shutdown()
					_, e = os.Stat(filepath1)
					So(os.IsNotExist(e), ShouldBeTrue)
					_, e = os.Stat(filepath2)
					So(os.IsNotExist(e), ShouldBeTrue)
				})
				Convey("which are deleted by calling Delete", func() {
					So(len(repo.storeLocks), ShouldEqual, 1)
					repo.Delete("key")

					So(len(repo.storeLocks), ShouldEqual, 0)
					_, e = os.Stat(filepath1)
					So(os.IsNotExist(e), ShouldBeTrue)
					_, e = os.Stat(filepath2)
					So(os.IsNotExist(e), ShouldBeTrue)
				})
			})
			Convey("PermStore creates file(s) on successful disk check and entry in internal map (no ttl)", func() {
				path, e := repo.PermStore("key", bytes.NewReader([]byte(content)), "name1")
				So(path, ShouldStartWith, dir)
				So(e, ShouldBeNil)
				stat, _ := os.Stat(path)
				So(stat.IsDir(), ShouldBeTrue)
				_, e = repo.PermStore("key", bytes.NewReader([]byte(content)), "name2")
				So(e, ShouldBeNil)

				// Verify that correct files exists.
				filepath1 := filepath.Join(path, "name1")
				filepath2 := filepath.Join(path, "name2")
				verifyFile(filepath1, content)
				verifyFile(filepath2, content)

				// Verify that entry is created in internal map.
				_, ok := repo.permStore["key"]
				So(ok, ShouldBeTrue)

				// Verify that repo.Get() properly returns correct path.
				So(repo.Get("key"), ShouldEqual, path)

				Convey("which are deleted on shutdown", func() {
					repo.Shutdown()
					_, e = os.Stat(filepath1)
					So(os.IsNotExist(e), ShouldBeTrue)
					_, e = os.Stat(filepath2)
					So(os.IsNotExist(e), ShouldBeTrue)
				})
			})
			Convey("StoreUnlock with save=false discards the file", func() {
				lockCh, storeKey := repo.StoreLock("key")
				path, _ := repo.Store("key", storeKey, bytes.NewReader([]byte(content)), "name")
				repo.StoreUnlock("key", storeKey, lockCh, false)
				_, e := os.Stat(path)
				So(os.IsNotExist(e), ShouldBeTrue)
			})
			Convey("StoreUnlock discards files if key was set in meantime", func() {
				lockCh, storeKey := repo.StoreLock("key")
				path, _ := repo.Store("key", storeKey, bytes.NewReader([]byte("abc")), "name")
				repo.fileCache.Set("key", Resource{Path: "path"})
				repo.StoreUnlock("key", storeKey, lockCh, true)
				_, e := os.Stat(path)
				So(os.IsNotExist(e), ShouldBeTrue)
			})
		})

		Convey("StoreLock waits until StoreUnlock is called", func() {
			sleepTime := 5 * time.Millisecond
			lockCh, storeKey := repo.StoreLock("key")
			So(lockCh, ShouldNotBeNil)
			t := time.Now()
			go func() {
				time.Sleep(sleepTime)
				repo.StoreUnlock("key", storeKey, lockCh, false)
			}()

			lockCh, _ = repo.StoreLock("key")
			So(lockCh, ShouldNotBeNil)
			So(time.Since(t), ShouldBeGreaterThan, sleepTime)
		})
		Convey("StoreLock times out after StoreLockTimeout and retries", func() {
			sleepTime := 5 * time.Millisecond
			repo.options.StoreLockTimeout = sleepTime
			lockCh, _ := repo.StoreLock("key")
			So(lockCh, ShouldNotBeNil)
			t := time.Now()
			lockCh, _ = repo.StoreLock("key")
			So(lockCh, ShouldNotBeNil)
			So(time.Since(t), ShouldBeGreaterThan, sleepTime)
		})
		Convey("StoreLock returns nil if file was saved", func() {
			sleepTime := 5 * time.Millisecond
			lockCh, storeKey := repo.StoreLock("key")
			So(lockCh, ShouldNotBeNil)
			t := time.Now()
			go func() {
				time.Sleep(sleepTime)
				repo.StoreUnlock("key", storeKey, lockCh, true)
			}()
			lockCh, _ = repo.StoreLock("key")
			So(lockCh, ShouldBeNil)
			So(time.Since(t), ShouldBeGreaterThan, sleepTime)
		})

		Convey("given full disk check", func() {
			sc.On("GetDiskUsage", dir).Return(float64(100), uint64(0))

			Convey("Store propagates storeFile error", func() {
				_, e := repo.Store("key", "storeKey", bytes.NewReader([]byte("abc")), "name")
				So(e, ShouldEqual, ErrNotEnoughDiskSpace)
			})
			Convey("PermStore propagates storeFile error", func() {
				_, e := repo.PermStore("key", bytes.NewReader([]byte("abc")), "name")
				So(e, ShouldEqual, ErrNotEnoughDiskSpace)
			})
		})

		Convey("storeFile tries to delete LRU until satisfied", func() {
			sc.On("GetDiskUsage", dir).Return(float64(100), uint64(math.MaxUint64)).Times(2)
			sc.On("GetDiskUsage", dir).Return(float64(0), uint64(0))

			eviction := 0
			repo.permStore["perm1"] = "val1"
			repo.fileCache.Set("cache1", "val1")
			repo.fileCache.Set("cache2", "val2")
			repo.fileCache.Set("cache3", "val3")
			repo.fileCache.OnValueEvicted(func(string, interface{}) { eviction++ })

			// Verify that from cache 2 firstly added items are deleted while perm store remains untouched.
			path := filepath.Join(repo.StoragePath(), fileStorageName, "name")
			e := repo.storeFile(path, bytes.NewReader([]byte("abc")), "name")
			So(e, ShouldBeNil)
			So(eviction, ShouldEqual, 2)
			So(repo.fileCache.Len(), ShouldEqual, 1)
			So(len(repo.permStore), ShouldEqual, 1)
			So(repo.fileCache.Get("cache3"), ShouldNotBeBlank)
		})

		Convey("Get returns empty string if key is not found", func() {
			path := repo.Get("key")
			So(path, ShouldBeBlank)
		})

		Convey("CreateVolume creates a random directory", func() {
			volKey1, volPath1, err1 := repo.CreateVolume()
			volKey2, volPath2, err2 := repo.CreateVolume()
			So(err1, ShouldBeNil)
			So(err2, ShouldBeNil)
			So(volKey1, ShouldNotEqual, volKey2)
			So(volPath1, ShouldNotEqual, volPath2)
			stat, _ := os.Stat(volPath1)
			So(stat.IsDir(), ShouldBeTrue)
			filePath := filepath.Join(volPath1, "file1")
			_, e := os.Create(filePath)
			So(e, ShouldBeNil)

			Convey("which is cleaned up by CleanupVolume", func() {
				e := repo.CleanupVolume(volKey1)
				So(e, ShouldBeNil)
				stat, _ = os.Stat(volPath1)
				So(stat.IsDir(), ShouldBeTrue)
				_, e = os.Stat(filePath)
				So(os.IsNotExist(e), ShouldBeTrue)
			})
			Convey("which is deleted by DeleteVolume", func() {
				e := repo.DeleteVolume(volKey1)
				So(e, ShouldBeNil)
				_, e = os.Stat(volPath1)
				So(os.IsNotExist(e), ShouldBeTrue)
				stat, _ := os.Stat(volPath2)
				So(stat.IsDir(), ShouldBeTrue)
			})
			Convey("which is deleted on shutdown", func() {
				repo.Shutdown()
				_, e := os.Stat(volPath1)
				So(os.IsNotExist(e), ShouldBeTrue)
				_, e = os.Stat(volPath2)
				So(os.IsNotExist(e), ShouldBeTrue)
			})
		})
		Convey("DeleteVolume returns error when volume is not found", func() {
			e := repo.DeleteVolume("key")
			So(e, ShouldEqual, ErrVolumeNotFound)
		})
		Convey("CleanupVolume returns error when volume is not found", func() {
			e := repo.CleanupVolume("key")
			So(e, ShouldEqual, ErrVolumeNotFound)
		})

		Convey("Link returns error when resource is not found", func() {
			e := repo.Link("volkey", "reskey", "dest")
			So(e, ShouldEqual, ErrResourceNotFound)
		})
		Convey("Link returns error when volume is not found", func() {
			sc.On("GetDiskUsage", dir).Return(float64(0), uint64(math.MaxUint64))
			lockCh, storeKey := repo.StoreLock("reskey")
			repo.Store("reskey", storeKey, bytes.NewReader([]byte("abc")), "name")
			repo.StoreUnlock("reskey", storeKey, lockCh, true)
			e := repo.Link("volkey", "reskey", "dest")
			So(e, ShouldEqual, ErrVolumeNotFound)
		})
		Convey("Link creates a link inside volume to resource", func() {
			sc.On("GetDiskUsage", dir).Return(float64(0), uint64(math.MaxUint64))
			lockCh, storeKey := repo.StoreLock("reskey")
			repo.Store("reskey", storeKey, bytes.NewReader([]byte("abc")), "name")
			repo.StoreUnlock("reskey", storeKey, lockCh, true)
			volKey, path, e := repo.CreateVolume()
			So(e, ShouldBeNil)
			e = repo.Link(volKey, "reskey", "dest")
			So(e, ShouldBeNil)

			// Verify dir and file existance.
			stat, _ := os.Stat(filepath.Join(path, "dest"))
			So(stat.IsDir(), ShouldBeTrue)
			stat, _ = os.Stat(filepath.Join(path, "dest", "name"))
			So(stat.IsDir(), ShouldBeFalse)
		})

		Convey("Mount returns error when resource is not found", func() {
			e := repo.Mount("volkey", "reskey", "file", "dest")
			So(e, ShouldEqual, ErrResourceNotFound)
		})
		Convey("Mount returns error when volume is not found", func() {
			repo.fileCache.Set("reskey", Resource{Path: "path"})
			e := repo.Mount("volkey", "reskey", "file", "dest")
			So(e, ShouldEqual, ErrVolumeNotFound)
		})

		Convey("CleanupUnused deletes all directories in storage except one equal to storeID", func() {
			os.Mkdir(filepath.Join(dir, "randomdir"), os.ModePerm)
			os.Mkdir(filepath.Join(dir, repo.storeID), os.ModePerm)
			repo.CleanupUnused()
			files, _ := ioutil.ReadDir(dir)
			So(len(files), ShouldEqual, 1)
			So(files[0].Name(), ShouldEqual, repo.storeID)
		})

		Convey("Options returns a copy of options struct", func() {
			So(repo.Options(), ShouldNotEqual, repo.options)
			So(repo.Options(), ShouldResemble, repo.options)
		})

		Convey("RelativePath returns path relative to BasePath", func() {
			repo.options.BasePath = "/var"
			relPath, e := repo.RelativePath("/var/tmp/log")
			So(relPath, ShouldEqual, "tmp/log")
			So(e, ShouldBeNil)
		})

		Convey("GetFS returns nested FS", func() {
			fs := repo.GetFS()
			So(fs, ShouldEqual, repo.fs)
		})

		sc.AssertExpectations(t)
		repo.Shutdown()
		os.RemoveAll(dir)
	})
}

type mockReader struct {
	err error
}

func (mr *mockReader) Read(p []byte) (n int, err error) {
	return 0, mr.err
}

func TestRepoWithMocks(t *testing.T) {
	Convey("Given file repo with mocked system checker, fs and commander", t, func() {
		sc := new(mocks.SystemChecker)
		fs := new(MockFs)
		commander := new(MockCommander)
		dir, _ := ioutil.TempDir("", "test")
		opts := Options{CleanupInterval: time.Minute, Capacity: 12, TTL: time.Second, BasePath: dir}
		fs.On("MkdirAll", dir, os.ModePerm).Return(nil)
		repo := New(opts, sc, fs, commander)
		err := errors.New("some err")

		Convey("storeFile propagates Create error", func() {
			sc.On("GetDiskUsage", dir).Return(float64(0), uint64(math.MaxUint64))
			fs.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)
			fs.On("Create", mock.Anything).Return(&os.File{}, err)

			e := repo.storeFile("path", nil, "name")
			So(e, ShouldEqual, err)
		})
		Convey("storeFile propagates Read error", func() {
			file, _ := ioutil.TempFile("", "")
			sc.On("GetDiskUsage", dir).Return(float64(0), uint64(math.MaxUint64))
			fs.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)
			fs.On("Create", mock.Anything).Return(file, nil)

			e := repo.storeFile("path", &mockReader{err}, "name")
			So(e, ShouldEqual, err)

			file.Close()
			os.Remove(file.Name())
		})
		Convey("storeFile propagates Close error", func() {
			file, _ := ioutil.TempFile("", "")
			file.Close()

			sc.On("GetDiskUsage", dir).Return(float64(0), uint64(math.MaxUint64))
			fs.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)
			fs.On("Create", mock.Anything).Return(file, nil)

			e := repo.storeFile("path", &mockReader{io.EOF}, "name")
			So(e, ShouldNotBeNil)
			So(e, ShouldResemble, file.Close())

			os.Remove(file.Name())
		})

		Convey("given Mkdirall returning error", func() {
			fs.On("MkdirAll", mock.Anything, mock.Anything).Return(err)

			Convey("CreateVolume propagates error", func() {
				_, _, e := repo.CreateVolume()
				So(e, ShouldEqual, err)
			})
			Convey("CleanupVolume propagates error", func() {
				fs.On("RemoveAll", mock.Anything).Return(nil)
				repo.volumes["volKey"] = &Volume{}
				e := repo.CleanupVolume("volKey")
				So(e, ShouldEqual, err)
			})
			Convey("storeFile propagates error", func() {
				sc.On("GetDiskUsage", dir).Return(float64(0), uint64(math.MaxUint64))
				e := repo.storeFile("path", nil, "name")
				So(e, ShouldEqual, err)
			})
		})

		Convey("given Open returning error", func() {
			fs.On("OpenFile", "/proc/mounts", os.O_RDONLY, os.ModePerm).Return(nil, err)
			fs.On("Open", mock.Anything).Return(nil, err)

			Convey("CleanupUnused panics", func() {
				So(repo.CleanupUnused, ShouldPanicWith, err)
			})
		})

		Convey("given RemoveAll returning error", func() {
			fs.On("RemoveAll", mock.Anything).Return(err)

			Convey("CleanupUnused panics on error", func() {
				dir, _ := ioutil.TempDir("", "test")
				dirHandle, _ := os.Open(dir)

				fs.On("OpenFile", "/proc/mounts", os.O_RDONLY, os.ModePerm).Return(nil, err)
				fs.On("Open", mock.Anything).Return(dirHandle, nil)
				os.Mkdir(filepath.Join(dir, "randomdir"), os.ModePerm)
				So(repo.CleanupUnused, ShouldPanicWith, err)

				dirHandle.Close()
				os.RemoveAll(dir)
			})
			Convey("CleanupVolume propagates error", func() {
				repo.volumes["volKey"] = &Volume{}
				e := repo.CleanupVolume("volKey")
				So(e, ShouldEqual, err)
			})
			Convey("Shutdown panics", func() {
				So(repo.Shutdown, ShouldPanicWith, err)
			})
			Convey("onValueEvictedHandler panics", func() {
				So(func() { repo.onValueEvictedHandler("key", Resource{Path: "val"}) }, ShouldPanicWith, err)
			})
		})

		Convey("deleteVolume calls unmount on all mounts", func() {
			fs.On("RemoveAll", mock.Anything).Return(nil)
			commander.On("Run", "fusermount", "-u", mock.Anything).Return(nil).Twice()
			e := repo.deleteVolume(&Volume{mounts: []string{"abc", "cba"}})
			So(e, ShouldBeNil)
		})
		Convey("deleteVolume propagates unmount error", func() {
			fs.On("RemoveAll", mock.Anything).Return(nil)
			commander.On("Run", "fusermount", "-u", mock.Anything).Return(err)
			e := repo.deleteVolume(&Volume{mounts: []string{"abc"}})
			So(e, ShouldEqual, err)
		})

		Convey("given existing volume and resource", func() {
			repo.fileCache.Set("reskey", Resource{Path: "path"})
			repo.volumes["volkey"] = &Volume{}

			Convey("propagates Mkdir error", func() {
				fs.On("Mkdir", mock.Anything, mock.Anything).Return(err)
				e := repo.Mount("volkey", "reskey", "file", "dest")
				So(e, ShouldEqual, err)
			})
			Convey("given successful Mkdir", func() {
				fs.On("Mkdir", mock.Anything, mock.Anything).Return(nil)

				Convey("propagates Run error", func() {
					commander.On("Run", "squashfuse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(err)
					e := repo.Mount("volkey", "reskey", "file", "dest")
					So(e, ShouldEqual, err)
				})
				Convey("given successful Run", func() {
					commander.On("Run", "squashfuse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

					Convey("sets up volume mounts", func() {
						e := repo.Mount("volkey", "reskey", "file", "dest")
						So(e, ShouldBeNil)
						So(repo.volumes["volkey"].mounts, ShouldHaveLength, 1)
					})
				})
			})
		})

		Convey("cleanupMounts stops if cannot open proc mounts", func() {
			fs.On("OpenFile", "/proc/mounts", os.O_RDONLY, os.ModePerm).Return(nil, err)
			repo.cleanupMounts()
		})
		Convey("cleanupMounts calls unmount on all squashfuse mount", func() {
			file, _ := ioutil.TempFile("", "")
			file.WriteString(
				`proc /proc/asound proc ro,relatime 0 0
tmpfs /proc/acpi tmpfs ro,relatime 0 0
squashfuse /tmp/storage/abc fuse.squashfuse rw,nosuid,nodev,relatime,user_id=0,group_id=0 0 0
`)
			file.Seek(0, 0)

			fs.On("OpenFile", "/proc/mounts", os.O_RDONLY, os.ModePerm).Return(file, nil)
			commander.On("Run", "fusermount", "-u", mock.Anything).Return(nil).Once()
			repo.cleanupMounts()

			file.Close()
			os.Remove(file.Name())
		})

		mock.AssertExpectationsForObjects(t, sc, fs)
	})
}
