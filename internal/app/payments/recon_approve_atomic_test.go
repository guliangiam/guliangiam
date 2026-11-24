package payments

import (
	"context"
	"errors"
	"testing"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
)

// ─── BE-C-18：ApproveItem 优先走原子能力，审计失败不得吞掉 ─────────────────

type atomicReconRepo struct {
	domain.ReconRepo
	approveCalls int
	atomicCalls  int
	atomicErr    error
	gotRecord    *domainrecords.Record
	gotEvent     *domainoutbox.OutboxEvent
}

func (r *atomicReconRepo) ApproveItemForOrganization(ctx context.Context, itemID, organizationID, approver string) (string, error) {
	r.approveCalls++
	return "resolver-1", nil
}

func (r *atomicReconRepo) ApproveItemWithAudit(ctx context.Context, itemID, organizationID, approver string, buildAudit func(string) (*domainrecords.Record, *domainoutbox.OutboxEvent, error)) (string, error) {
	r.atomicCalls++
	if r.atomicErr != nil {
		return "", r.atomicErr
	}
	record, event, err := buildAudit("resolver-1")
	if err != nil {
		return "", err
	}
	r.gotRecord, r.gotEvent = record, event
	return "resolver-1", nil
}

func TestReconService_ApproveItem_PrefersAtomicAuditor(t *testing.T) {
	repo := &atomicReconRepo{}
	svc := NewReconService(repo, nil, fixedUUID{s: "audit-1"})

	if err := svc.ApproveItem(context.Background(), "org-1", "item-1", "approver-1"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.atomicCalls != 1 || repo.approveCalls != 0 {
		t.Fatalf("atomic path must be used exclusively: atomic=%d legacy=%d", repo.atomicCalls, repo.approveCalls)
	}
	if repo.gotRecord == nil || repo.gotEvent == nil {
		t.Fatal("audit record and outbox event must be built inside the approval transaction")
	}
	if repo.gotRecord.Action != "payment.recon.item.approved" || repo.gotEvent.EventType != "payment.recon.item.approved" {
		t.Error("audit action/event type mismatch")
	}
	if repo.gotRecord.Metadata["originalOperator"] != "resolver-1" {
		t.Errorf("originalOperator = %v, want resolver-1", repo.gotRecord.Metadata["originalOperator"])
	}
	if repo.gotRecord.ID != repo.gotEvent.ID {
		t.Error("record and outbox event should share the same audit id")
	}
}

func TestReconService_ApproveItem_AtomicAuditFailureFailsApproval(t *testing.T) {
	repo := &atomicReconRepo{atomicErr: errors.New("audit insert failed")}
	svc := NewReconService(repo, nil, fixedUUID{s: "audit-1"})

	if err := svc.ApproveItem(context.Background(), "org-1", "item-1", "approver-1"); err == nil {
		t.Fatal("approval must fail when the atomic audit write fails (no silent gap)")
	}
}

// legacyReconRepo 不实现原子能力 → 回退旧两步路径（MySQL 镜像兼容）。
type legacyReconRepo struct {
	domain.ReconRepo
	approveCalls int
}

func (r *legacyReconRepo) ApproveItemForOrganization(ctx context.Context, itemID, organizationID, approver string) (string, error) {
	r.approveCalls++
	return "resolver-1", nil
}

func TestReconService_ApproveItem_LegacyFallbackStillWorks(t *testing.T) {
	repo := &legacyReconRepo{}
	svc := NewReconService(repo, nil, fixedUUID{s: "audit-1"})

	if err := svc.ApproveItem(context.Background(), "org-1", "item-1", "approver-1"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.approveCalls != 1 {
		t.Fatalf("legacy approve calls = %d, want 1", repo.approveCalls)
	}
}
