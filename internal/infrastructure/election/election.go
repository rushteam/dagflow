package election

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// LeaderCallbacks 定义 Leader 状态变更时的回调。
type LeaderCallbacks struct {
	OnStartedLeading func(ctx context.Context)
	OnStoppedLeading func()
}

// LeaderElector 抽象 Leader Election 机制。
type LeaderElector interface {
	// Run 阻塞运行选举循环，直到 ctx 取消。
	Run(ctx context.Context, cb LeaderCallbacks) error
	// IsLeader 返回当前是否为 Leader。
	IsLeader() bool
}

// Config 选举配置。
type Config struct {
	Driver string      `yaml:"driver"` // "pg" | "redis" | "kube" | "" (noop)
	PG     PGConfig    `yaml:"pg"`
	Redis  RedisConfig `yaml:"redis"`
	Kube   KubeConfig  `yaml:"kube"`
}

type PGConfig struct {
	LockID int64 `yaml:"lock_id"`
}

type RedisConfig struct {
	Addr     string        `yaml:"addr"`
	Password string        `yaml:"password"`
	DB       int           `yaml:"db"`
	Key      string        `yaml:"key"`
	TTL      time.Duration `yaml:"ttl"`
}

type KubeConfig struct {
	LeaseName      string `yaml:"lease_name"`
	LeaseNamespace string `yaml:"lease_namespace"`
}

// DefaultConfig 返回选举默认配置。
func DefaultConfig() Config {
	return Config{
		PG:    PGConfig{LockID: 8080},
		Redis: RedisConfig{Key: "dash:leader", TTL: 15 * time.Second},
		Kube:  KubeConfig{LeaseName: "dash-leader"},
	}
}

// NewElector 根据配置创建对应的 LeaderElector。
// db 仅在 driver="pg" 时使用。
func NewElector(cfg Config, db *sql.DB) (LeaderElector, error) {
	switch cfg.Driver {
	case "pg":
		if db == nil {
			return nil, fmt.Errorf("pg election requires a database connection")
		}
		return newPGElector(db, cfg.PG), nil
	case "redis":
		return newRedisElector(cfg.Redis)
	case "kube":
		return newKubeElector(cfg.Kube), nil
	default:
		slog.Info("未配置 election driver，使用单实例模式")
		return newNoopElector(), nil
	}
}

// podID 返回当前 Pod 的唯一标识。
func podID() string {
	if name := os.Getenv("POD_NAME"); name != "" {
		return name
	}
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		return hostname
	}
	return fmt.Sprintf("pod-%d", os.Getpid())
}

// sleepCtx 在 ctx 取消前等待指定时长。
func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
