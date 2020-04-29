package limiter

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/imdario/mergo"

	"sync/atomic"

	"github.com/Syncano/codebox/pkg/cache"
)

// Limiter allows semaphore rate limiting functionality.
type Limiter struct {
	mu       sync.Mutex
	channels *cache.LRUCache // string->channel

	options Options
}

// Options holds settable options for limiter.
type Options struct {
	Queue int
	TTL   time.Duration
}

// DefaultOptions holds default options values for limiter.
var DefaultOptions = &Options{
	Queue: 100,
	TTL:   10 * time.Minute,
}

type lockData struct {
	ch    chan struct{}
	queue int32
}

const lockTemplate = "%s:%d"

var (
	// ErrMaxQueueSizeReached signals that queue has overflown.
	ErrMaxQueueSizeReached = errors.New("max queue size reached")
)

// New initializes new limiter.
func New(options *Options) *Limiter {
	if options != nil {
		mergo.Merge(options, DefaultOptions) // nolint - error not possible
	} else {
		options = DefaultOptions
	}

	channels := cache.NewLRUCache(&cache.Options{
		TTL: options.TTL,
	}, &cache.LRUOptions{})
	l := &Limiter{
		options:  *options,
		channels: channels,
	}

	return l
}

func (l *Limiter) createLock(key string, limit int) (lock *lockData) {
	l.mu.Lock()

	v := l.channels.Get(key)
	if v == nil {
		ch := make(chan struct{}, limit)
		lock = &lockData{ch: ch}

		l.channels.Set(key, lock)
	} else {
		lock = v.(*lockData)
	}

	l.mu.Unlock()

	return
}

// Lock tries to get a lock on a semaphore on key with limit.
func (l *Limiter) Lock(ctx context.Context, key string, limit int) error {
	if limit <= 0 {
		return ErrMaxQueueSizeReached
	}

	var lock *lockData

	key = fmt.Sprintf(lockTemplate, key, limit)

	v := l.channels.Get(key)
	if v == nil {
		lock = l.createLock(key, limit)
	} else {
		lock = v.(*lockData)
	}

	defer atomic.AddInt32(&lock.queue, -1)

	if int(atomic.AddInt32(&lock.queue, 1)) > l.options.Queue {
		return ErrMaxQueueSizeReached
	}

	select {
	case lock.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Unlock returns lock to semaphore pool.
func (l *Limiter) Unlock(key string, limit int) {
	key = fmt.Sprintf(lockTemplate, key, limit)

	v := l.channels.Get(key)
	if v == nil {
		return
	}

	<-v.(*lockData).ch
}

// Shutdown stops everything.
func (l *Limiter) Shutdown() {
	// Stop cache janitor.
	l.channels.StopJanitor()
}
