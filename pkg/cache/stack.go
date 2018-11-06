package cache

import (
	"container/list"
	"time"
)

// StackCache describes a struct used for LIFO stack cache with ttl.
type StackCache struct {
	Cache
}

// NewStackCache creates and initializes a new cache object. It was adapted from go-cache for needs of script runner.
// This one is based on linked lists - supporting LIFO-like stack with ttl.
// Can easily be adapted to FIFO if needed.
func NewStackCache(options Options) *StackCache {
	cache := StackCache{}
	cache.Cache.Init(options, cache.deleteHandlerImpl)
	return &cache
}

// Push adds an item to cache at given key to the end of list.
func (c *StackCache) Push(key string, val interface{}) {
	c.mu.Lock()

	listI, ok := c.valueMap[key]
	var lst *list.List
	if ok {
		lst = listI.(*list.List)
	} else {
		lst = list.New()
		c.valueMap[key] = lst
	}
	cItem := &Item{
		object:     val,
		expiration: time.Now().Add(c.options.TTL).UnixNano(),
	}

	c.increaseLength()
	elem := lst.PushBack(cItem)
	// Push element to the valuesList and store reference to that element in cacheItem as well.
	cItem.valuesListElement = c.valuesList.PushBack(&valuesItem{key: key, item: cItem, meta: elem})

	c.mu.Unlock()
}

// Pop pops an item from the end of cache list at given key. Returns the item or nil.
func (c *StackCache) Pop(key string) interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	val, ok := c.valueMap[key]
	if !ok {
		return nil
	}

	lst := val.(*list.List)
	for ele := lst.Back(); ele != nil; ele = ele.Prev() {
		cItem := ele.Value.(*Item)

		if time.Now().UnixNano() > cItem.expiration {
			continue
		}

		c.deleteValue(cItem.valuesListElement, 0)
		return cItem.object
	}
	return nil
}

func (c *StackCache) deleteHandlerImpl(item *valuesItem, val interface{}) (valueEvicted *keyValue) {
	lst := val.(*list.List)
	lst.Remove(item.meta.(*list.Element))
	valueEvicted = &keyValue{key: item.key, value: item.item.object}

	// If it's the last element, remove the whole list from map.
	if lst.Len() == 0 {
		delete(c.valueMap, item.key)
	}
	return
}
