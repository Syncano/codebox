package script_test

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/go-redis/redis/v7"
	"github.com/sirupsen/logrus"
	. "github.com/smartystreets/goconvey/convey"

	"github.com/Syncano/codebox/app/docker"
	"github.com/Syncano/codebox/app/filerepo"
	"github.com/Syncano/codebox/app/script"
	"github.com/Syncano/pkg-go/v2/sys"
	"github.com/Syncano/pkg-go/v2/util"
)

type scriptTest struct {
	runtime string
	script  string
}

type scriptExpectedTest struct {
	runtime  string
	script   string
	expected string
}

type scriptTimeoutTest struct {
	runtime          string
	script           string
	deadlineExceeded bool
}

const (
	environmentFilename = "squashfs.img"
)

func uploadFile(repo filerepo.Repo, key string, data []byte, filename string) error {
	lockCh, storeKey := repo.StoreLock(key)
	_, err := repo.Store(key, storeKey, bytes.NewReader(data), filename, 0)
	if err != nil {
		return err
	}
	repo.StoreUnlock(key, storeKey, lockCh, true)
	return nil
}

func TestRunnerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	logrus.SetOutput(ioutil.Discard)
	rand.Seed(time.Now().UTC().UnixNano())

	Convey("Given initialized script runner", t, func() {
		// Initialize docker client.
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion(docker.DockerVersion))
		So(err, ShouldBeNil)

		// Initialize docker manager.
		dockerMgr, err := docker.NewManager(&docker.Options{ReservedMCPU: 250}, cli)
		So(err, ShouldBeNil)

		// Initialize system checker.
		syschecker := new(sys.SigarChecker)

		// Initialize file repo.
		repo := filerepo.New(&filerepo.Options{
			BasePath: os.Getenv("REPO_PATH"),
		}, syschecker, new(filerepo.LinkFs), new(filerepo.Command))

		redisAddr, ok := os.LookupEnv("REDIS_ADDR")
		if !ok {
			redisAddr = "redis:6379"
		}
		redisCli := redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: "",
			DB:       0,
		})

		// Initialize script runner.
		runner, err := script.NewRunner(&script.Options{
			Concurrency:       2,
			PruneImages:       false,
			HostStoragePath:   os.Getenv("HOST_STORAGE_PATH"),
			UseExistingImages: true,
		}, dockerMgr, syschecker, repo, redisCli)
		So(err, ShouldBeNil)

		So(runner.DownloadAllImages(), ShouldBeNil)
		poolID, err := runner.CreatePool()
		So(err, ShouldBeNil)
		So(poolID, ShouldNotBeBlank)

		Convey("runs simple scripts", func() {
			var tests = []scriptTest{
				{"nodejs_v8", `console.log(ARGS['arg'] + META['meta'] + CONFIG['cfg'] + '¿¡!')`},
				{"nodejs_v8", `exports.default = (ctx) => { console.log(ctx.args['arg'] + ctx.meta['meta'] + ctx.config['cfg'] + '¿¡!') }`},
				{"nodejs_v8", `module.exports=function(n){function r(t){if(e[t])return e[t].exports;var o=e[t]={i:t,l:!1,exports:{}};return n[t].call(o.exports,o,o.exports,r),o.l=!0,o.exports}var e={};return r.m=n,r.c=e,r.i=function(n){return n},r.d=function(n,e,t){r.o(n,e)||Object.defineProperty(n,e,{configurable:!1,enumerable:!0,get:t})},r.n=function(n){var e=n&&n.__esModule?function(){return n.default}:function(){return n};return r.d(e,"a",e),e},r.o=function(n,r){return Object.prototype.hasOwnProperty.call(n,r)},r.p="",r(r.s=239)}({239:function(n,r){console.log(ARGS['arg']+META['meta']+CONFIG['cfg']+'¿¡!')}});`},
				{"nodejs_v12", `console.log(ARGS['arg'] + META['meta'] + CONFIG['cfg'] + '¿¡!')`},
			}
			for _, data := range tests {
				hash := util.GenerateKey()
				err := uploadFile(repo, hash, []byte(data.script), script.SupportedRuntimes[data.runtime].DefaultEntryPoint)
				So(err, ShouldBeNil)

				for i := 0; i < 2; i++ {
					res, err := runner.Run(context.Background(), logrus.StandardLogger(), "reqID",
						script.NewDefinition(data.runtime,
							hash, "",
							"user",
							"",
							0, 0,
						),
						&script.RunOptions{
							Args:   []byte(`{"arg":"co"}`),
							Meta:   []byte(`{"meta":"de"}`),
							Config: []byte(`{"cfg":"box"}`),
						})
					So(err, ShouldBeNil)

					So(res.Code, ShouldEqual, 0)
					So(res.Took, ShouldBeGreaterThan, 0)
					So(string(res.Stdout), ShouldEqual, "codebox¿¡!\n")
					So(res.Stderr, ShouldBeEmpty)
					So(res.Response, ShouldBeNil)
				}
			}
		})

		Convey("runs script with files", func() {
			var tests = []scriptExpectedTest{
				{"nodejs_v8", `console.log(Object.keys(ARGS).length, ARGS['file'].filename, ARGS['file'].contentType, ARGS['file'].length)`, "1 'fname' 'ctype' 10\n"},
				{"nodejs_v12", `console.log(Object.keys(ARGS).length, ARGS['file'].filename, ARGS['file'].contentType, ARGS['file'].length)`, "1 fname ctype 10\n"},
			}
			for _, data := range tests {
				hash := util.GenerateKey()
				err := uploadFile(repo, hash, []byte(data.script), script.SupportedRuntimes[data.runtime].DefaultEntryPoint)
				So(err, ShouldBeNil)
				res, err := runner.Run(context.Background(), logrus.StandardLogger(), "reqID",
					script.NewDefinition(data.runtime,
						hash, "",
						"user",
						"",
						0, 0,
					),
					&script.RunOptions{
						Files: map[string]*script.File{"file": {Filename: "fname", ContentType: "ctype", Data: []byte("content123")}},
					})
				So(err, ShouldBeNil)

				So(res.Code, ShouldEqual, 0)
				So(res.Took, ShouldBeGreaterThan, 0)
				So(string(res.Stdout), ShouldEqual, data.expected)
				So(res.Stderr, ShouldBeEmpty)
				So(res.Response, ShouldBeNil)
			}
		})

		Convey("runs scripts with custom response", func() {
			var tests = []scriptTest{
				{"nodejs_v8", `setResponse(new HttpResponse(200, 'content', 'content/type', {a:1, b:'c'})); console.log('codebox')`},
				{"nodejs_v12", `exports.default = (ctx) => { ctx.setResponse(new HttpResponse(200, 'content', 'content/type', {a:1, b:'c'})); console.log('codebox'); }`},
				{"nodejs_v12", `exports.default = (ctx) => { console.log('codebox'); return new ctx.HttpResponse(200, 'content', 'content/type', {a:1, b:'c'}); }`},
				{"nodejs_v12", `exports.default = async function(ctx) { console.log('codebox'); return new ctx.HttpResponse(200, 'content', 'content/type', {a:1, b:'c'}); }`},
			}
			for _, data := range tests {
				hash := util.GenerateKey()
				err := uploadFile(repo, hash, []byte(data.script), script.SupportedRuntimes[data.runtime].DefaultEntryPoint)
				So(err, ShouldBeNil)
				res, err := runner.Run(context.Background(), logrus.StandardLogger(), "reqID",
					script.NewDefinition(data.runtime,
						hash, "",
						"user",
						"",
						0, 0,
					),
					&script.RunOptions{})
				So(err, ShouldBeNil)

				So(res.Code, ShouldEqual, 0)
				So(res.Took, ShouldBeGreaterThan, 0)
				So(string(res.Stdout), ShouldEqual, "codebox\n")
				So(res.Stderr, ShouldBeEmpty)

				So(res.Response, ShouldNotBeNil)
				So(res.Response.StatusCode, ShouldEqual, 200)
				So(res.Response.Content, ShouldResemble, []byte("content"))
				So(res.Response.ContentType, ShouldEqual, "content/type")
				So(res.Response.Headers, ShouldResemble, map[string]string{"a": "1", "b": "c"})
			}
		})

		Convey("runs scripts with custom environment", func() {
			tempDir, _ := ioutil.TempDir("", "example")
			node_modules_dir := filepath.Join(tempDir, "node_modules")
			os.MkdirAll(node_modules_dir, os.ModePerm)
			ioutil.WriteFile(filepath.Join(node_modules_dir, "testfile"), []byte("abc"), 0644)
			defer os.RemoveAll(tempDir)

			squashfs := filepath.Join(os.TempDir(), environmentFilename)
			cmd := exec.Command("mksquashfs", tempDir, squashfs, "-comp", "xz", "-noappend")
			e := cmd.Run()
			So(e, ShouldBeNil)
			squashBytes, _ := ioutil.ReadFile(squashfs)
			os.Remove(squashfs)

			var tests = []scriptTest{
				{"nodejs_v8", `require('fs').readdirSync('/app/env/node_modules').forEach(file => { console.log(file) })`},
				{"nodejs_v12", `require('fs').readdirSync('/app/env/node_modules').forEach(file => { console.log(file) })`},
			}
			for _, data := range tests {
				env := util.GenerateKey()
				err := uploadFile(repo, env, squashBytes, environmentFilename)
				So(err, ShouldBeNil)

				hash := util.GenerateKey()
				err = uploadFile(repo, hash, []byte(data.script), "test/entry.js")
				So(err, ShouldBeNil)

				res, err := runner.Run(context.Background(), logrus.StandardLogger(), "reqID",
					script.NewDefinition(data.runtime,
						hash, env,
						"user",
						"test/entry.js",
						0, 0,
					),
					&script.RunOptions{})
				So(err, ShouldBeNil)

				So(res.Code, ShouldEqual, 0)
				So(string(res.Stdout), ShouldEqual, "testfile\n")
				So(res.Stderr, ShouldBeEmpty)
			}
		})

		Convey("handles squashfs malfunctioning properly", func() {
			var tests = []scriptTest{
				{"nodejs_v8", ``},
			}
			for _, data := range tests {
				env := util.GenerateKey()
				err := uploadFile(repo, env, []byte("abc"), environmentFilename)
				So(err, ShouldBeNil)

				hash := util.GenerateKey()
				err = uploadFile(repo, hash, []byte(data.script), "test/entry.js")
				So(err, ShouldBeNil)

				res, err := runner.Run(context.Background(), logrus.StandardLogger(), "reqID",
					script.NewDefinition(data.runtime,
						hash, env,
						"user",
						"",
						0, 0,
					),
					&script.RunOptions{})
				So(res, ShouldBeNil)
				So(err, ShouldNotBeNil)
			}
		})

		Convey("runs scripts with custom entry point", func() {
			var tests = []scriptTest{
				{"nodejs_v8", `console.log(__dirname); console.log(__filename)`},
				{"nodejs_v12", `console.log(__dirname); console.log(__filename)`},
			}
			for _, data := range tests {
				hash := util.GenerateKey()
				err := uploadFile(repo, hash, []byte(data.script), "test/entry.js")
				So(err, ShouldBeNil)
				res, err := runner.Run(context.Background(), logrus.StandardLogger(), "reqID",
					script.NewDefinition(data.runtime,
						hash, "",
						"user",
						"test/entry.js",
						0, 0,
					),
					&script.RunOptions{})
				So(err, ShouldBeNil)

				So(res.Code, ShouldEqual, 0)
				So(string(res.Stdout), ShouldEqual, "/app/code/test\n/app/code/test/entry.js\n")
				So(res.Stderr, ShouldBeEmpty)
			}
		})

		Convey("runs scripts with timeout", func() {
			timeout := 100 * time.Millisecond
			graceTimeout := 3 * time.Second

			var tests = []scriptTimeoutTest{
				{"nodejs_v8", `console.log('codebox'); while(true){}`, false},
				{"nodejs_v12", `console.log('codebox'); while(true){}`, false},
				{"nodejs_v8", `console.log('codebox'); setTimeout(function(){}, 30000)`, true},
			}
			for _, data := range tests {
				t := time.Now()
				hash := util.GenerateKey()
				err := uploadFile(repo, hash, []byte(data.script), script.SupportedRuntimes[data.runtime].DefaultEntryPoint)
				So(err, ShouldBeNil)
				res, err := runner.Run(context.Background(), logrus.StandardLogger(), "reqID",
					script.NewDefinition(data.runtime,
						hash, "",
						"user",
						"",
						0, 0,
					),
					&script.RunOptions{Timeout: timeout})

				if data.deadlineExceeded {
					So(err, ShouldResemble, context.DeadlineExceeded)
					So(time.Since(t), ShouldBeBetween, timeout+graceTimeout, 1*time.Second+graceTimeout)

				} else {
					So(err, ShouldBeNil)
					So(time.Since(t), ShouldBeBetween, timeout, graceTimeout)
					So(string(res.Stdout), ShouldEqual, "codebox\n")
					So(res.Stderr, ShouldNotBeEmpty)
				}

				So(res.Code, ShouldEqual, 124)
				So(res.Took, ShouldBeGreaterThanOrEqualTo, timeout)
				So(res.Response, ShouldBeNil)
			}
		})

		Convey("runs scripts with out of memory error", func() {
			var tests = []scriptTest{
				{"nodejs_v8", `console.log('codebox'); a=Array(128*1024*1024).join('a'); b=Array(128*1024*1024).join('a');`},
			}
			for _, data := range tests {
				hash := util.GenerateKey()
				err := uploadFile(repo, hash, []byte(data.script), script.SupportedRuntimes[data.runtime].DefaultEntryPoint)
				So(err, ShouldBeNil)
				res, err := runner.Run(context.Background(), logrus.StandardLogger(), "reqID",
					script.NewDefinition(data.runtime,
						hash, "",
						"user",
						"",
						0, 0,
					),
					&script.RunOptions{})
				So(err, ShouldEqual, io.EOF)

				So(res.Code, ShouldEqual, 1)
				So(string(res.Stdout), ShouldStartWith, "codebox\n")
				So(res.Stderr, ShouldNotBeEmpty)
				So(res.Response, ShouldBeNil)
			}
		})

		Convey("runs weighted scripts", func() {
			var tests = []scriptTest{
				{"nodejs_v8", `console.log('codebox')`},
				{"nodejs_v12", `console.log('codebox')`},
			}
			for _, data := range tests {
				hash := util.GenerateKey()
				err := uploadFile(repo, hash, []byte(data.script), script.SupportedRuntimes[data.runtime].DefaultEntryPoint)
				So(err, ShouldBeNil)
				res, err := runner.Run(context.Background(), logrus.StandardLogger(), "reqID",
					script.NewDefinition(data.runtime,
						hash, "",
						"user",
						"",
						uint32(runner.Options().Constraints.CPULimit/1e6)*2, 0,
					),
					&script.RunOptions{})
				So(err, ShouldBeNil)

				So(res.Code, ShouldEqual, 0)
				So(string(res.Stdout), ShouldStartWith, "codebox\n")
				So(res.Stderr, ShouldBeEmpty)
				So(res.Response, ShouldBeNil)
				So(res.Weight, ShouldEqual, 2)
			}
		})

		Convey("runs async scripts", func() {
			var tests = []scriptTest{
				{"nodejs_v8", `exports.default = (ctx) => { ctx.log('codebox'); abrakadabra }`},
				{"nodejs_v8", `exports.default = async (ctx) => { ctx.log('codebox'); abrakadabra }`},
				{"nodejs_v12", `exports.default = (ctx) => { ctx.log('codebox'); abrakadabra }`},
				{"nodejs_v12", `exports.default = async (ctx) => { ctx.log('codebox'); abrakadabra }`},
			}
			for _, data := range tests {
				hash := util.GenerateKey()
				err := uploadFile(repo, hash, []byte(data.script), script.SupportedRuntimes[data.runtime].DefaultEntryPoint)
				So(err, ShouldBeNil)
				res, err := runner.Run(context.Background(), logrus.StandardLogger(), "reqID",
					script.NewDefinition(data.runtime,
						hash, "",
						"user",
						"",
						0, 100,
					),
					&script.RunOptions{})
				So(err, ShouldBeNil)

				So(res.Code, ShouldEqual, 1)
				So(string(res.Stdout), ShouldEqual, "codebox\n")
				So(string(res.Stderr), ShouldContainSubstring, "ReferenceError: abrakadabra is not defined\n")
				So(res.Response, ShouldBeNil)
			}
		})

		Convey("use cache in scripts", func() {
			var tests = []scriptTest{
				{"nodejs_v8", `exports.default = async (ctx) => { let a = await ctx.cache.get('test1'); console.log(a === null); await ctx.cache.set('test1', 'val'); a = await ctx.cache.get('test1'); console.log(a.toString()); }`},
				{"nodejs_v12", `exports.default = async (ctx) => { let a = await ctx.cache.get('test2'); console.log(a === null); await ctx.cache.set('test2', 'val'); a = await ctx.cache.get('test2'); console.log(a.toString()); }`},
			}
			for _, data := range tests {
				redisCli.FlushDB()

				hash := util.GenerateKey()
				err := uploadFile(repo, hash, []byte(data.script), script.SupportedRuntimes[data.runtime].DefaultEntryPoint)
				So(err, ShouldBeNil)
				res, err := runner.Run(context.Background(), logrus.StandardLogger(), "reqID",
					script.NewDefinition(data.runtime,
						hash, "",
						"user",
						"",
						0, 0,
					),
					&script.RunOptions{})
				So(err, ShouldBeNil)

				So(res.Code, ShouldEqual, 0)
				So(string(res.Stdout), ShouldEqual, "true\nval\n")
				So(res.Stderr, ShouldBeEmpty)
				So(res.Response, ShouldBeNil)
			}
		})

		runner.Shutdown()
		repo.Shutdown()
	})
}
