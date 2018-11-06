package util

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func handlerFunc(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/wait") {
		time.Sleep(10 * time.Millisecond)
	}
	w.Write([]byte(r.URL.Path))
}

func TestDownloader(t *testing.T) {

	Convey("Given http server and initialized downloader", t, func() {
		srv := http.Server{Addr: ":9123", Handler: http.HandlerFunc(handlerFunc)}
		go srv.ListenAndServe()
		downloader := NewDownloader(DownloaderOptions{RetryCount: 0})

		Convey("downloader processes all files as soon as they are ready", func() {
			ctx := context.Background()
			ch := downloader.Download(ctx, []string{"http://localhost:9123/wait", "http://localhost:9123/instant/1", "http://localhost:9123/instant/2"})
			res := <-ch
			So(res, ShouldNotBeNil)
			So(res.Error, ShouldBeNil)
			So(res.URL, ShouldStartWith, "http://localhost:9123/instant")
			So(string(res.Data), ShouldStartWith, "/instant")

			res = <-ch
			So(res, ShouldNotBeNil)
			So(res.Error, ShouldBeNil)
			So(res.URL, ShouldStartWith, "http://localhost:9123/instant")
			So(string(res.Data), ShouldStartWith, "/instant")

			res = <-ch
			So(res, ShouldNotBeNil)
			So(res.Error, ShouldBeNil)
			So(res.URL, ShouldEqual, "http://localhost:9123/wait")
			So(string(res.Data), ShouldEqual, "/wait")

			res = <-ch
			So(res, ShouldBeNil)
		})

		Convey("Options returns a copy of options struct", func() {
			So(downloader.Options(), ShouldNotEqual, downloader.options)
			So(downloader.Options(), ShouldResemble, downloader.options)
		})

		Convey("downloader propagates invalid request error", func() {
			ctx := context.Background()
			ch := downloader.Download(ctx, []string{"htt&p://abc"})
			res := <-ch
			So(res, ShouldNotBeNil)
			So(res.Error, ShouldNotBeNil)
		})

		Convey("downloader propagates download error", func() {
			ctx := context.Background()
			ch := downloader.Download(ctx, []string{"abc"})
			res := <-ch
			So(res, ShouldNotBeNil)
			So(res.Error, ShouldNotBeNil)
		})

		Convey("downloader stops on context canceled (and optionally returns url error)", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			ch := downloader.Download(ctx, []string{"http://localhost:9123/instant"})
			res := <-ch
			if res != nil {
				So(res.Error.(*url.Error).Err, ShouldResemble, context.Canceled)
			}
		})

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		srv.Shutdown(ctx)
	})
}
