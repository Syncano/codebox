package cache

import (
	"container/list"
	"sync"
	"time"

	"github.com/imdario/mergo"
)

// Cache is an abstract raw struct used for LIFO-like LRU cache with ttl.
// As it provides no value access interface, it should be extended depending on what's needed.
// All exported methods should be thread-safe and use internal mutex.
// See LRUCache for example implementation.
type Cache struct {
	options Options

	// Maintain both valueMap and valuesList in sync.
	// valueMap is used as a storage for key->list
	// valuesList is used to keep it in TTL order so that we can expire them in same order.
	mu         sync.RWMutex
	valueMap   map[string]interface{}
	valuesList *list.List
	janitor    *janitor

	muHandler      sync.RWMutex
	onValueEvicted EvictionHandler

	deleteHandler DeleteHandler
}

// Options holds settable options for cache.
type Options struct {
	TTL             time.Duration
	CleanupInterval time.Duration
	Capacity        int
}

var defaultOptions = &Options{
	TTL:             30 * time.Second,
	CleanupInterval: 15 * time.Second,
}

// Item is used as a single element in cache.
type Item struct {
	object            interface{}
	expiration        int64
	ttl               time.Duration
	valuesListElement *list.Element
}

type valuesItem struct {
	key  string
	item *Item
}

type keyValue struct {
	key   string
	value interface{}
}

// DeleteHandler is a function that should be provided in an implementation embedding Cache struct.
// It is especially useful when dealing with LIFO/FIFO stack-like objects.
// For standard KV storage there is a defaultDeleteHandler that should be enough.
type DeleteHandler func(*valuesItem) *keyValue

// EvictionHandler is a callback function called whenever eviction happens.
type EvictionHandler func(string, interface{})

// Init initializes cache struct fields and starts janitor process.
func (c *Cache) Init(options *Options, deleteHandler DeleteHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.janitor != nil {
		panic("init on cache cannot be called twice")
	}

	if options != nil {
		mergo.Merge(options, defaultOptions) // nolint - error not possible
	} else {
		options = defaultOptions
	}

	c.options = *options

	if deleteHandler == nil {
		deleteHandler = c.defaultDeleteHandler
	}

	c.deleteHandler = deleteHandler
	c.valueMap = make(map[string]interface{})
	c.valuesList = list.New()
	c.janitor = &janitor{
		interval: options.CleanupInterval,
		stop:     make(chan struct{}),
	}

	go c.janitor.Run(c)
}

// Options returns a copy of options struct.
func (c *Cache) Options() Options {
	return c.options
}

// StopJanitor is meant to be called when cache is no longer needed to avoid leaking goroutine.
func (c *Cache) StopJanitor() {
	c.mu.Lock()
	if c.janitor != nil {
		close(c.janitor.stop)
		c.janitor = nil
	}
	c.mu.Unlock()
}

// Len returns cache length.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.valuesList.Len()
}

// OnValueEvicted sets an (optional) function that is called with the key and value when value is evicted from the cache.
// Set to nil to disable.
func (c *Cache) OnValueEvicted(f EvictionHandler) {
	c.muHandler.Lock()
	c.onValueEvicted = f
	c.muHandler.Unlock()
}

// DeleteOne deletes one element that is closest to expiring. Returns true if list was not empty.
// Calls onValueEvicted.
func (c *Cache) DeleteLRU() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.deleteLRU()
}

func (c *Cache) deleteLRU() bool {
	e := c.valuesList.Front()
	if e == nil {
		return false
	}

	return c.delete(e)
}

// Flush deletes all elements in cache.
func (c *Cache) Flush() {
	c.DeleteByTime(0)
}

// DeleteExpired deletes items by checking their expiration against current time. Calls onValueEvicted.
func (c *Cache) DeleteExpired() {
	c.DeleteByTime(time.Now().UnixNano())
}

// DeleteByTime deletes items by checking their expiration against passed now value. If now == 0, deletes all items.
// Calls onValueEvicted.
func (c *Cache) DeleteByTime(now int64) {
	var evictedValues []*keyValue

	c.mu.Lock()

	// Iterate through valuesList (list of valuesListItem) and delete underlying cacheItem if it has expired.
	for e := c.valuesList.Front(); e != nil; {
		nextE := e.Next()

		valueEvicted := c.deleteValue(e, now)
		if valueEvicted == nil {
			break
		}

		e = nextE

		evictedValues = append(evictedValues, valueEvicted)
	}

	c.muHandler.RLock()
	if c.onValueEvicted != nil {
		for _, v := range evictedValues {
			c.onValueEvicted(v.key, v.value)
		}
	}
	c.muHandler.RUnlock()
	c.mu.Unlock()
}

func (c *Cache) defaultDeleteHandler(item *valuesItem) *keyValue {
	delete(c.valueMap, item.key)
	return &keyValue{key: item.key, value: item.item.object}
}

func (c *Cache) delete(e *list.Element) bool {
	if v := c.deleteValue(e, 0); v != nil {
		c.muHandler.RLock()
		if c.onValueEvicted != nil {
			c.onValueEvicted(v.key, v.value)
		}
		c.muHandler.RUnlock()

		return true
	}

	return false
}

// deleteValue deletes exactly one element from valuesList. If now != 0, it is checked against expiration time.
// Note: doesn't call onValueEvicted. Returns evicted keyValue.
func (c *Cache) deleteValue(e *list.Element, now int64) (valueEvicted *keyValue) {
	item := e.Value.(*valuesItem)

	if now != 0 && item.item.expiration > now {
		return
	}

	if _, ok := c.valueMap[item.key]; ok {
		valueEvicted = c.deleteHandler(item)
	}

	c.valuesList.Remove(e)

	return
}

func (c *Cache) checkLength() {
	// If we are over the capacity, delete one closest to expiring.
	if c.options.Capacity > 0 {
		for c.valuesList.Len() > c.options.Capacity {
			c.deleteLRU()
		}
	}
}

func (c *Cache) add(vi *valuesItem) *list.Element {
	exp := vi.item.expiration

	var at *list.Element

	for at = c.valuesList.Front(); at != nil; {
		if at.Value.(*valuesItem).item.expiration > exp {
			break
		}

		at = at.Next()
	}

	if at != nil {
		return c.valuesList.InsertBefore(vi, at)
	}

	return c.valuesList.PushBack(vi)
}

func (c *Cache) sortMove(ele *list.Element) {
	exp := ele.Value.(*valuesItem).item.expiration

	var at *list.Element

	for at = c.valuesList.Front(); at != nil; {
		if at.Value.(*valuesItem).item.expiration > exp {
			break
		}

		at = at.Next()
	}

	if at != nil {
		c.valuesList.MoveBefore(ele, at)
	}

	c.valuesList.MoveToBack(ele)
}

type janitor struct {
	interval time.Duration
	stop     chan struct{}
}

func (j *janitor) Run(cache *Cache) {
	ticker := time.NewTicker(j.interval)

	for {
		select {
		case <-ticker.C:
			cache.DeleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}
