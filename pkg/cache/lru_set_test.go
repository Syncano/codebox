package cache

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
)

func TestLRUSetCache(t *testing.T) {
	Convey("Given new LRU set", t, func() {
		c := NewLRUSetCache(&Options{Capacity: 3, CleanupInterval: 1 * time.Millisecond})

		So(c.janitor, ShouldNotBeNil)
		So(c.deleteHandler, ShouldEqual, c.deleteHandler)
		mh := new(MockHandler)

		Convey("Get/Contains of non existent key returns nil/false", func() {
			So(c.Get("key1"), ShouldBeNil)
			So(c.Contains("key1", "val"), ShouldBeFalse)
		})
		Convey("DeleteLRU on empty cache returns false", func() {
			So(c.DeleteLRU(), ShouldBeFalse)
			mh.AssertNumberOfCalls(t, "OnValueEvicted", 0)
		})
		Convey("with stopped janitor", func() {
			c.StopJanitor()

			Convey("Get/Contains of expired item returns nil/false", func() {
				c.Add("key1", "value1")
				c.valueMap["key1"].(map[interface{}]*Item)["value1"].expiration = 1
				So(c.Get("key1"), ShouldBeNil)
				So(c.Contains("key1", "val"), ShouldBeFalse)
			})
			Convey("Refresh returns false for expired item", func() {
				c.Add("key1", "value1")
				c.valueMap["key1"].(map[interface{}]*Item)["value1"].expiration = 1
				So(c.Refresh("key1", "value1"), ShouldBeFalse)
			})
			Convey("DeleteExpired deletes all items that are expired", func() {
				c.Add("key1", "value1")
				c.valueMap["key1"].(map[interface{}]*Item)["value1"].expiration = 1
				c.Add("key2", "value2")
				c.valueMap["key2"].(map[interface{}]*Item)["value2"].expiration = 1
				c.Add("key3", "value3")

				c.DeleteExpired()
				So(c.Len(), ShouldEqual, 1)
				So(c.Get("key3"), ShouldResemble, []interface{}{"value3"})
			})
		})

		Convey("with OnValueEvicted handler", func() {
			c.OnValueEvicted(mh.OnValueEvicted)

			Convey("Add adds a new item that can be retrieved by Get", func() {
				c.Add("key1", "value1")
				So(c.Len(), ShouldEqual, 1)
				So(c.Get("key1"), ShouldResemble, []interface{}{"value1"})

				c.Add("key2", "value2")
				So(c.Len(), ShouldEqual, 2)
				So(c.Get("key2"), ShouldResemble, []interface{}{"value2"})

				Convey("Add on same key adds a new value", func() {
					c.Add("key1", "value2")
					So(c.Len(), ShouldEqual, 3)
					So(c.Get("key1"), ShouldHaveLength, 2)
				})
				Convey("Add on same key with same value, refreshes", func() {
					exp := c.valueMap["key1"].(map[interface{}]*Item)["value1"].expiration
					c.Add("key1", "value1")
					So(c.Len(), ShouldEqual, 2)
					So(c.Get("key1"), ShouldHaveLength, 1)
					So(exp, ShouldBeLessThan, c.valueMap["key1"].(map[interface{}]*Item)["value1"].expiration)
				})
				Convey("Get doesn't affect refreshes expiration time", func() {
					exp := c.valueMap["key1"].(map[interface{}]*Item)["value1"].expiration
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldNotEqual, "key1")
					c.Get("key1")
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldNotEqual, "key1")
					So(exp, ShouldEqual, c.valueMap["key1"].(map[interface{}]*Item)["value1"].expiration)
				})
				Convey("Refresh refreshes expiration time and moves element to back of LRU", func() {
					exp := c.valueMap["key1"].(map[interface{}]*Item)["value1"].expiration
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldNotEqual, "key1")
					So(c.Refresh("key1", "value1"), ShouldBeTrue)
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldEqual, "key1")
					So(exp, ShouldBeLessThan, c.valueMap["key1"].(map[interface{}]*Item)["value1"].expiration)
				})
				Convey("Refresh returns false if key or value was not found", func() {
					So(c.Refresh("key1", "value2"), ShouldBeFalse)
					So(c.Refresh("key3", "value1"), ShouldBeFalse)
				})
				Convey("Contains doesn't affect expiration time", func() {
					exp := c.valueMap["key1"].(map[interface{}]*Item)["value1"].expiration
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldNotEqual, "key1")
					So(c.Contains("key1", "value1"), ShouldBeTrue)
					So(c.valuesList.Back().Value.(*valuesItem).key, ShouldNotEqual, "key1")
					So(exp, ShouldEqual, c.valueMap["key1"].(map[interface{}]*Item)["value1"].expiration)
				})
				Convey("Delete removes a key", func() {
					mh.On("OnValueEvicted", "key1", "value1").Once()
					So(c.Delete("key1", "value1"), ShouldBeTrue)
					So(c.Delete("key1", "value1"), ShouldBeFalse)
					So(c.deleteHandler(&valuesItem{key: "key3"}), ShouldBeNil)
					So(c.Len(), ShouldEqual, 1)
					So(c.Get("key1"), ShouldBeNil)
				})
				Convey("deleteHandler should return nil for non existing key", func() {
					So(c.Len(), ShouldEqual, 2)
					So(c.deleteHandler(&valuesItem{key: "key3"}), ShouldBeNil)
					So(c.Len(), ShouldEqual, 2)
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
				Convey("Add respects capacity", func() {
					c.OnValueEvicted(nil)
					c.Add("key3", "value3")
					c.Add("key4", "value4")
					So(c.Len(), ShouldEqual, 3)
					So(c.Get("key1"), ShouldBeNil)
				})
			})

			Convey("Janitor cleans up expired values", func() {
				mh.On("OnValueEvicted", mock.Anything, mock.Anything).Twice()
				c.Add("key1", "value1")
				c.Add("key1", "value2")
				c.Add("key2", "value3")
				So(c.Len(), ShouldEqual, 3)
				c.mu.Lock()
				c.valueMap["key1"].(map[interface{}]*Item)["value1"].expiration = 1
				c.valueMap["key1"].(map[interface{}]*Item)["value2"].expiration = 1
				c.mu.Unlock()

				time.Sleep(5 * time.Millisecond)
				So(c.Len(), ShouldEqual, 1)
				So(c.Get("key2"), ShouldResemble, []interface{}{"value3"})
			})
		})

		mh.AssertExpectations(t)
		c.StopJanitor()
	})
}
