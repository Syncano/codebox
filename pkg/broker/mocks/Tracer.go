// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"
import opentracing "github.com/opentracing/opentracing-go"

// Tracer is an autogenerated mock type for the Tracer type
type Tracer struct {
	mock.Mock
}

// Extract provides a mock function with given fields: format, carrier
func (_m *Tracer) Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error) {
	ret := _m.Called(format, carrier)

	var r0 opentracing.SpanContext
	if rf, ok := ret.Get(0).(func(interface{}, interface{}) opentracing.SpanContext); ok {
		r0 = rf(format, carrier)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(opentracing.SpanContext)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(interface{}, interface{}) error); ok {
		r1 = rf(format, carrier)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Inject provides a mock function with given fields: sm, format, carrier
func (_m *Tracer) Inject(sm opentracing.SpanContext, format interface{}, carrier interface{}) error {
	ret := _m.Called(sm, format, carrier)

	var r0 error
	if rf, ok := ret.Get(0).(func(opentracing.SpanContext, interface{}, interface{}) error); ok {
		r0 = rf(sm, format, carrier)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// StartSpan provides a mock function with given fields: operationName, opts
func (_m *Tracer) StartSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, operationName)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 opentracing.Span
	if rf, ok := ret.Get(0).(func(string, ...opentracing.StartSpanOption) opentracing.Span); ok {
		r0 = rf(operationName, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(opentracing.Span)
		}
	}

	return r0
}
