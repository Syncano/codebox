package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
	"sync/atomic"

	"github.com/hashicorp/yamux"
)

const (
	cacheSock = "/tmp/cache.sock"
)

type Server struct {
	lis, cacheLis         net.Listener
	stdout, stderr, cache net.Conn
	session               *yamux.Session
	running               uint32
}

func Listen(network, address string) (*Server, error) {
	s := &Server{running: 1}

	var err error

	// Create a server used for yamux (stdout, stderr and app connection).
	s.lis, err = net.Listen(network, address) // nolint: gosec
	if err != nil {
		return nil, err
	}

	// Print listening address.
	fmt.Println(address)

	// Setup server side of yamux.
	conn, err := s.lis.Accept()
	if err != nil {
		return nil, err
	}

	s.session, err = yamux.Server(conn, nil)
	if err != nil {
		return nil, err
	}

	// Accept a stream for stdin/out and stderr streams.
	s.stdout, err = s.session.Accept()
	if err != nil {
		return nil, err
	}

	s.stderr, err = s.session.Accept()
	if err != nil {
		return nil, err
	}

	// Start cache server.
	s.cacheLis, err = net.Listen("unix", cacheSock) // nolint: gosec
	if err != nil {
		return nil, err
	}

	s.cache, err = s.session.Accept()

	return s, err
}

func copyStreams(dst io.WriteCloser, src io.Reader) error {
	_, err := io.Copy(dst, src)
	dst.Close()

	return err
}

func (s *Server) StartAccepting() error {
	// Read data from stream.
	r := bufio.NewReader(s.stdout)

	line, _, err := r.ReadLine()
	if err != nil {
		return err
	}

	// Decode data to json.
	var setup Setup

	err = json.Unmarshal(line, &setup)
	if err != nil {
		return err
	}

	// Start subprocess.
	var args []string
	if len(setup.Command) > 1 {
		args = setup.Command[1:]
	}

	subProcess := exec.Command(setup.Command[0], args...) // nolint: gosec

	stdoutP, err := subProcess.StdoutPipe()
	if err != nil {
		return err
	}

	stderrP, err := subProcess.StderrPipe()
	if err != nil {
		return err
	}

	err = subProcess.Start()
	if err != nil {
		return err
	}

	// Read app wrapper address from stdout.
	line, _, err = bufio.NewReader(stdoutP).ReadLine()
	if err != nil {
		return err
	}

	appAddr := string(line)

	// Set up stream copying job.
	// Copy process stdout to server stream 1.
	go s.criticalCopyStreams(s.stdout, stdoutP)
	// Copy process stderr to server stream 2.
	go s.criticalCopyStreams(s.stderr, stderrP)

	// Start cache server async accept.
	go func() {
		conn, err := s.cacheLis.Accept()
		if err != nil {
			s.Shutdown()
			return
		}

		go s.criticalCopyStreams(conn, s.cache)
		go s.criticalCopyStreams(s.cache, conn)
	}()

	// Wait for new connections and forward them to app.
	for {
		if connS, err := s.session.Accept(); err == nil {
			if connP, err := net.Dial("unix", appAddr); err == nil {
				go copyStreams(connP, connS) // nolint: errcheck

				go copyStreams(connS, connP) // nolint: errcheck
			} else {
				connS.Close()
			}
		} else {
			return nil
		}
	}
}

func (s *Server) criticalCopyStreams(dst io.WriteCloser, src io.Reader) {
	if err := copyStreams(dst, src); err != nil {
		s.Shutdown()
	}
}

func (s *Server) setRunning(running bool) bool {
	if running {
		return atomic.SwapUint32(&s.running, 1) != 1
	}

	return atomic.SwapUint32(&s.running, 0) != 0
}

func (s *Server) IsRunning() bool {
	return atomic.LoadUint32(&s.running) == 1
}

func (s *Server) Shutdown() {
	if !s.setRunning(false) {
		return
	}

	s.session.Close()
	s.lis.Close()
	s.cacheLis.Close()
}
