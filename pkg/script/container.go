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

	"github.com/docker/docker/api/types"
	"github.com/hashicorp/yamux"

	"github.com/Syncano/codebox/codewrapper/pkg"
	"github.com/Syncano/codebox/pkg/docker"
	"github.com/Syncano/codebox/pkg/util"
)

var (
	// ErrTooManyConnections is an error signaling that there are too many simultaneous connections to container.
	ErrTooManyConnections = errors.New("too many simultaneous connections")
	// ErrConnectionNotFound is an error signaling that connection with specified ID was not found.
	ErrConnectionNotFound = errors.New("connection not found")
)

// Container defines unique container information.
type Container struct {
	Hash        string
	ID          string
	SourceHash  string
	Environment string
	UserID      string
	Runtime     string

	ok        uint32
	conns     map[string]struct{}
	connLimit int

	mu        sync.Mutex
	resp      types.HijackedResponse
	session   YamuxSession
	stdout    io.ReadWriteCloser
	stderr    io.ReadWriteCloser
	addr      string
	volumeKey string
}

func (c *Container) String() string {
	return fmt.Sprintf("{ID:%s, VolumeKey:%s}", c.ID, c.volumeKey)
}

// NewContainer creates new container and returns it.
func NewContainer(runtime string) *Container {
	return &Container{Runtime: runtime, ok: 1, conns: make(map[string]struct{})}
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
func (c *Container) Stdout() io.ReadWriteCloser {
	return c.stdout
}

// Stderr returns container stderr stream.
func (c *Container) Stderr() io.ReadWriteCloser {
	return c.stderr
}

func (c *Container) setupCodewrapper(conn io.Writer, constraints *docker.Constraints) error {
	// Send codewrapper setup.
	s := pkg.Setup{
		Command: SupportedRuntimes[c.Runtime].Command(constraints),
	}

	data, err := json.Marshal(s)
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

func (c *Container) setupSocket(conn io.Writer, options *RunOptions) error {
	// Send socket wrapper setup.
	setup := scriptSetup{
		Async:      options.Async > 1,
		EntryPoint: options.EntryPoint,
		Timeout:    options.Timeout,
	}
	setupBytes, err := json.Marshal(setup)

	util.Must(err)

	setupSize := len(setupBytes)

	totalLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(totalLen, uint32(setupSize+4))

	for _, data := range [][]byte{totalLen, setupBytes} {
		if _, err = conn.Write(data); err != nil {
			return err
		}
	}

	return nil
}

// Configure sets up internal container settings.
func (c *Container) Configure(options *RunOptions) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Setup container properties.
	c.connLimit = int(options.Async)
	if c.connLimit == 0 {
		c.connLimit = 1
	}
}

// Setup sends setup packet to container.
func (c *Container) Setup(options *RunOptions, constraints *docker.Constraints) (io.ReadWriteCloser, error) {
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
	}

	if err := c.setupCodewrapper(c.stdout, constraints); err != nil {
		return nil, err
	}

	conn, err := c.conn()
	if err != nil {
		return nil, err
	}

	if err = c.setupSocket(conn, options); err != nil {
		conn.Close()
	}

	return conn, err
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
	totalLen := make([]byte, 4)
	contextLen := make([]byte, 4)

	binary.LittleEndian.PutUint32(totalLen, uint32(contextSize+filesSize+8))
	binary.LittleEndian.PutUint32(contextLen, uint32(contextSize))

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, data := range [][]byte{totalLen, contextLen, scriptContextBytes} {
		if _, err = conn.Write(data); err != nil {
			return "", err
		}
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
	return atomic.LoadUint32(&c.ok) == 1
}

// StopAcceptingConnections sets container to stop accepting connections.
func (c *Container) StopAcceptingConnections() {
	atomic.StoreUint32(&c.ok, 0)
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
		return ErrConnectionNotFound
	}

	c.conns[connID] = struct{}{}

	return success(len(c.conns))
}

// Release decreases one semaphore unit. Unsafe concurrently without lock.
func (c *Container) Release(connID string, success func(numConns int) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.conns, connID)

	return success(len(c.conns))
}

// Stop closes session if it is opened.
func (c *Container) Stop() {
	c.StopAcceptingConnections()
	c.mu.Lock()
	if c.session != nil {
		c.session.Close()
	}
	c.mu.Unlock()
}
