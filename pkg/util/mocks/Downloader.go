// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	context "context"

	util "github.com/Syncano/codebox/pkg/util"
	mock "github.com/stretchr/testify/mock"
)

// Downloader is an autogenerated mock type for the Downloader type
type Downloader struct {
	mock.Mock
}

// Download provides a mock function with given fields: ctx, urls
func (_m *Downloader) Download(ctx context.Context, urls []string) <-chan *util.DownloadResult {
	ret := _m.Called(ctx, urls)

	var r0 <-chan *util.DownloadResult
	if rf, ok := ret.Get(0).(func(context.Context, []string) <-chan *util.DownloadResult); ok {
		r0 = rf(ctx, urls)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan *util.DownloadResult)
		}
	}

	return r0
}
