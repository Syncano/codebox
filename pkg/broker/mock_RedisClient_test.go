// Code generated by mockery v1.0.0. DO NOT EDIT.

package broker

import (
	redis "github.com/go-redis/redis/v7"
	mock "github.com/stretchr/testify/mock"

	time "time"
)

// MockRedisClient is an autogenerated mock type for the RedisClient type
type MockRedisClient struct {
	mock.Mock
}

// Del provides a mock function with given fields: keys
func (_m *MockRedisClient) Del(keys ...string) *redis.IntCmd {
	_va := make([]interface{}, len(keys))
	for _i := range keys {
		_va[_i] = keys[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *redis.IntCmd
	if rf, ok := ret.Get(0).(func(...string) *redis.IntCmd); ok {
		r0 = rf(keys...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*redis.IntCmd)
		}
	}

	return r0
}

// Get provides a mock function with given fields: key
func (_m *MockRedisClient) Get(key string) *redis.StringCmd {
	ret := _m.Called(key)

	var r0 *redis.StringCmd
	if rf, ok := ret.Get(0).(func(string) *redis.StringCmd); ok {
		r0 = rf(key)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*redis.StringCmd)
		}
	}

	return r0
}

// Set provides a mock function with given fields: key, value, expiration
func (_m *MockRedisClient) Set(key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	ret := _m.Called(key, value, expiration)

	var r0 *redis.StatusCmd
	if rf, ok := ret.Get(0).(func(string, interface{}, time.Duration) *redis.StatusCmd); ok {
		r0 = rf(key, value, expiration)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*redis.StatusCmd)
		}
	}

	return r0
}
