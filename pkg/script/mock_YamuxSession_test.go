// Code generated by mockery v1.0.0. DO NOT EDIT.

package script

import mock "github.com/stretchr/testify/mock"
import net "net"

// MockYamuxSession is an autogenerated mock type for the YamuxSession type
type MockYamuxSession struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *MockYamuxSession) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Open provides a mock function with given fields:
func (_m *MockYamuxSession) Open() (net.Conn, error) {
	ret := _m.Called()

	var r0 net.Conn
	if rf, ok := ret.Get(0).(func() net.Conn); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(net.Conn)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
