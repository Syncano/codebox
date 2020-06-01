// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	context "context"

	broker "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/broker/v1"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"
)

// ScriptRunnerClient is an autogenerated mock type for the ScriptRunnerClient type
type ScriptRunnerClient struct {
	mock.Mock
}

// Run provides a mock function with given fields: ctx, opts
func (_m *ScriptRunnerClient) Run(ctx context.Context, opts ...grpc.CallOption) (broker.ScriptRunner_RunClient, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 broker.ScriptRunner_RunClient
	if rf, ok := ret.Get(0).(func(context.Context, ...grpc.CallOption) broker.ScriptRunner_RunClient); ok {
		r0 = rf(ctx, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(broker.ScriptRunner_RunClient)
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

// SimpleRun provides a mock function with given fields: ctx, in, opts
func (_m *ScriptRunnerClient) SimpleRun(ctx context.Context, in *broker.SimpleRunRequest, opts ...grpc.CallOption) (broker.ScriptRunner_SimpleRunClient, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 broker.ScriptRunner_SimpleRunClient
	if rf, ok := ret.Get(0).(func(context.Context, *broker.SimpleRunRequest, ...grpc.CallOption) broker.ScriptRunner_SimpleRunClient); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(broker.ScriptRunner_SimpleRunClient)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *broker.SimpleRunRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}