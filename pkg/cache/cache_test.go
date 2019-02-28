package cache

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
)

type MockHandler struct {
	mock.Mock
}

func (m *MockHandler) OnValueEvicted(key string, val interface{}) {
	m.Called(key, val)
}

func TestCache(t *testing.T) {
	Convey("Given empty cache struct", t, func() {
		c := new(Cache)

		Convey("Init sets default delete handler and starts janitor", func() {
			c.Init(Options{}, nil)
			So(c.janitor, ShouldNotBeNil)
			So(c.deleteHandler, ShouldEqual, c.defaultDeleteHandler)

			Convey("and cannot be called twice", func() {
				So(func() { c.Init(Options{}, nil) }, ShouldPanic)
			})
			Convey("delete returns false when delete handler returns nil", func() {
				c.deleteHandler = func(item *valuesItem) *keyValue {
					return nil
				}
				c.valuesList.PushBack(&valuesItem{key: "key", item: new(Item)})
				So(c.DeleteLRU(), ShouldBeFalse)
			})

			c.StopJanitor()

			Convey("which is stopped by StopJanitor", func() {
				So(c.janitor, ShouldBeNil)
			})
		})

		Convey("Options returns a copy of options struct", func() {
			So(c.Options(), ShouldNotEqual, c.options)
			So(c.Options(), ShouldResemble, c.options)
		})

	})
}
