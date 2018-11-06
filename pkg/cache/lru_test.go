package cache

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
)

func TestLRUCache(t *testing.T) {
	Convey("Given new LRU cache with auto refresh", t, func() {
		c := NewLRUCache(Options{Capacity: 3, CleanupInterval: 1 * time.Millisecond}, LRUOptions{})

		So(c.janitor, ShouldNotBeNil)
		So(c.deleteHandler, ShouldEqual, c.defaultDeleteHandler)
		mh := new(MockHandler)

		Convey("Get/Contains of non existent key returns nil/false", func() {
			So(c.Get("key1"), ShouldBeNil)
			So(c.Contains("key1"), ShouldBeFalse)
		})
		Convey("DeleteLRU on empty cache returns false", func() {
			So(c.DeleteLRU(), ShouldBeFalse)
			mh.AssertNumberOfCalls(t, "OnValueEvicted", 0)
		})
		Convey("with stopped janitor", func() {
			c.StopJanitor()

			Convey("Get/Contains of expired item returns nil/false", func() {
				c.Set("key1", "value1")
				c.valueMap["key1"].(*Item).expiration = 1
				So(c.Get("key1"), ShouldBeNil)
				So(c.Contains("key1"), ShouldBeFalse)
			})
			Convey("DeleteExpired deletes all items that are expired", func() {

				c.Set("key1", "value1")
				c.valueMap["key1"].(*Item).expiration = 1
				c.Set("key2", "value2")
				c.valueMap["key2"].(*Item).expiration = 1
				c.Set("key3", "value3")

				c.DeleteExpired()
				So(c.Len(), ShouldEqual, 1)
				So(c.Get("key3"), ShouldEqual, "value3")
			})
		})

		Convey("with OnValueEvicted handler", func() {
			c.OnValueEvicted(mh.OnValueEvicted)

			Convey("Set adds a new item that can be retrieved by Get", func() {
				c.Set("key1", "value1")
				So(c.Len(), ShouldEqual, 1)
				So(c.Get("key1"), ShouldEqual, "value1")

				c.Set("key2", "value2")
				So(c.Len(), ShouldEqual, 2)
				So(c.Get("key2"), ShouldEqual, "value2")

				Convey("Add returns true if key doesn't exist", func() {
					So(c.Add("key3", "value3"), ShouldBeTrue)
					So(c.Len(), ShouldEqual, 3)
					So(c.Get("key3"), ShouldEqual, "value3")
				})
				Convey("Set on same key overwrites", func() {
					mh.On("OnValueEvicted", "key1", "value1").Once()
					c.Set("key1", "value3")
					So(c.Len(), ShouldEqual, 2)
					So(c.Get("key1"), ShouldEqual, "value3")
				})
				Convey("Add on same key returns false and refreshes expiration", func() {
					exp := c.valueMap["key1"].(*Item).expiration
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldNotEqual, "key1")
					So(c.Add("key1", "value3"), ShouldBeFalse)
					So(c.Len(), ShouldEqual, 2)
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldEqual, "key1")
					So(exp, ShouldBeLessThan, c.valueMap["key1"].(*Item).expiration)
					So(c.Get("key1"), ShouldEqual, "value1")
				})
				Convey("Get refreshes expiration time and moves element to back of LRU", func() {
					exp := c.valueMap["key1"].(*Item).expiration
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldNotEqual, "key1")
					c.Get("key1")
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldEqual, "key1")
					So(exp, ShouldBeLessThan, c.valueMap["key1"].(*Item).expiration)
				})
				Convey("Get with autorefresh disabled - doesn't refresh expiration time", func() {
					c.lruOptions.AutoRefresh = false
					exp := c.valueMap["key1"].(*Item).expiration
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldNotEqual, "key1")
					c.Get("key1")
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldNotEqual, "key1")
					So(exp, ShouldEqual, c.valueMap["key1"].(*Item).expiration)
				})
				Convey("Refresh refreshes expiration time and moves element to back of LRU", func() {
					c.lruOptions.AutoRefresh = false
					exp := c.valueMap["key1"].(*Item).expiration
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldNotEqual, "key1")
					c.Refresh("key1")
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldEqual, "key1")
					So(exp, ShouldBeLessThan, c.valueMap["key1"].(*Item).expiration)
				})
				Convey("Contains doesn't affect expiration time", func() {
					exp := c.valueMap["key1"].(*Item).expiration
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldNotEqual, "key1")
					c.Contains("key1")
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldNotEqual, "key1")
					So(exp, ShouldEqual, c.valueMap["key1"].(*Item).expiration)
				})
				Convey("Delete removes a key", func() {
					mh.On("OnValueEvicted", "key1", "value1").Once()
					So(c.Delete("key1"), ShouldBeTrue)
					So(c.Delete("key1"), ShouldBeFalse)
					So(c.Len(), ShouldEqual, 1)
					So(c.Get("key1"), ShouldBeNil)
				})
				Convey("Reduce calls function on each key, value pair", func() {
					var keys []string
					c.Reduce(func(key string, val interface{}, total interface{}) interface{} {
						keys = append(keys, key)
						return nil
					})
					So(len(keys), ShouldEqual, c.Len())
				})
				Convey("Flush removes all elements", func() {
					mh.On("OnValueEvicted", mock.Anything, mock.Anything).Twice()
					c.Flush()
					So(c.Len(), ShouldEqual, 0)
				})
				Convey("DeleteLRU deletes one least recently used item", func() {
					mh.On("OnValueEvicted", "key1", "value1").Once()
					c.DeleteLRU()
					So(c.Len(), ShouldEqual, 1)
				})
				Convey("Set respects capacity", func() {
					c.OnValueEvicted(nil)
					c.Set("key3", "value3")
					c.Set("key4", "value4")
					So(c.Len(), ShouldEqual, 3)
					So(c.Get("key1"), ShouldBeNil)
				})
			})

			Convey("Janitor cleans up expired values", func() {
				mh.On("OnValueEvicted", mock.Anything, mock.Anything).Twice()
				c.Set("key1", "value1")
				c.Set("key2", "value2")
				c.Set("key3", "value3")
				So(c.Len(), ShouldEqual, 3)
				c.mu.Lock()
				c.valueMap["key1"].(*Item).expiration = 1
				c.valueMap["key2"].(*Item).expiration = 1
				c.mu.Unlock()

				time.Sleep(5 * time.Millisecond)
				So(c.Len(), ShouldEqual, 1)
				So(c.Get("key3"), ShouldEqual, "value3")
			})
		})

		mh.AssertExpectations(t)
		c.StopJanitor()
	})
}
