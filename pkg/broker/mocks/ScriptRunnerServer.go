// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	proto "github.com/Syncano/codebox/pkg/broker/proto"
	mock "github.com/stretchr/testify/mock"
)

// ScriptRunnerServer is an autogenerated mock type for the ScriptRunnerServer type
type ScriptRunnerServer struct {
	mock.Mock
}

// Run provides a mock function with given fields: _a0, _a1
func (_m *ScriptRunnerServer) Run(_a0 *proto.RunRequest, _a1 proto.ScriptRunner_RunServer) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*proto.RunRequest, proto.ScriptRunner_RunServer) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
