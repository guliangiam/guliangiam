package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type consistencyRepositoryStub struct {
	registrations []domain.RegistrationConsistency
	details       map[domain.Family][]domain.DetailConsistency
	err           error
}

func (r consistencyRepositoryStub) ListConsistencyRegistrations(context.Context) ([]domain.RegistrationConsistency, error) {
	return r.registrations, r.err
}

func (r consistencyRepositoryStub) ListConsistencyDetails(_ context.Context, family domain.Family) ([]domain.DetailConsistency, error) {
	return r.details[family], r.err
}

type consistencyUUIDGenerator struct{ next int }

func (g *consistencyUUIDGenerator) NewString() (string, error) {
	g.next++
	return fmt.Sprintf("01900000-0000-7000-8000-%012d", g.next), nil
}

type consistencyFailingOutbox struct {
	items []*domainoutbox.OutboxEvent
}

func (r *consistencyFailingOutbox) Enqueue(_ context.Context, item *domainoutbox.OutboxEvent) error {
	r.items = append(r.items, item)
	return errors.New("enqueue failed")
}

func (r *consistencyFailingOutbox) EnqueueBatch(_ context.Context, items []*domainoutbox.OutboxEvent) error {
	r.items = append(r.items, items...)
	return errors.New("enqueue failed")
}

func (r *consistencyFailingOutbox) snapshot() func() {
	items := append([]*domainoutbox.OutboxEvent(nil), r.items...)
	return func() { r.items = items }
}

func cleanConsistencyInput() (registrationsOut []domain.RegistrationConsistency, detailsOut map[domain.Family][]domain.DetailConsistency) {
	registrations := []domain.RegistrationConsistency{
		{ID: "identity-1", OrganizationID: "org-1", Family: domain.FamilyIdentity, Classified: true, Status: domain.StatusActive, Enabled: true},
		{
			ID: "ai-registration", OrganizationID: "org-1", Name: "OpenAI", DisplayName: "OpenAI Display",
			Family: domain.FamilyAI, Kind: "openai", Classified: true, Status: domain.StatusActive,
			DetailID: "ai-1", Enabled: true,
		},
		{
			ID: "payment-registration", OrganizationID: "org-1", Name: "Alipay", Family: domain.FamilyPayment,
			Kind: "alipay", Variant: "production", Classified: true, Status: domain.StatusActive,
			DetailID: "payment-2", Enabled: true,
		},
		{
			ID: "notification-registration", OrganizationID: "org-1", Name: "Mail", Family: domain.FamilyNotification,
			Kind: "email", Classified: true, Status: domain.StatusDisabled, DetailID: "notification-1",
		},
	}
	details := map[domain.Family][]domain.DetailConsistency{
		domain.FamilyAI: {
			{ID: "ai-1", ProviderID: "ai-registration", OrganizationID: "org-1", Name: "OpenAI", DisplayName: "OpenAI Display", Kind: "openai", Status: "active", Enabled: true},
		},
		domain.FamilyPayment: {
			{ID: "payment-1", ProviderID: "payment-registration", OrganizationID: "org-1", Kind: "alipay", Variant: "production", Status: "disabled"},
			{ID: "payment-2", ProviderID: "payment-registration", OrganizationID: "org-1", Kind: "alipay", Variant: "production", Status: "active", Enabled: true},
		},
		domain.FamilyNotification: {
			{ID: "notification-1", ProviderID: "notification-registration", OrganizationID: "org-1", Name: "Mail", Kind: "email", Status: "disabled"},
		},
	}
	return registrations, details
}

