package script

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/imdario/mergo"

	"github.com/Syncano/codebox/pkg/util"
)

// Only GET/SET are allowed for now.
const (
	minCommandLength = 3
	maxCommandLength = 3
)

type UserCacheConstraints struct {
	MaxKeyLen   int
	MaxValueLen int

	CardinalityLimit int
	SizeLimit        int
	DefaultTimeout   time.Duration
}

type UserCache struct {
	userID      string
	rw          io.ReadWriter
	redisCli    RedisClient
	constraints *UserCacheConstraints
}

var (
	DefaultUserCacheConstraints = &UserCacheConstraints{
		MaxKeyLen:        128,
		MaxValueLen:      5 << 20,
		CardinalityLimit: 50,
		SizeLimit:        25 << 20,
		DefaultTimeout:   24 * time.Hour,
	}
	ErrCacheCommandUnrecognized        = errors.New("cache command unrecognized")
	ErrCacheCommandMalformed           = errors.New("cache command malformed")
	ErrCacheKeyLengthExceeded          = errors.New("cache key length exceeded")
	ErrCacheValueLengthExceeded        = errors.New("cache value length exceeded")
	errLengthExceeded                  = errors.New("length exceeded")
	initOnceCache                      sync.Once
	redisPopCacheSHA, redisDecrSizeSHA string
)

const redisPopCacheScript = `
local zset = KEYS[1]
local card = redis.call('ZCARD', zset)
local cl = tonumber(ARGV[1])
local sl = tonumber(ARGV[2])
local size = tonumber(redis.call('GET', zset .. ':size'))

if size > sl then
	local ele, eleval, elesize

	while size > sl do
		ele = redis.call('ZPOPMIN', zset)
		elesize = redis.call('STRLEN', ele[0])
		redis.call('DEL', ele[0])
		redis.call('DECRBY', zset .. ':size', elesize)
	end
elseif card > cl then
	local ele = redis.call('ZPOPMIN', zset)
	local elesize = redis.call('STRLEN', ele[0])
	redis.call('DEL', ele[0])
	redis.call('DECRBY', zset .. ':size', elesize)
end
return 1
`

const redisDecrSizeScript = `
local l = redis.call('STRLEN', KEYS[2]);
if l > 0 then
	return redis.call('DECRBY', KEYS[1], l)
end
return 1
`

func NewUserCache(userID string, rw io.ReadWriter, redisCli RedisClient, constraints *UserCacheConstraints) *UserCache {
	if constraints != nil {
		mergo.Merge(constraints, DefaultUserCacheConstraints) // nolint - error not possible
	} else {
		constraints = DefaultUserCacheConstraints
	}

	initOnceCache.Do(func() {
		var err error

		redisPopCacheSHA, err = redisCli.ScriptLoad(redisPopCacheScript).Result()
		util.Must(err)

		redisDecrSizeSHA, err = redisCli.ScriptLoad(redisDecrSizeScript).Result()
		util.Must(err)
	})

	return &UserCache{
		userID:      userID,
		rw:          rw,
		redisCli:    redisCli,
		constraints: constraints,
	}
}

func (c *UserCache) readPart(maxLen int) ([]byte, error) {
	var l uint32

	err := binary.Read(c.rw, binary.LittleEndian, &l)
	if err != nil {
		return nil, err
	}

	if maxLen > 0 && l > uint32(maxLen) {
		return nil, errLengthExceeded
	}

	buf := make([]byte, l)
	_, err = io.ReadFull(c.rw, buf)

	return buf, err
}

func (c *UserCache) readCommand() ([]byte, error) {
	cmd, err := c.readPart(maxCommandLength)
	if err == errLengthExceeded {
		return cmd, ErrCacheKeyLengthExceeded
	}

	if err != nil {
		return cmd, err
	}

	l := len(cmd)

	if l < minCommandLength || l > maxCommandLength {
		return nil, ErrCacheCommandUnrecognized
	}

	return cmd, err
}

func (c *UserCache) readKey() ([]byte, error) {
	key, err := c.readPart(c.constraints.MaxKeyLen)
	if err == errLengthExceeded {
		return key, ErrCacheKeyLengthExceeded
	}

	return key, err
}

func (c *UserCache) readValue() ([]byte, error) {
	val, err := c.readPart(c.constraints.MaxValueLen)
	if err == errLengthExceeded {
		return val, ErrCacheKeyLengthExceeded
	}

	return val, err
}

func (c *UserCache) cacheZSet() string {
	return fmt.Sprintf("%s:user_cache:zset", c.userID)
}

func (c *UserCache) cacheKey(key []byte) string {
	return fmt.Sprintf("%s:user_cache:val:%s", c.userID, string(key))
}

func (c *UserCache) processGET(parts uint32) error {
	if parts != 2 {
		return ErrCacheCommandMalformed
	}

	key, err := c.readKey()
	if err != nil {
		return err
	}

	storKey := c.cacheKey(key)

	ret, err := c.redisCli.Get(storKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return c.send(nil)
		}

		return err
	}

	_, err = c.redisCli.Pipelined(func(r redis.Pipeliner) error {
		zset := c.cacheZSet()

		r.ZAdd(zset, &redis.Z{
			Score:  float64(time.Now().Unix()),
			Member: storKey,
		})
		r.Expire(zset, c.constraints.DefaultTimeout)

		return nil
	})
	if err != nil {
		return fmt.Errorf("user cache get error: %w", err)
	}

	return c.send(ret)
}

func (c *UserCache) processSET(parts uint32) error {
	if parts != 3 {
		return ErrCacheCommandMalformed
	}

	key, err := c.readKey()
	if err != nil {
		return err
	}

	val, err := c.readValue()
	if err != nil {
		return err
	}

	zset := c.cacheZSet()
	storKey := c.cacheKey(key)

	_, err = c.redisCli.Pipelined(func(r redis.Pipeliner) error {
		zsetSizeKey := zset + ":size"
		r.EvalSha(redisDecrSizeSHA, []string{zsetSizeKey, storKey})
		r.Set(storKey, val, c.constraints.DefaultTimeout)

		r.ZAdd(zset, &redis.Z{
			Score:  float64(time.Now().Unix()),
			Member: storKey,
		})
		r.Expire(zset, c.constraints.DefaultTimeout)
		r.IncrBy(zsetSizeKey, int64(len(val)))
		r.Expire(zsetSizeKey, c.constraints.DefaultTimeout)
		return nil
	})
	if err != nil {
		return fmt.Errorf("user cache set error: %w", err)
	}

	err = c.redisCli.EvalSha(redisPopCacheSHA, []string{zset},
		c.constraints.CardinalityLimit, c.constraints.SizeLimit).Err()
	if err != nil {
		return fmt.Errorf("user cache eval error: %w", err)
	}

	return c.send([]byte("OK"))
}

func (c *UserCache) send(data []byte) error {
	if data == nil {
		return binary.Write(c.rw, binary.LittleEndian, int32(-1))
	}

	err := binary.Write(c.rw, binary.LittleEndian, int32(len(data)+4))
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	_, err = c.rw.Write(data)

	return err
}

func (c *UserCache) Process() error {
	for {
		var parts uint32

		err := binary.Read(c.rw, binary.LittleEndian, &parts)
		if err != nil {
			return err
		}

		if parts == 0 {
			return ErrCacheCommandUnrecognized
		}

		cmd, err := c.readCommand()
		if err != nil {
			return err
		}

		switch string(cmd) {
		case "GET":
			err = c.processGET(parts)
		case "SET":
			err = c.processSET(parts)
		}

		if err != nil {
			return err
		}
	}
}