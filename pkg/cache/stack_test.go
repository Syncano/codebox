package cache

import (
	"container/list"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStackCache(t *testing.T) {
	Convey("Given new Stack cache with OnValueEvicted handler", t, func() {
		c := NewStackCache(Options{Capacity: 3, CleanupInterval: 1 * time.Millisecond})

		So(c.janitor, ShouldNotBeNil)

		Convey("Push/Pop is LIFO", func() {
			c.Push("key1", "value1")
			c.Push("key1", "value2")
			So(c.Pop("key1"), ShouldEqual, "value2")
			So(c.Pop("key1"), ShouldEqual, "value1")
			So(c.Pop("key1"), ShouldBeNil)
		})
		Convey("Pop of non existent key returns nil", func() {
			So(c.Pop("key1"), ShouldBeNil)
		})
		Convey("Pop returns first non-expired item", func() {
			c.Push("key1", "value1")
			c.Push("key1", "value2")
			c.valueMap["key1"].(*list.List).Back().Value.(*Item).expiration = 1
			So(c.Pop("key1"), ShouldEqual, "value1")
		})
		Convey("Pop returns nil if all items are expired", func() {
			c.Push("key1", "value1")
			c.valueMap["key1"].(*list.List).Back().Value.(*Item).expiration = 1
			So(c.Pop("key1"), ShouldBeNil)
		})

		c.StopJanitor()
	})
}