func TestConsistencyWorkerCleanRunWritesCompletedEvidence(t *testing.T) {
	registrations, details := cleanConsistencyInput()
	outbox := &providerOutboxRepo{}
	now := time.Date(2026, time.October, 27, 10, 0, 0, 0, time.UTC)
	worker := NewConsistencyWorker(
		consistencyRepositoryStub{registrations: registrations, details: details},
		testUnitOfWork{outboxRepo: outbox}, fakeClock{fixedTime: now}, &consistencyUUIDGenerator{}, time.Hour, nil,
	)

	report, err := worker.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 4, report.RegistrationCount)
	assert.Equal(t, 4, report.DetailCount)
	assert.Zero(t, report.DifferenceCount)
	assert.NotEmpty(t, report.Digest)
	require.Len(t, outbox.items, 1)
	event := outbox.items[0]
	assert.Equal(t, ConsistencyAggregateType, event.AggregateType)
	assert.Equal(t, ConsistencyCompletedEvent, event.EventType)
	assert.True(t, platformuuid.IsValid(event.ID), "outbox event id must be a UUID: %s", event.ID)
	assert.True(t, platformuuid.IsValid(event.AggregateID), "aggregate id must be a UUID: %s", event.AggregateID)
	assert.Equal(t, report.RunID, event.AggregateID)
	assert.NotContains(t, string(event.Payload), `"differences"`)
	assert.NoError(t, NewConsistencyOutboxHandler(nil).Handle(context.Background(), &platformoutbox.OutboxItem{EventType: event.EventType, Payload: event.Payload}))
}

func TestConsistencyDigestIsStableAcrossInputOrder(t *testing.T) {
	registrations, details := cleanConsistencyInput()
	first := evaluateConsistency(registrations, details)
	for left, right := 0, len(registrations)-1; left < right; left, right = left+1, right-1 {
		registrations[left], registrations[right] = registrations[right], registrations[left]
	}
	for _, family := range []domain.Family{domain.FamilyAI, domain.FamilyPayment, domain.FamilyNotification} {
		items := details[family]
		for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
			items[left], items[right] = items[right], items[left]
		}
	}
	second := evaluateConsistency(registrations, details)

	assert.Equal(t, first, second)
	assert.Equal(t, consistencyDigest(4, 4, first), consistencyDigest(4, 4, second))
}

func TestConsistencyWorkerReportsDifferencesWithoutMutatingInput(t *testing.T) {
	deletedAt := time.Date(2026, time.October, 27, 9, 0, 0, 0, time.UTC)
	registrations := []domain.RegistrationConsistency{
		{ID: "identity-1", OrganizationID: "org-1", Family: domain.FamilyIdentity},
		{
			ID: "ai-registration", OrganizationID: "org-1", Name: "Expected", DisplayName: "Expected Display",
			Family: domain.FamilyAI, Kind: "openai", Classified: true, Status: domain.StatusDraft,
			DetailID: "ai-1", Enabled: true, DeletedAt: &deletedAt,
		},
		{
			ID: "payment-registration", OrganizationID: "org-1", Family: domain.FamilyPayment,
			Kind: "alipay", Variant: "production", Classified: true, Status: domain.StatusActive,
			DetailID: "payment-2", Enabled: true,
		},
		{
			ID: "notification-registration", OrganizationID: "org-1", Name: "Mail", Family: domain.FamilyNotification,
			Kind: "email", Classified: true, Status: domain.StatusActive, DetailID: "notification-1",
			Enabled: true, DeletedAt: &deletedAt,
		},
	}
	details := map[domain.Family][]domain.DetailConsistency{
		domain.FamilyAI: {
			{ID: "ai-1", ProviderID: "ai-registration", OrganizationID: "org-1", Name: "Actual", DisplayName: "Actual Display", Kind: "openai", Status: "active", Enabled: true},
			{ID: "ai-unanchored", OrganizationID: "org-1", Kind: "openai", Status: "testing"},
		},
		domain.FamilyPayment: {
			{ID: "payment-1", ProviderID: "payment-registration", OrganizationID: "org-1", Kind: "alipay", Variant: "production", Status: "disabled"},
			{ID: "payment-unknown", ProviderID: "missing-registration", OrganizationID: "org-1", Kind: "alipay", Variant: "production", Status: "active"},
		},
		domain.FamilyNotification: {
			{ID: "notification-1", ProviderID: "notification-registration", OrganizationID: "org-1", Name: "Mail", Kind: "email", Status: "active", Enabled: true},
		},
	}
	originalRegistrations := append([]domain.RegistrationConsistency(nil), registrations...)
	originalDetails := map[domain.Family][]domain.DetailConsistency{}
	for family, items := range details {
		originalDetails[family] = append([]domain.DetailConsistency(nil), items...)
	}
	outbox := &providerOutboxRepo{}
	worker := NewConsistencyWorker(
		consistencyRepositoryStub{registrations: registrations, details: details},
		testUnitOfWork{outboxRepo: outbox}, fakeClock{fixedTime: deletedAt}, &consistencyUUIDGenerator{}, time.Hour, nil,
	)

	report, err := worker.RunOnce(context.Background())
	require.NoError(t, err)
	codes := make([]string, 0, len(report.Differences))
	for _, difference := range report.Differences {
		codes = append(codes, difference.Code)
	}
	for _, expected := range []string{
		"registration_unclassified", "registration_enabled_mismatch", "name_mismatch", "display_name_mismatch", "status_mismatch",
		"deleted_at_mismatch", "detail_not_found", "deleted_detail_routable", "registration_not_found",
	} {
		assert.Contains(t, codes, expected)
	}
	assert.Len(t, outbox.items, report.DifferenceCount+1)
	assert.Equal(t, ConsistencyCompletedEvent, outbox.items[len(outbox.items)-1].EventType)
	assert.Equal(t, originalRegistrations, registrations)
	assert.Equal(t, originalDetails, details)

	seenAggregateIDs := make(map[string]struct{}, len(outbox.items))
	for _, event := range outbox.items {
		require.True(t, platformuuid.IsValid(event.ID), "outbox event id must be a UUID: %s", event.ID)
		require.True(t, platformuuid.IsValid(event.AggregateID), "aggregate id must be a UUID: %s", event.AggregateID)
		_, duplicate := seenAggregateIDs[event.AggregateID]
		require.False(t, duplicate, "aggregate ids must be unique within one run: %s", event.AggregateID)
		seenAggregateIDs[event.AggregateID] = struct{}{}
	}
	for _, event := range outbox.items[:report.DifferenceCount] {
		assert.Equal(t, ConsistencyDifferenceEvent, event.EventType)
		assert.Equal(t, ConsistencyAggregateType, event.AggregateType)
	}
}

