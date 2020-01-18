package cache

import (
	"time"
)

// LRUSetCache describes a struct for caching.
type LRUSetCache struct {
	Cache
}

// NewLRUSetCache creates and initializes a new cache object.
// This one is based on LRU K->List with TTL
func NewLRUSetCache(options *Options) *LRUSetCache {
	cache := LRUSetCache{}
	cache.Cache.Init(options, cache.deleteHandler)

	return &cache
}

// Get returns all items at given key. Returns list of items or nil.
func (c *LRUSetCache) Get(key string) []interface{} {
	return c.get(key)
}

func (c *LRUSetCache) get(key string) []interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	val, ok := c.valueMap[key]
	if !ok {
		return nil
	}

	var values []interface{}

	cItemMap := val.(map[interface{}]*Item)

	for _, cItem := range cItemMap {
		if time.Now().UnixNano() < cItem.expiration {
			values = append(values, cItem.object)
		}
	}

	return values
}

// Refresh extends the expiration of given key. Returns true on success.
func (c *LRUSetCache) Refresh(key string, val interface{}) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	v, ok := c.valueMap[key]
	if !ok {
		return false
	}

	cItem, ok := v.(map[interface{}]*Item)[val]
	if !ok {
		return false
	}

	if cItem.object == val && time.Now().UnixNano() < cItem.expiration {
		cItem.expiration = time.Now().Add(c.options.TTL).UnixNano()
		c.valuesList.MoveToBack(cItem.valuesListElement)

		return true
	}

	return false
}

// Set in LRUSetCache just calls Add. Use it instead.
func (c *LRUSetCache) Set(key string, val interface{}) {
	c.Add(key, val)
}

// Add assigns a new value to an item at given key if it doesn't exist.
func (c *LRUSetCache) Add(key string, val interface{}) bool {
	c.mu.Lock()

	curVal, ok := c.valueMap[key]
	if ok {
		if cItem, ok2 := curVal.(map[interface{}]*Item)[val]; ok2 {
			cItem.expiration = time.Now().Add(c.options.TTL).UnixNano()
			c.valuesList.MoveToBack(cItem.valuesListElement)
			c.mu.Unlock()

			return false
		}
	}

	// Add item.
	cItem := &Item{object: val, expiration: time.Now().Add(c.options.TTL).UnixNano()}
	cItem.valuesListElement = c.valuesList.PushBack(&valuesItem{key: key, item: cItem})
	c.checkLength()

	if ok {
		curVal.(map[interface{}]*Item)[val] = cItem
	} else {
		c.valueMap[key] = map[interface{}]*Item{val: cItem}
	}

	c.mu.Unlock()

	return true
}

// Delete removes an item at given key.
func (c *LRUSetCache) Delete(key string, val interface{}) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if curVal, ok := c.valueMap[key]; ok {
		if v, ok := curVal.(map[interface{}]*Item)[val]; ok {
			return c.delete(v.valuesListElement)
		}
	}

	return false
}

// Reduce iterates through values and calls func() with key, val and previous returned value.
func (c *LRUSetCache) Reduce(f func(key string, val interface{}, total interface{}) interface{}) interface{} {
	c.mu.Lock()
	var (
		total interface{}
		item  *valuesItem
	)

	for e := c.valuesList.Front(); e != nil; {
		item = e.Value.(*valuesItem)
		total = f(item.key, item.item.object, total)
		e = e.Next()
	}

	c.mu.Unlock()

	return total
}

// Contains returns true if item exists, false otherwise. Doesn't affect the order of recently used items.
func (c *LRUSetCache) Contains(key string, val interface{}) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	curVal, ok := c.valueMap[key]
	if !ok {
		return false
	}

	cItem, ok := curVal.(map[interface{}]*Item)[val]
	if !ok {
		return false
	}

	return time.Now().UnixNano() < cItem.expiration
}

func (c *LRUSetCache) deleteHandler(item *valuesItem) *keyValue {
	if curVal, ok := c.valueMap[item.key]; ok {
		m := curVal.(map[interface{}]*Item)
		if _, ok := m[item.item.object]; ok {
			if len(m) == 1 {
				delete(c.valueMap, item.key)
			} else {
				delete(m, item.item.object)
			}

			return &keyValue{key: item.key, value: item.item.object}
		}
	}

	return nil
}
