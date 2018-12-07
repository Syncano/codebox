package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
	"sync"

	"github.com/hashicorp/yamux"
)

var (
	wg sync.WaitGroup
)

// Data defines input data for wrapper.
type Data struct {
	Command string   `json:"c"`
	Args    []string `json:"a"`
	Delim   []byte   `json:"m"`
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func copyStreams(dst io.Writer, src io.Reader) {
	wg.Add(1)

	go func() {
		buf := make([]byte, 32*1024)

		for {
			if _, err := io.CopyBuffer(dst, src, buf); err != nil {
				wg.Done()
				return
			}
		}
	}()
}

func main() {
	// Create a TCP server.
	lis, err := net.Listen("tcp", ":0") // nolint: gosec
	must(err)

	port := lis.Addr().(*net.TCPAddr).Port

	// Print port.
	fmt.Println(port)

	// Setup server side of yamux.
	conn, err := lis.Accept()
	must(err)
	session, err := yamux.Server(conn, nil)
	must(err)

	// Accept a stream for stdin/out and stderr streams.
	stdinoutS, err := session.Accept()
	must(err)

	stderrS, err := session.Accept()
	must(err)

	// Read data from stream.
	stdinReader := bufio.NewReader(stdinoutS)
	line, _, err := stdinReader.ReadLine()
	must(err)

	// Decode data to json.
	var d Data
	err = json.Unmarshal(line, &d)
	must(err)

	// Start subprocess.
	subProcess := exec.Command(d.Command, d.Args...) // nolint: gosec
	stdoutP, err := subProcess.StdoutPipe()
	must(err)
	stderrP, err := subProcess.StderrPipe()
	must(err)
	stdinP, err := subProcess.StdinPipe()
	must(err)
	err = subProcess.Start()
	must(err)

	// Set up stream copying job.
	// Copy server stream 1 to process stdin.
	copyStreams(stdinP, stdinReader)
	// Copy process stdout to server stream 1.
	copyStreams(stdinoutS, stdoutP)
	// Copy process stderr to server stream 2.
	copyStreams(stderrS, stderrP)

	wg.Wait()
}
