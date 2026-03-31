package election

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisElector struct {
	client *redis.Client
	key    string
	id     string
	ttl    time.Duration
	leader atomic.Bool
}

func newRedisElector(cfg RedisConfig) (*redisElector, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("redis election 缺少 addr 配置")
	}
	key := cfg.Key
	if key == "" {
		key = "dash:leader"
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 15 * time.Second
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	return &redisElector{
		client: client,
		key:    key,
		id:     podID(),
		ttl:    ttl,
	}, nil
}

func (e *redisElector) Run(ctx context.Context, cb LeaderCallbacks) error {
	defer e.client.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		lost, err := e.tryLead(ctx, cb)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("Redis leader election 出错，重试中", "error", err)
		}
		if lost {
			slog.Warn("Redis lock 续期失败，失去 Leader")
		}

		sleepCtx(ctx, 5*time.Second)
	}
}

func (e *redisElector) IsLeader() bool {
	return e.leader.Load()
}

func (e *redisElector) tryLead(ctx context.Context, cb LeaderCallbacks) (lost bool, err error) {
	ok, err := e.client.SetNX(ctx, e.key, e.id, e.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("SETNX 失败: %w", err)
	}
	if !ok {
		return false, nil
	}

	// 退出时尝试释放（仅删除自己持有的 key）
	defer func() {
		e.release(context.Background())
	}()

	e.leader.Store(true)
	slog.Info("获得 Redis lock，当选 Leader", "key", e.key, "id", e.id)
	cb.OnStartedLeading(ctx)

	defer func() {
		e.leader.Store(false)
		cb.OnStoppedLeading()
	}()

	// 每 ttl/3 续期
	ticker := time.NewTicker(e.ttl / 3)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-ticker.C:
			if !e.renew(ctx) {
				return true, nil
			}
		}
	}
}

// renewScript: 仅续期自己持有的 lock。
var renewScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`)

func (e *redisElector) renew(ctx context.Context) bool {
	result, err := renewScript.Run(ctx, e.client, []string{e.key}, e.id, e.ttl.Milliseconds()).Int64()
	if err != nil {
		slog.Warn("Redis lock 续期出错", "error", err)
		return false
	}
	return result == 1
}

// releaseScript: 仅删除自己持有的 lock。
var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

func (e *redisElector) release(ctx context.Context) {
	_, _ = releaseScript.Run(ctx, e.client, []string{e.key}, e.id).Result()
}
