// Code generated by mockery v1.0.0. DO NOT EDIT.

package filerepo

import mock "github.com/stretchr/testify/mock"

// MockCommander is an autogenerated mock type for the Commander type
type MockCommander struct {
	mock.Mock
}

// Run provides a mock function with given fields: _a0, _a1
func (_m *MockCommander) Run(_a0 string, _a1 ...string) error {
	_va := make([]interface{}, len(_a1))
	for _i := range _a1 {
		_va[_i] = _a1[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, ...string) error); ok {
		r0 = rf(_a0, _a1...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
