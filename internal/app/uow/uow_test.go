package uow

import (
	"context"
	"testing"

	domainuow "github.com/guliangiam/guliangiam/internal/domain/uow"
)

type runnerUoW struct{}

func (runnerUoW) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx domainuow.UnitOfWork) error) error {
	return fn(ctx, nil)
}

type plainUoW struct{}

func TestEnsureTxRunnerReturnsRunnerWhenImplemented(t *testing.T) {
	runner, ok := EnsureTxRunner(runnerUoW{})
	if !ok || runner == nil {
		t.Fatalf("ok = %v, runner = %v", ok, runner)
	}
	if err := runner.WithinTransaction(context.Background(), func(context.Context, domainuow.UnitOfWork) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureTxRunnerReportsMissingCapability(t *testing.T) {
	runner, ok := EnsureTxRunner(plainUoW{})
	if ok || runner != nil {
		t.Fatalf("ok = %v, runner = %v; want false/nil", ok, runner)
	}
}
