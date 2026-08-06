package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter is the shared surface of the local and Redis-backed limiters.
type Limiter interface {
	Allow(ctx context.Context, rawKey string) Decision
	// HasBudget reports whether the key has at least one token left without
	// consuming it.
	HasBudget(ctx context.Context, rawKey string) bool
}

// tokenBucketScript implements a shared token bucket: refill by elapsed time,
// cap at burst, then consume (ARGV[4]=1) or peek (ARGV[4]=0). State lives in a
// Redis hash so every replica draws from the same budget.
var tokenBucketScript = redis.NewScript(`
local tokens = tonumber(redis.call('HGET', KEYS[1], 't'))
local ts = tonumber(redis.call('HGET', KEYS[1], 's'))
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local consume = tonumber(ARGV[4])
if tokens == nil or ts == nil then
  tokens = burst
  ts = now
end
local elapsed = now - ts
if elapsed > 0 then
  tokens = math.min(burst, tokens + elapsed * rate / 1000000)
  ts = now
end
local allowed = 0
if tokens >= 1 then
  allowed = 1
  if consume == 1 then
    tokens = tokens - 1
  end
end
if consume == 1 or elapsed > 0 then
  redis.call('HSET', KEYS[1], 't', tokens, 's', ts)
  redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[5]))
end
return allowed
`)

// RedisLimiter is a token bucket whose state is shared through Redis, so the
// budget holds across multiple backend replicas. When Redis is unreachable it
// degrades to an in-process bucket with the same policy (per-replica budget),
// matching the documented single-replica behaviour.
type RedisLimiter struct {
	client   redis.Scripter
	policy   Policy
	fallback *LocalLimiter
	now      func() time.Time
}

// NewRedisLimiter builds a Redis-backed limiter with a local fallback.
func NewRedisLimiter(client redis.Scripter, p Policy) *RedisLimiter {
	local := NewLocalLimiter(p)
	return &RedisLimiter{client: client, policy: local.policy, fallback: local, now: time.Now}
}

func (l *RedisLimiter) redisKey(rawKey string) string {
	return "ratelimit:" + l.policy.ID + ":" + safeKey(rawKey)
}

func (l *RedisLimiter) run(ctx context.Context, rawKey string, consume int) (bool, error) {
	res, err := tokenBucketScript.Run(ctx, l.client,
		[]string{l.redisKey(rawKey)},
		float64(l.policy.Rate),
		l.policy.Burst,
		l.now().UnixMicro(),
		consume,
		l.policy.TTL.Milliseconds(),
	).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// Allow consumes one token when available; on Redis failure it falls back to
// the in-process bucket so the entry keeps limiting instead of failing open.
func (l *RedisLimiter) Allow(ctx context.Context, rawKey string) Decision {
	if l == nil || l.client == nil {
		return l.fallbackAllow(ctx, rawKey)
	}
	allowed, err := l.run(ctx, rawKey, 1)
	if err != nil {
		return l.fallbackAllow(ctx, rawKey)
	}
	if allowed {
		return Decision{Allowed: true, PolicyID: l.policy.ID, KeyHash: safeKey(rawKey)}
	}
	return Decision{Allowed: false, RetryAfter: l.policy.RetryHint, PolicyID: l.policy.ID, KeyHash: safeKey(rawKey)}
}

// HasBudget peeks without consuming; on Redis failure it uses the local view.
func (l *RedisLimiter) HasBudget(ctx context.Context, rawKey string) bool {
	if l == nil || l.client == nil {
		return l.fallback.HasBudget(ctx, rawKey)
	}
	allowed, err := l.run(ctx, rawKey, 0)
	if err != nil {
		return l.fallback.HasBudget(ctx, rawKey)
	}
	return allowed
}

func (l *RedisLimiter) fallbackAllow(ctx context.Context, rawKey string) Decision {
	if l == nil || l.fallback == nil {
		return Decision{Allowed: true}
	}
	return l.fallback.Allow(ctx, rawKey)
}
