// Package cache keeps rarely-changing reads (branches, rules, prices, floor plans, events…) in Redis, so a rush of
// customers does not make the database repeat the same queries.
//
// Three rules keep it harmless:
//   - It is optional: switched on by config (redis.enabled / APP_REDIS_ENABLED). Off, nothing here runs.
//   - It never breaks the app: any Redis error means "not cached": the caller reads the database as usual. After
//     an error Redis is left alone for a few seconds, so a dead Redis costs no waiting.
//   - It is never stale after a change: every cached value is stored under the VERSION of the scopes it depends on
//     (a branch, the company…). A write bumps the version, so the old values are never read again; a safety expiry
//     removes them from Redis. Booking data (slots, bookings, payments, seats) is never cached.
//
// Everything is under the prefix "bk:" and is cleared at every start of the app.
package cache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"backend/internal/platform/redis"

	goredis "github.com/redis/go-redis/v9"
)

const prefix = "bk:"

// Cache is the Redis cache. The zero value (and Off()) is a cache that is switched off.
type Cache struct {
	rdb *goredis.Client
	ttl time.Duration

	downUntil atomic.Int64 // after an error, Redis is not asked again until this time (unix nanoseconds)
	dirty     atomic.Bool  // a version bump failed: everything cached must be cleared before it is trusted again
	lastLog   atomic.Int64
}

// Off is a cache that does nothing
func Off() *Cache { return &Cache{} }

// New connects to Redis when the cache is enabled and clears what an earlier run left. A Redis that cannot be
// reached now does not stop the app: reads go to the database until it comes back.
func New(r redis.Config) *Cache {
	if !r.Enabled {
		log.Printf("🧊 cache: off (redis.enabled is false)")
		return Off()
	}
	ttl := time.Duration(r.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	c := &Cache{ttl: ttl, rdb: goredis.NewClient(&goredis.Options{
		Addr: fmt.Sprintf("%s:%s", r.Host, r.Port), Password: r.Password, DB: r.DB,
		DialTimeout: 300 * time.Millisecond, ReadTimeout: 250 * time.Millisecond, WriteTimeout: 250 * time.Millisecond,
		MaxRetries: -1, // an error is the answer: the caller falls back to the database
	})}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Clear(ctx); err != nil {
		c.dirty.Store(true) // clear it before trusting anything, once Redis answers
		log.Printf("⚠️  cache: Redis is not reachable (%v): reading the database until it is", describe(err))
		return c
	}
	log.Printf("🧊 cache: on (Redis %s:%s, expiry %s), cleared at start", r.Host, r.Port, ttl)
	return c
}

// Close closes the Redis connection
func (c *Cache) Close() {
	if c != nil && c.rdb != nil {
		_ = c.rdb.Close()
	}
}

// On reports whether the cache is configured on (it may still be skipping a Redis that is down)
func (c *Cache) On() bool { return c != nil && c.rdb != nil }

// Ping asks Redis directly (not through the "left alone after an error" pause), for the health check.
// enabled is false when the cache is switched off: then there is nothing to ping and err is nil.
func (c *Cache) Ping(ctx context.Context) (enabled bool, err error) {
	if !c.On() {
		return false, nil
	}
	return true, c.rdb.Ping(ctx).Err()
}

// usable: switched on, and Redis was not failing a moment ago
func (c *Cache) usable() bool {
	return c.On() && time.Now().UnixNano() >= c.downUntil.Load()
}

// describe keeps an error short and free of addresses worth hiding (there are no secrets in Redis errors, but no noise)
func describe(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return err.Error()
}

// fail notes a Redis error: leave Redis alone for 5 seconds, say so at most once a minute
func (c *Cache) fail(err error) {
	if errors.Is(err, goredis.Nil) {
		return
	}
	now := time.Now()
	c.downUntil.Store(now.Add(5 * time.Second).UnixNano())
	if last := c.lastLog.Load(); now.UnixNano()-last > int64(time.Minute) && c.lastLog.CompareAndSwap(last, now.UnixNano()) {
		log.Printf("⚠️  cache: Redis error (%s): reading the database meanwhile", describe(err))
	}
}

// Clear removes every value this app stored (only keys under the "bk:" prefix; other data in Redis is untouched)
func (c *Cache) Clear(ctx context.Context) error {
	if !c.On() {
		return nil
	}
	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, prefix+"*", 500).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := c.rdb.Unlink(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		if cursor = next; cursor == 0 {
			return nil
		}
	}
}

