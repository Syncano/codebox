package script

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/hashicorp/yamux"
	"github.com/juju/ratelimit"

	codewrapper "github.com/Syncano/codebox/codewrapper/server"
	"github.com/Syncano/pkg-go/v2/util"
)

var (
	// ErrTooManyConnections is an error signaling that there are too many simultaneous connections to container.
	ErrTooManyConnections = errors.New("too many simultaneous connections")
	// ErrConnectionIDReserved is an error signaling that connection with specified ID was already reserved.
	ErrConnectionIDReserved = errors.New("connection already reserved")
)

const (
	ContainerStateFresh = iota
	ContainerStateRunning
	ContainerStateStopping
	ContainerStateStopped

	containerLogFormat = "stream:%s:%s:log"
)

// Container defines unique container information.
type Container struct {
	*Definition

	ID string

	state     uint32
	conns     map[string]struct{}
	connLimit int
	calls     uint64

	redisCli              RedisClient
	mu                    sync.Mutex
	resp                  types.HijackedResponse
	session               YamuxSession
	stdout, stderr, cache io.ReadWriteCloser
	addr                  string
	volumeKey             string

	opts                 *ContainerOptions
	bucket               *ratelimit.Bucket
	userCache            *UserCache
	userCacheBucket      *ratelimit.Bucket
	userCacheConstraints *UserCacheConstraints
}

type ContainerOptions struct {
	Timeout time.Duration
}

func (c *Container) String() string {
	return fmt.Sprintf("{ID:%s, Calls:%d, VolumeKey:%s}", c.ID, c.Calls(), c.volumeKey)
}

// NewContainer creates new container and returns it.
func NewContainer(runtime string, redisCli RedisClient) *Container {
	return &Container{
		Definition: &Definition{
			Index: &Index{Runtime: runtime},
		},
		conns:    make(map[string]struct{}),
		redisCli: redisCli,
	}
}

func (c *Container) Calls() uint64 {
	return atomic.LoadUint64(&c.calls)
}

func (c *Container) IncreaseCalls() {
	atomic.AddUint64(&c.calls, 1)
}

func (c *Container) conn() (io.ReadWriteCloser, error) {
	return c.session.Open()
}

// Conn returns container conn.
func (c *Container) Conn() (io.ReadWriteCloser, error) {
	c.mu.Lock()
	conn, err := c.conn()
	c.mu.Unlock()

	return conn, err
}

// Stdout returns container stdout stream.
func (c *Container) Stdout() io.ReadWriter {
	return c.stdout
}

func (c *Container) StdoutRateLimited() io.ReadWriter {
	return util.NewRateLimitedReadWriter(c.stdout, c.bucket)
}

// Stderr returns container stderr stream.
func (c *Container) Stderr() io.ReadWriteCloser {
	return c.stderr
}

func (c *Container) StderrRateLimited() io.ReadWriter {
	return util.NewRateLimitedReadWriter(c.stderr, c.bucket)
}

// Cache returns container cache stream.
func (c *Container) Cache() io.ReadWriter {
	return c.cache
}

func (c *Container) CacheRateLimited() io.ReadWriter {
	return util.NewRateLimitedReadWriter(c.cache, c.userCacheBucket)
}

func (c *Container) setupCodewrapper(conn io.Writer, setup *codewrapper.Setup) error {
	// Send codewrapper setup.
	data, err := json.Marshal(setup)
	if err != nil {
		return err
	}

	for _, data := range [][]byte{data, {'\n'}} {
		if _, err = conn.Write(data); err != nil {
			return err
		}
	}

	return nil
}

func (c *Container) setupSocket(conn io.Writer) error {
	// Send socket wrapper setup.
	setup := scriptSetup{
		Async:      c.Async,
		EntryPoint: c.Entrypoint,
		Timeout:    c.opts.Timeout,
	}
	setupBytes, err := json.Marshal(setup)

	util.Must(err)

	err = binary.Write(conn, binary.LittleEndian, uint32(len(setupBytes)+4))
	if err != nil {
		return err
	}

	_, err = conn.Write(setupBytes)

	return err
}

// Configure sets up internal container settings.
func (c *Container) Configure(options *ContainerOptions, userCacheConstraints *UserCacheConstraints, bucket, userCacheBucket *ratelimit.Bucket) {
	c.opts = options
	c.bucket = bucket
	c.userCacheBucket = userCacheBucket
	c.userCacheConstraints = userCacheConstraints

	c.connLimit = int(c.Async)
	if c.connLimit == 0 {
		c.connLimit = 1
	}
}