func TestConsistencyWorkerRollsBackEvidenceWhenEnqueueFails(t *testing.T) {
	registrations, details := cleanConsistencyInput()
	outbox := &consistencyFailingOutbox{}
	worker := NewConsistencyWorker(
		consistencyRepositoryStub{registrations: registrations, details: details},
		testUnitOfWork{outboxRepo: outbox}, fakeClock{fixedTime: time.Now()}, &consistencyUUIDGenerator{}, time.Hour, nil,
	)

	_, err := worker.RunOnce(context.Background())
	require.Error(t, err)
	assert.Empty(t, outbox.items)
}

func TestConsistencyOrganizationMismatchStaysGlobal(t *testing.T) {
	registration := domain.RegistrationConsistency{
		ID: "ai-registration", OrganizationID: "org-1", Family: domain.FamilyAI,
		Kind: "openai", Classified: true, Status: domain.StatusActive, DetailID: "ai-1", Enabled: true,
	}
	detail := domain.DetailConsistency{
		ID: "ai-1", ProviderID: registration.ID, OrganizationID: "org-2",
		Kind: "openai", Status: "active", Enabled: true,
	}
	differences := evaluateConsistency(
		[]domain.RegistrationConsistency{registration},
		map[domain.Family][]domain.DetailConsistency{domain.FamilyAI: {detail}},
	)
	require.Len(t, differences, 1)
	assert.Equal(t, "organization_mismatch", differences[0].Code)
	assert.Empty(t, differences[0].OrganizationID)
	assert.Equal(t, "org-1", differences[0].Expected)
	assert.Equal(t, "org-2", differences[0].Actual)
}

func TestConsistencyOutboxHandlerRejectsMalformedPayload(t *testing.T) {
	handler := NewConsistencyOutboxHandler(nil)
	err := handler.Handle(context.Background(), &platformoutbox.OutboxItem{
		EventType: ConsistencyDifferenceEvent,
		Payload:   []byte(`{"runId":"run-1"}`),
	})
	require.Error(t, err)

	payload, err := json.Marshal(map[string]any{
		"runId": "run-1", "digest": "abc",
		"difference": map[string]any{"organizationId": "org-1", "providerId": "provider-1", "family": "ai", "code": "status_mismatch"},
	})
	require.NoError(t, err)
	assert.NoError(t, handler.Handle(context.Background(), &platformoutbox.OutboxItem{
		EventType: ConsistencyDifferenceEvent,
		Payload:   payload,
	}))
}