// settle clears everything once if a bump failed earlier: only then are old values safe to look at again
func (c *Cache) settle(ctx context.Context) bool {
	if !c.dirty.Load() {
		return true
	}
	if err := c.Clear(ctx); err != nil {
		c.fail(err)
		return false
	}
	c.dirty.Store(false)
	return true
}

// Bump makes everything cached under these scopes unreadable. Call it after the change is saved.
func (c *Cache) Bump(ctx context.Context, scopes ...string) {
	if !c.On() || len(scopes) == 0 {
		return
	}
	pipe := c.rdb.Pipeline()
	for _, s := range scopes {
		pipe.Incr(ctx, prefix+"v:"+s)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		c.dirty.Store(true) // the old values stay in Redis: they are cleared before the next read is trusted
		c.fail(err)
	}
}

// versions reads the current version of each scope (0 when never bumped)
func (c *Cache) versions(ctx context.Context, scopes []string) (string, bool) {
	keys := make([]string, len(scopes))
	for i, s := range scopes {
		keys[i] = prefix + "v:" + s
	}
	vals, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		c.fail(err)
		return "", false
	}
	var sb strings.Builder
	for i, s := range scopes {
		v := "0"
		if str, ok := vals[i].(string); ok {
			v = str
		}
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(s)
		sb.WriteByte('=')
		sb.WriteString(v)
	}
	return sb.String(), true
}

// Remember returns the cached value of key, or loads it (and keeps it) when there is none. The value is stored under
// the current versions of scopes. Any cache problem means load() is simply called.
func Remember[T any](ctx context.Context, c *Cache, scopes []string, key string, load func() (T, error)) (T, error) {
	if !c.usable() || !c.settle(ctx) {
		return load()
	}
	vers, ok := c.versions(ctx, scopes)
	if !ok {
		return load()
	}
	full := prefix + "d:" + vers + ":" + key
	if raw, err := c.rdb.Get(ctx, full).Bytes(); err == nil {
		var out T
		if json.Unmarshal(raw, &out) == nil {
			return out, nil
		}
	} else {
		c.fail(err)
	}
	out, err := load()
	if err != nil {
		return out, err // a failure is never cached
	}
	if raw, merr := json.Marshal(out); merr == nil {
		if serr := c.rdb.Set(ctx, full, raw, c.ttl).Err(); serr != nil {
			c.fail(serr)
		}
	}
	return out, nil
}

// Key makes a short key from anything that tells one query from another (a filter, a time range…)
func Key(name string, parts ...any) string {
	raw, _ := json.Marshal(parts)
	sum := sha1.Sum(raw)
	return name + ":" + hex.EncodeToString(sum[:8])
}

// Scope names
const (
	ScopeCompany   = "company"   // the company's own settings (every branch's settings in force depend on them)
	ScopeBranches  = "branches"  // the branch rows (a branch looked up by id or slug)
	ScopeDirectory = "directory" // the public list of places
)

// Branch is the scope of everything a branch owns: sports, spaces, prices, hours, rules, plans, events…
func Branch(id uint) string {
	if id == 0 {
		return ScopeCompany // company-wide rows (the company's booking rule) have branch 0
	}
	return fmt.Sprintf("b:%d", id)
}

// Event is the scope of one event's prices, areas and lineup
func Event(id uint) string { return fmt.Sprintf("e:%d", id) }
