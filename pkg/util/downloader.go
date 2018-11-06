package util

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/imdario/mergo"
)

// Downloader defines file downloader interface.
//go:generate mockery -name Downloader
type Downloader interface {
	Download(ctx context.Context, urls []string) <-chan *DownloadResult
}

// HTTPDownloader is used to download files asynchronously.
type HTTPDownloader struct {
	sem     chan bool
	client  *http.Client
	options DownloaderOptions
}

// DownloadResult is returned after download is finished.
type DownloadResult struct {
	URL   string
	Data  []byte
	Error error
}

func (res *DownloadResult) String() string {
	if res.Data != nil {
		return fmt.Sprintf("{URL:%s, DataLen:%d}", res.URL, len(res.Data))
	}
	return fmt.Sprintf("{URL:%s, Error:%s}", res.URL, res.Error)
}

// DownloaderOptions holds information about downloader settable options.
type DownloaderOptions struct {
	Concurrency uint
	Timeout     time.Duration
	RetryCount  int
	RetrySleep  time.Duration
}

var defaultDownloaderOptions = DownloaderOptions{
	Concurrency: 2,
	Timeout:     15 * time.Second,
	RetryCount:  3,
	RetrySleep:  100 * time.Millisecond,
}

// NewDownloader initializes a new downloader.
func NewDownloader(options DownloaderOptions) *HTTPDownloader {
	mergo.Merge(&options, defaultDownloaderOptions) // nolint - error not possible

	sem := make(chan bool, options.Concurrency)
	client := &http.Client{
		Timeout: options.Timeout,
	}

	return &HTTPDownloader{
		options: options,
		sem:     sem,
		client:  client,
	}
}

// Options returns a copy of downloader options struct.
func (d *HTTPDownloader) Options() DownloaderOptions {
	return d.options
}

func (d *HTTPDownloader) downloadURL(ctx context.Context, url string) ([]byte, error) {
	// Create request.
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)

	// Retry request if needed - stop when context error happens.
	var data []byte
	err = RetryWithCritical(d.options.RetryCount, d.options.RetrySleep, func() (bool, error) {
		var e error
		resp, e := d.client.Do(req)
		if e == nil {
			data, e = ioutil.ReadAll(resp.Body)
			resp.Body.Close()
		}
		if e != nil {
			// If it was not a context error - it is not a critical one.
			return ctx.Err() != nil, e
		}
		return false, nil
	})

	return data, err
}

// Download downloads all files in url list and sends them to returned channel.
func (d *HTTPDownloader) Download(ctx context.Context, urls []string) <-chan *DownloadResult {
	ch := make(chan *DownloadResult)

	go func() {
		var wg sync.WaitGroup

		for _, url := range urls {
			wg.Add(1)

			go func(url string) {
				d.sem <- true
				defer func() {
					<-d.sem
					wg.Done()
				}()

				data, err := d.downloadURL(ctx, url)
				select {
				case <-ctx.Done():
				case ch <- &DownloadResult{URL: url, Error: err, Data: data}:
				}
			}(url)
		}

		// Close channel after all urls are downloaded.
		wg.Wait()
		close(ch)
	}()
	return ch
}
