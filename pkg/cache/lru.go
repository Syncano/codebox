package cache

import (
	"time"

	"github.com/imdario/mergo"
)

// LRUCache describes a struct for caching.
type LRUCache struct {
	Cache
	lruOptions LRUOptions
}

// LRUOptions holds settable options for cache.
type LRUOptions struct {
	AutoRefresh bool
}

var defaultLRUOptions = LRUOptions{
	AutoRefresh: true,
}

// NewLRUCache creates and initializes a new cache object.
// This one is based on LRU KV with TTL
func NewLRUCache(options Options, lruOptions LRUOptions) *LRUCache {
	mergo.Merge(&lruOptions, defaultLRUOptions) // nolint - error not possible
	cache := LRUCache{lruOptions: lruOptions}
	cache.Cache.Init(options, cache.deleteHandler)
	return &cache
}

// Get returns an item at given key. It automatically extends the expiration if auto refresh is true. Returns the item or nil.
func (c *LRUCache) Get(key string) interface{} {
	return c.get(key, c.lruOptions.AutoRefresh)
}

func (c *LRUCache) get(key string, refresh bool) interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	val, ok := c.valueMap[key]
	if !ok {
		return nil
	}

	cItem := val.(*Item)
	if time.Now().UnixNano() > cItem.expiration {
		return nil
	}
	if refresh {
		cItem.expiration = time.Now().Add(c.options.TTL).UnixNano()
		c.valuesList.MoveToBack(cItem.valuesListElement)
	}
	return cItem.object
}

// Refresh extends the expiration of given key. Returns true on success.
func (c *LRUCache) Refresh(key string) bool {
	return c.get(key, true) != nil
}

func (c *LRUCache) set(key string, val interface{}) {
	cItem := &Item{object: val, expiration: time.Now().Add(c.options.TTL).UnixNano()}
	c.increaseLength()
	cItem.valuesListElement = c.valuesList.PushBack(&valuesItem{key: key, item: cItem})
	c.valueMap[key] = cItem
}

// Set assigns a new value to an item at given key.
func (c *LRUCache) Set(key string, val interface{}) {
	c.mu.Lock()

	curVal, ok := c.valueMap[key]
	if ok {
		c.delete(curVal.(*Item).valuesListElement)
	}

	c.set(key, val)
	c.mu.Unlock()
}

// Add assigns a new value to an item at given key if it doesn't exist.
func (c *LRUCache) Add(key string, val interface{}) bool {
	c.mu.Lock()

	curVal, ok := c.valueMap[key]
	if ok {
		cItem := curVal.(*Item)
		cItem.expiration = time.Now().Add(c.options.TTL).UnixNano()
		c.valuesList.MoveToBack(cItem.valuesListElement)

		c.mu.Unlock()
		return false
	}

	c.set(key, val)
	c.mu.Unlock()
	return true
}

// Delete removes an item at given key.
func (c *LRUCache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	curVal, ok := c.valueMap[key]
	if ok {
		c.delete(curVal.(*Item).valuesListElement)
		return true
	}
	return false
}

// Reduce iterates through values and calls func() with key, val and previous returned value.
func (c *LRUCache) Reduce(f func(key string, val interface{}, total interface{}) interface{}) interface{} {
	c.mu.Lock()
	var total interface{}
	for key, val := range c.valueMap {
		cItem := val.(*Item)
		total = f(key, cItem.object, total)
	}
	c.mu.Unlock()
	return total
}

// Contains returns true if item exists, false otherwise. Doesn't affect the order of recently used items.
func (c *LRUCache) Contains(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	val, ok := c.valueMap[key]
	if !ok {
		return false
	}

	cItem := val.(*Item)
	return time.Now().UnixNano() < cItem.expiration
}
