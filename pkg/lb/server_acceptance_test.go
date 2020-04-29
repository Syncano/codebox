package lb_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/grpc"

	"github.com/Syncano/codebox/pkg/docker"
	repopb "github.com/Syncano/codebox/pkg/filerepo/proto"
	lbpb "github.com/Syncano/codebox/pkg/lb/proto"
	"github.com/Syncano/codebox/pkg/script"
	scriptpb "github.com/Syncano/codebox/pkg/script/proto"
)

func readURL(url string) ([]byte, error) {
	r, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer r.Body.Close()
	return ioutil.ReadAll(r.Body)
}

func TestLBAcceptance(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Go to root dir.
	os.Chdir("../..")

	Convey("Given initialized lb and worker", t, func() {
		// Start load balancer.
		lbCmd := exec.Command("build/codebox", "--debug", "--metric-port", "10080", "lb", "-p", "10000")
		lbCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		lbStderr, _ := lbCmd.StderrPipe()
		lbCmd.Start()

		// Start worker.
		workerCmd := exec.Command("build/codebox", "--debug", "--metric-port", "10180", "worker", "--lb", "127.0.0.1:10000")
		workerCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		wStderr, _ := workerCmd.StderrPipe()
		workerCmd.Start()

		// Wait for worker to connect.
		scanner := bufio.NewScanner(lbStderr)
		for scanner.Scan() {
			l := scanner.Text()
			if strings.Contains(l, "grpc:lb:Register") {
				break
			}
		}

		conn, err := grpc.Dial("localhost:10000", grpc.WithInsecure(), grpc.WithBlock())
		So(err, ShouldBeNil)
		defer conn.Close()

		Convey("given uploaded scripts", func() {
			// Check if Exists returns false for non-existing key.
			repoClient := repopb.NewRepoClient(conn)
			r, err := repoClient.Exists(context.Background(), &repopb.ExistsRequest{Key: "hash"})
			So(err, ShouldBeNil)
			So(r.Ok, ShouldBeFalse)

			// Upload source to filerepo.
			upStream, err := repoClient.Upload(context.Background())
			So(err, ShouldBeNil)

			upStream.Send(&repopb.UploadRequest{
				Value: &repopb.UploadRequest_Meta{
					Meta: &repopb.UploadRequest_MetaMessage{Key: "hash"},
				},
			})
			_, err = upStream.Recv()
			So(err, ShouldBeNil)

			upStream.Send(&repopb.UploadRequest{
				Value: &repopb.UploadRequest_Chunk{
					Chunk: &repopb.UploadRequest_ChunkMessage{
						Name: "file.js",
						Data: []byte(`
setTimeout(function() {
	console.log(META['metaKey'], ARGS['argKey'], CONFIG['configKey'])
}, 1000);
`),
					},
				},
			})
			upStream.Send(&repopb.UploadRequest{
				Value: &repopb.UploadRequest_Done{
					Done: true,
				},
			})
			_, err = upStream.Recv()
			So(err, ShouldBeNil)

			// Check if Exists returns true now.
			r, err = repoClient.Exists(context.Background(), &repopb.ExistsRequest{Key: "hash"})
			So(err, ShouldBeNil)
			So(r.Ok, ShouldBeTrue)

			// Simple request.
			requestMeta := scriptpb.RunRequest_MetaMessage{
				Runtime:    "nodejs_v8",
				SourceHash: "hash",
				Options: &scriptpb.RunRequest_MetaMessage_OptionsMessage{
					EntryPoint: "file.js",
					Args:       []byte(`{"argKey":"argVal"}`),
					Meta:       []byte(`{"metaKey":"metaVal"}`),
					Config:     []byte(`{"configKey":"configVal"}`),
				},
			}

			runRequest := lbpb.RunRequest{
				Value: &lbpb.RunRequest_Request{
					Request: &scriptpb.RunRequest{
						Value: &scriptpb.RunRequest_Meta{
							Meta: &requestMeta,
						},
					},
				},
			}

			scriptClient := lbpb.NewScriptRunnerClient(conn)

			Convey("run single script", func() {
				runStream, err := scriptClient.Run(context.Background())
				So(err, ShouldBeNil)
				err = runStream.Send(&runRequest)
				So(err, ShouldBeNil)
				runStream.CloseSend()

				result, err := runStream.Recv()
				So(err, ShouldEqual, nil)
				So(result.GetCode(), ShouldEqual, 0)
				So(result.GetTook(), ShouldBeBetweenOrEqual, 1000, 1500)
				So(result.GetResponse(), ShouldBeNil)
				So(result.GetStdout(), ShouldResemble, []byte("metaVal argVal configVal\n"))
				So(result.GetStderr(), ShouldBeEmpty)

				_, err = runStream.Recv()
				So(err, ShouldEqual, io.EOF)
			})
			Convey("finishes running scripts during shutdown", func() {
				runStream, err := scriptClient.Run(context.Background())
				So(err, ShouldBeNil)
				err = runStream.Send(&runRequest)
				So(err, ShouldBeNil)
				runStream.CloseSend()

				// Kill servers to check if they gracefully finish remaining scripts.
				// Wait for worker to pick up the RPC.
				scan := bufio.NewScanner(wStderr)
				for scan.Scan() {
					l := scan.Text()
					if strings.Contains(l, "grpc:script:Run") {
						break
					}
				}
				syscall.Kill(-lbCmd.Process.Pid, syscall.SIGTERM)
				syscall.Kill(-workerCmd.Process.Pid, syscall.SIGTERM)

				result, err := runStream.Recv()
				So(err, ShouldEqual, nil)
				So(result.GetCode(), ShouldEqual, 0)
				So(result.GetStdout(), ShouldResemble, []byte("metaVal argVal configVal\n"))
			})
			Convey("run scripts concurrently", func() {
				scriptClient := lbpb.NewScriptRunnerClient(conn)
				resCh := make(chan *scriptpb.RunResponse, 2)
				var wg sync.WaitGroup
				now := time.Now()

				for i := uint(0); i < script.DefaultOptions.Concurrency; i++ {
					wg.Add(1)
					go func() {
						runStream, _ := scriptClient.Run(context.Background())

						runStream.Send(&runRequest)
						runStream.CloseSend()
						result, _ := runStream.Recv()
						resCh <- result
						wg.Done()
					}()
				}
				wg.Wait()
				So(time.Since(now), ShouldBeBetweenOrEqual, 1*time.Second, 1500*time.Millisecond)

				for i := uint(0); i < script.DefaultOptions.Concurrency; i++ {
					res := <-resCh
					So(res.GetStdout(), ShouldResemble, []byte("metaVal argVal configVal\n"))
				}
			})
		})
		Convey("expvar is correctly exposed", func() {
			lbVars, err := readURL("http://localhost:10080/metrics")
			So(err, ShouldBeNil)
			So(string(lbVars), ShouldContainSubstring, "codebox_worker_count 1")
			workerVars, err := readURL("http://localhost:10180/metrics")
			So(err, ShouldBeNil)
			So(string(workerVars), ShouldContainSubstring, fmt.Sprintf("codebox_worker_cpu %d", script.DefaultOptions.MCPU-docker.DefaultOptions.ReservedMCPU))
		})

		// Kill started processes and their children.
		conn.Close()
		syscall.Kill(-workerCmd.Process.Pid, syscall.SIGTERM)
		syscall.Kill(-lbCmd.Process.Pid, syscall.SIGTERM)
		workerCmd.Wait()
		lbCmd.Wait()
	})
}
