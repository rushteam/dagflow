package election

import (
	"context"
	"sync/atomic"
)

type noopElector struct {
	leader atomic.Bool
}

func newNoopElector() *noopElector {
	return &noopElector{}
}

func (e *noopElector) Run(ctx context.Context, cb LeaderCallbacks) error {
	e.leader.Store(true)
	cb.OnStartedLeading(ctx)
	<-ctx.Done()
	e.leader.Store(false)
	cb.OnStoppedLeading()
	return ctx.Err()
}

func (e *noopElector) IsLeader() bool {
	return e.leader.Load()
}
