// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"

	proto "github.com/Syncano/codebox/pkg/lb/proto"
)

// WorkerPlugClient is an autogenerated mock type for the WorkerPlugClient type
type WorkerPlugClient struct {
	mock.Mock
}

// ContainerRemoved provides a mock function with given fields: ctx, in, opts
func (_m *WorkerPlugClient) ContainerRemoved(ctx context.Context, in *proto.ContainerRemovedRequest, opts ...grpc.CallOption) (*proto.ContainerRemovedResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *proto.ContainerRemovedResponse
	if rf, ok := ret.Get(0).(func(context.Context, *proto.ContainerRemovedRequest, ...grpc.CallOption) *proto.ContainerRemovedResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*proto.ContainerRemovedResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *proto.ContainerRemovedRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Disconnect provides a mock function with given fields: ctx, in, opts
func (_m *WorkerPlugClient) Disconnect(ctx context.Context, in *proto.DisconnectRequest, opts ...grpc.CallOption) (*proto.DisconnectResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *proto.DisconnectResponse
	if rf, ok := ret.Get(0).(func(context.Context, *proto.DisconnectRequest, ...grpc.CallOption) *proto.DisconnectResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*proto.DisconnectResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *proto.DisconnectRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Heartbeat provides a mock function with given fields: ctx, in, opts
func (_m *WorkerPlugClient) Heartbeat(ctx context.Context, in *proto.HeartbeatRequest, opts ...grpc.CallOption) (*proto.HeartbeatResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *proto.HeartbeatResponse
	if rf, ok := ret.Get(0).(func(context.Context, *proto.HeartbeatRequest, ...grpc.CallOption) *proto.HeartbeatResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*proto.HeartbeatResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *proto.HeartbeatRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Register provides a mock function with given fields: ctx, in, opts
func (_m *WorkerPlugClient) Register(ctx context.Context, in *proto.RegisterRequest, opts ...grpc.CallOption) (*proto.RegisterResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *proto.RegisterResponse
	if rf, ok := ret.Get(0).(func(context.Context, *proto.RegisterRequest, ...grpc.CallOption) *proto.RegisterResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*proto.RegisterResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *proto.RegisterRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
