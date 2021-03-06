// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"

	script "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"
)

// ScriptRunnerClient is an autogenerated mock type for the ScriptRunnerClient type
type ScriptRunnerClient struct {
	mock.Mock
}

// Delete provides a mock function with given fields: ctx, in, opts
func (_m *ScriptRunnerClient) Delete(ctx context.Context, in *script.DeleteRequest, opts ...grpc.CallOption) (*script.DeleteResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *script.DeleteResponse
	if rf, ok := ret.Get(0).(func(context.Context, *script.DeleteRequest, ...grpc.CallOption) *script.DeleteResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*script.DeleteResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *script.DeleteRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Run provides a mock function with given fields: ctx, opts
func (_m *ScriptRunnerClient) Run(ctx context.Context, opts ...grpc.CallOption) (script.ScriptRunner_RunClient, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 script.ScriptRunner_RunClient
	if rf, ok := ret.Get(0).(func(context.Context, ...grpc.CallOption) script.ScriptRunner_RunClient); ok {
		r0 = rf(ctx, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(script.ScriptRunner_RunClient)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