// Setup sends setup packet to container.
func (c *Container) SendSetup(setup *codewrapper.Setup) (io.ReadWriteCloser, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session == nil {
		conn, err := net.DialTimeout("tcp", c.addr, dockerTimeout)
		if err != nil {
			return nil, err
		}

		if c.session, err = yamux.Client(conn, nil); err != nil {
			return nil, err
		}

		if c.stdout, err = c.session.Open(); err != nil {
			return nil, err
		}

		if c.stderr, err = c.session.Open(); err != nil {
			return nil, err
		}

		if c.cache, err = c.session.Open(); err != nil {
			return nil, err
		}

		c.userCache = NewUserCache(c.UserID, c.CacheRateLimited(), c.redisCli, c.userCacheConstraints)
	}

	// Setup codewrapper and socket connection.
	if err := c.setupCodewrapper(c.stdout, setup); err != nil {
		return nil, err
	}

	conn, err := c.conn()
	if err != nil {
		return nil, err
	}

	if err = c.setupSocket(conn); err != nil {
		conn.Close()
	}

	atomic.CompareAndSwapUint32(&c.state, ContainerStateFresh, ContainerStateRunning)

	return conn, err
}

func (c *Container) ProcessCache() error {
	return c.userCache.Process()
}

// Run sends run packet to container.
func (c *Container) Run(conn io.Writer, options *RunOptions) (string, error) {
	// Prepare files for context.
	var filesSize int

	files := make([]contextFile, 0, len(options.Files))

	for f, data := range options.Files {
		flen := len(data.Data)

		files = append(files, contextFile{
			Name:        f,
			Filename:    data.Filename,
			ContentType: data.ContentType,
			Length:      flen,
		})
		filesSize += flen
	}

	// Prepare context for container.
	context := scriptContext{
		Delim:  util.GenerateKey(),
		Args:   (*json.RawMessage)(&options.Args),
		Meta:   (*json.RawMessage)(&options.Meta),
		Config: (*json.RawMessage)(&options.Config),
		Files:  files,
	}
	scriptContextBytes, err := json.Marshal(context)

	util.Must(err)

	contextSize := len(scriptContextBytes)

	// Send context.
	c.mu.Lock()
	defer c.mu.Unlock()

	err = binary.Write(conn, binary.LittleEndian, uint32(contextSize+filesSize+8))
	if err != nil {
		return "", err
	}

	err = binary.Write(conn, binary.LittleEndian, uint32(contextSize))
	if err != nil {
		return "", err
	}

	_, err = conn.Write(scriptContextBytes)
	if err != nil {
		return "", err
	}

	// Now send files for context.
	for _, f := range files {
		if _, err = conn.Write(options.Files[f.Name].Data); err != nil {
			return "", err
		}
	}

	return context.Delim, nil
}

// IsAcceptingConnections returns true of container is accepting connections.
func (c *Container) IsAcceptingConnections() bool {
	return atomic.LoadUint32(&c.state) == ContainerStateRunning
}

// StopAcceptingConnections sets container to stop accepting connections.
func (c *Container) StopAcceptingConnections() {
	if !atomic.CompareAndSwapUint32(&c.state, ContainerStateRunning, ContainerStateStopping) {
		atomic.CompareAndSwapUint32(&c.state, ContainerStateFresh, ContainerStateStopping)
	}
}

// ConnsNum returns number of connections currently in container. Unsafe concurrently without lock.
func (c *Container) ConnsNum() int {
	c.mu.Lock()
	l := len(c.conns)
	c.mu.Unlock()

	return l
}

// Reserve checks if it is possible to reserve new connection and increases semaphore unit.
func (c *Container) Reserve(connID string, success func(numConns int) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conns == nil || len(c.conns) >= c.connLimit {
		return ErrTooManyConnections
	}

	if _, ok := c.conns[connID]; ok {
		return ErrConnectionIDReserved
	}

	c.conns[connID] = struct{}{}

	if success == nil {
		return nil
	}

	return success(len(c.conns))
}

// Release decreases one semaphore unit. Unsafe concurrently without lock.
func (c *Container) Release(connID string, success func(numConns int) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.conns, connID)

	if success == nil {
		return nil
	}

	return success(len(c.conns))
}

// Stop closes session if it is opened.
func (c *Container) Stop() bool {
	c.StopAcceptingConnections()

	if !atomic.CompareAndSwapUint32(&c.state, ContainerStateStopping, ContainerStateStopped) {
		return false
	}

	c.mu.Lock()

	if c.session != nil {
		c.session.Close()

		c.session = nil
	}

	c.mu.Unlock()

	return true
}

func (c *Container) LogChannel() string {
	return fmt.Sprintf(containerLogFormat, c.UserID, c.SourceHash)
}
