package ldapsync

import "context"

// LockProvider is the port for a cross-process mutual-exclusion primitive.
//
// App layer code must stay dialect-agnostic (STD 3.3.1), so the concrete
// implementation (PostgreSQL advisory lock today; MySQL GET_LOCK or an app-level
// lock table later) lives in the infra layer and is injected here.
type LockProvider interface {
	// TryLock attempts to acquire the named lock without blocking.
	// It returns a release function and true when the lock was acquired.
	// When the lock is already held elsewhere it returns (nil, false, nil).
	TryLock(ctx context.Context, name string) (release func(context.Context) error, acquired bool, err error)
}

// noopLockProvider is used when no lock provider is wired (e.g. tests, or a
// deliberately single-replica deployment). It always grants the lock.
type noopLockProvider struct{}

func (noopLockProvider) TryLock(_ context.Context, _ string) (func(context.Context) error, bool, error) {
	return func(context.Context) error { return nil }, true, nil
}
