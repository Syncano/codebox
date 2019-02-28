package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"

	"github.com/hashicorp/yamux"

	"github.com/Syncano/codebox/codewrapper/pkg"
)

const (
	port = 8123
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func copyStreams(dst io.WriteCloser, src io.Reader) {
	io.Copy(dst, src) // nolint: errcheck
	dst.Close()
}

func main() {
	// Get address for eth0.
	i, err := net.InterfaceByName("eth0")
	must(err)
	addrs, err := i.Addrs()
	must(err)

	var ip net.IP
	for _, addr := range addrs {
		if v, ok := addr.(*net.IPNet); ok {
			ip = v.IP
			break
		}
	}
	// Create a TCP server.
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", ip.String(), port)) // nolint: gosec
	must(err)

	// Print listening address.
	fmt.Println(lis.Addr().String())

	// Setup server side of yamux.
	conn, err := lis.Accept()
	must(err)
	session, err := yamux.Server(conn, nil)
	must(err)

	// Accept a stream for stdin/out and stderr streams.
	stdoutS, err := session.Accept()
	must(err)

	stderrS, err := session.Accept()
	must(err)

	// Read data from stream.
	r := bufio.NewReader(stdoutS)
	line, _, err := r.ReadLine()
	must(err)

	// Decode data to json.
	var s pkg.Setup
	err = json.Unmarshal(line, &s)
	must(err)

	// Start subprocess.
	var args []string
	if len(s.Command) > 1 {
		args = s.Command[1:]
	}
	subProcess := exec.Command(s.Command[0], args...) // nolint: gosec
	stdoutP, err := subProcess.StdoutPipe()
	must(err)
	stderrP, err := subProcess.StderrPipe()
	must(err)
	err = subProcess.Start()
	must(err)

	// Read app wrapper address from stdout.
	line, _, err = bufio.NewReader(stdoutP).ReadLine()
	must(err)
	appAddr := string(line)

	// Set up stream copying job.
	// Copy process stdout to server stream 1.
	go copyStreams(stdoutS, stdoutP)
	// Copy process stderr to server stream 2.
	go copyStreams(stderrS, stderrP)

	// Wait for new connections and forward them to app.
	for {
		if connS, err := session.Accept(); err == nil {
			if connP, err := net.Dial("unix", appAddr); err == nil {
				go copyStreams(connP, connS)
				go copyStreams(connS, connP)
			} else {
				connS.Close()
			}
		} else {
			return
		}
	}
}
