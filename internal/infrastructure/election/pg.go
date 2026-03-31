package election

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

type pgElector struct {
	db     *sql.DB
	lockID int64
	leader atomic.Bool
}

func newPGElector(db *sql.DB, cfg PGConfig) *pgElector {
	lockID := cfg.LockID
	if lockID == 0 {
		lockID = 8080
	}
	return &pgElector{db: db, lockID: lockID}
}

func (e *pgElector) Run(ctx context.Context, cb LeaderCallbacks) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := e.tryLead(ctx, cb); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("PG leader election 出错，重试中", "error", err)
		}

		sleepCtx(ctx, 5*time.Second)
	}
}

func (e *pgElector) IsLeader() bool {
	return e.leader.Load()
}

// tryLead 尝试获取 advisory lock 并持有直到连接断开或 ctx 取消。
func (e *pgElector) tryLead(ctx context.Context, cb LeaderCallbacks) error {
	conn, err := e.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("获取连接失败: %w", err)
	}
	defer conn.Close()

	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", e.lockID).Scan(&acquired); err != nil {
		return fmt.Errorf("尝试获取锁失败: %w", err)
	}

	if !acquired {
		return nil
	}

	// 退出时释放锁（用 Background 防止原 ctx 已取消）
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", e.lockID)
	}()

	e.leader.Store(true)
	slog.Info("获得 PG advisory lock，当选 Leader", "lock_id", e.lockID)
	cb.OnStartedLeading(ctx)

	defer func() {
		e.leader.Store(false)
		slog.Info("释放 PG advisory lock，不再是 Leader", "lock_id", e.lockID)
		cb.OnStoppedLeading()
	}()

	// 持续 ping 保活，检测连接是否断开
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			if err := conn.PingContext(ctx); err != nil {
				return fmt.Errorf("连接断开: %w", err)
			}
		}
	}
}
