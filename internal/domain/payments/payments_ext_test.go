package payments

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ─── PayAttempt.Validate ────────────────────────────────────────

func TestPayAttempt_Validate_AllDefaultsSet(t *testing.T) {
	a := &PayAttempt{
		ID:             "a-1",
		OrganizationID: "org-1",
		PaymentID:      "p-1",
		Channel:        "alipay",
		AttemptType:    AttemptTypeCharge,
		OccurredAt:     time.Now(),
		// AttemptNo 和 State 留空 — Validate 会补默认
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if a.AttemptNo != 1 {
		t.Errorf("expected AttemptNo default 1, got %d", a.AttemptNo)
	}
	if a.State != AttemptStatePending {
		t.Errorf("expected State default pending, got %s", a.State)
	}
}

func TestPayAttempt_Validate_MissingID(t *testing.T) {
	a := &PayAttempt{OrganizationID: "o", PaymentID: "p", Channel: "alipay", AttemptType: AttemptTypeCharge}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestPayAttempt_Validate_InvalidAttemptType(t *testing.T) {
	a := &PayAttempt{ID: "a", OrganizationID: "o", PaymentID: "p", Channel: "c", AttemptType: "weird"}
	err := a.Validate()
	if err == nil {
		t.Fatal("expected error for invalid attemptType")
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeUnprocessableEntity || appErr.Meta["field"] != "attemptType" {
		t.Errorf("got code=%q meta=%v, want 422 with field=attemptType", appErr.Code, appErr.Meta)
	}
}

func TestPayAttempt_Validate_InvalidState(t *testing.T) {
	a := &PayAttempt{ID: "a", OrganizationID: "o", PaymentID: "p", Channel: "c", AttemptType: AttemptTypeCharge, State: "weird"}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for invalid state")
	}
}

func TestPayAttempt_Validate_AllAttemptTypesAccepted(t *testing.T) {
	for _, at := range []string{AttemptTypeCharge, AttemptTypeQuery, AttemptTypeRefund,
		AttemptTypeQueryRefund, AttemptTypeClose, AttemptTypeCallback} {
		a := &PayAttempt{ID: "a", OrganizationID: "o", PaymentID: "p", Channel: "c", AttemptType: at}
		if err := a.Validate(); err != nil {
			t.Errorf("attemptType %s rejected: %v", at, err)
		}
	}
}

func TestPayAttempt_Validate_AllStatesAccepted(t *testing.T) {
	for _, s := range []AttemptState{AttemptStatePending, AttemptStateSucceeded, AttemptStateFailed, AttemptStateUnknown, AttemptStateRejected} {
		a := &PayAttempt{ID: "a", OrganizationID: "o", PaymentID: "p", Channel: "c", AttemptType: AttemptTypeCharge, State: s}
		if err := a.Validate(); err != nil {
			t.Errorf("state %s rejected: %v", s, err)
		}
	}
}

// ─── ChannelConfig ─────────────────────────────────────────────

func TestChannelConfig_Validate_Defaults(t *testing.T) {
	c := &PayChannelConfig{
		ID:                "c-1",
		OrganizationID:    "org-1",
		Channel:           "alipay",
		ConfigFingerprint: "fp-abc",
		// Environment 和 Status 留空 — 默认 production + draft
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if c.Environment != "production" {
		t.Errorf("expected env default production, got %s", c.Environment)
	}
	if c.Status != ChannelConfigStatusDraft {
		t.Errorf("expected status default draft, got %s", c.Status)
	}
	if c.VersionNo != 1 {
		t.Errorf("expected VersionNo default 1, got %d", c.VersionNo)
	}
}

func TestChannelConfig_Validate_InvalidEnvironment(t *testing.T) {
	c := &PayChannelConfig{ID: "c", OrganizationID: "o", Channel: "alipay", Environment: "staging", ConfigFingerprint: "fp"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for invalid environment")
	}
}

func TestChannelConfig_Validate_SandboxAccepted(t *testing.T) {
	c := &PayChannelConfig{ID: "c", OrganizationID: "o", Channel: "alipay", Environment: "sandbox", ConfigFingerprint: "fp"}
	if err := c.Validate(); err != nil {
		t.Fatalf("sandbox should be accepted: %v", err)
	}
}

func TestChannelConfig_Validate_InvalidStatus(t *testing.T) {
	c := &PayChannelConfig{ID: "c", OrganizationID: "o", Channel: "alipay", ConfigFingerprint: "fp", Status: "archived"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestChannelConfig_Validate_MissingFingerprint(t *testing.T) {
	c := &PayChannelConfig{ID: "c", OrganizationID: "o", Channel: "alipay", Status: ChannelConfigStatusDraft}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for missing fingerprint")
	}
}

// ─── ChannelConfig 四状态机 ────────────────────────────────────

func TestChannelConfig_TransitionTo_DraftToTesting(t *testing.T) {
	c := &PayChannelConfig{Status: ChannelConfigStatusDraft}
	if err := c.TransitionTo(ChannelConfigStatusTesting); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if c.Status != ChannelConfigStatusTesting {
		t.Errorf("expected testing, got %s", c.Status)
	}
}

func TestChannelConfig_TransitionTo_TestingToActive_SetsTimestamp(t *testing.T) {
	c := &PayChannelConfig{Status: ChannelConfigStatusTesting}
	before := time.Now()
	if err := c.TransitionTo(ChannelConfigStatusActive); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	after := time.Now()
	if c.ActivatedAt == nil {
		t.Fatal("ActivatedAt should be set")
	}
	if c.ActivatedAt.Before(before) || c.ActivatedAt.After(after) {
		t.Errorf("ActivatedAt out of range")
	}
}

func TestChannelConfig_TransitionTo_ActiveToDisabled_SetsTimestamp(t *testing.T) {
	c := &PayChannelConfig{Status: ChannelConfigStatusActive}
	before := time.Now()
	if err := c.TransitionTo(ChannelConfigStatusDisabled); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	after := time.Now()
	if c.DisabledAt == nil {
		t.Fatal("DisabledAt should be set")
	}
	if c.DisabledAt.Before(before) || c.DisabledAt.After(after) {
		t.Errorf("DisabledAt out of range")
	}
}

func TestChannelConfig_TransitionTo_IllegalPath_DraftToActive(t *testing.T) {
	c := &PayChannelConfig{Status: ChannelConfigStatusDraft}
	if err := c.TransitionTo(ChannelConfigStatusActive); err == nil {
		t.Fatal("draft → active is illegal, should error")
	}
}

func TestChannelConfig_TransitionTo_Idempotent(t *testing.T) {
	c := &PayChannelConfig{Status: ChannelConfigStatusActive}
	if err := c.TransitionTo(ChannelConfigStatusActive); err != nil {
		t.Fatalf("self-transition should be no-op: %v", err)
	}
}

func TestChannelConfig_TransitionTo_AllValidPaths(t *testing.T) {
	type tc struct{ from, to string }
	valid := []tc{
		{ChannelConfigStatusDraft, ChannelConfigStatusTesting},
		{ChannelConfigStatusDraft, ChannelConfigStatusDisabled},
		{ChannelConfigStatusTesting, ChannelConfigStatusActive},
		{ChannelConfigStatusTesting, ChannelConfigStatusDraft},
		{ChannelConfigStatusTesting, ChannelConfigStatusDisabled},
		{ChannelConfigStatusActive, ChannelConfigStatusDisabled},
		{ChannelConfigStatusDisabled, ChannelConfigStatusDraft},
	}
	for _, p := range valid {
		c := &PayChannelConfig{Status: p.from}
		if err := c.TransitionTo(p.to); err != nil {
			t.Errorf("valid transition %s → %s rejected: %v", p.from, p.to, err)
		}
	}
}

func TestChannelConfig_IsUsable_ActiveNotDeleted(t *testing.T) {
	c := &PayChannelConfig{Status: ChannelConfigStatusActive}
	if !c.IsUsable() {
		t.Error("active + nil DeletedAt should be usable")
	}
}

func TestChannelConfig_IsUsable_Disabled(t *testing.T) {
	c := &PayChannelConfig{Status: ChannelConfigStatusDisabled}
	if c.IsUsable() {
		t.Error("disabled should not be usable")
	}
}

func TestChannelConfig_IsUsable_ActiveButDeleted(t *testing.T) {
	tm := time.Now()
	c := &PayChannelConfig{Status: ChannelConfigStatusActive, DeletedAt: &tm}
	if c.IsUsable() {
		t.Error("deleted should not be usable")
	}
}

// ─── ReconBatch.Validate ────────────────────────────────────────

func TestReconBatch_Validate_Defaults(t *testing.T) {
	b := &ReconBatch{
		ID:             "rb-1",
		OrganizationID: "org-1",
		Channel:        "alipay",
		BatchDate:      time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC),
		// Status 留空 → pending
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if b.Status != string(ReconBatchStatusPending) {
		t.Errorf("expected pending, got %s", b.Status)
	}
}

func TestReconBatch_Validate_AllStatusAccepted(t *testing.T) {
	for _, s := range []ReconBatchStatus{ReconBatchStatusPending, ReconBatchStatusProcessing, ReconBatchStatusCompleted, ReconBatchStatusFailed} {
		b := &ReconBatch{ID: "rb", OrganizationID: "o", Channel: "c", BatchDate: time.Now(), Status: string(s)}
		if err := b.Validate(); err != nil {
			t.Errorf("status %s rejected: %v", s, err)
		}
	}
}

func TestReconBatch_Validate_MissingBatchDate(t *testing.T) {
	b := &ReconBatch{ID: "rb", OrganizationID: "o", Channel: "c"}
	if err := b.Validate(); err == nil {
		t.Fatal("expected error for zero BatchDate")
	}
}

func TestReconBatch_Validate_InvalidStatus(t *testing.T) {
	b := &ReconBatch{ID: "rb", OrganizationID: "o", Channel: "c", BatchDate: time.Now(), Status: "weird"}
	if err := b.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

// ─── ReconItem.Validate ─────────────────────────────────────────

func TestReconItem_Validate_AllDiffTypes(t *testing.T) {
	for _, dt := range []DiffType{DiffMissingLocal, DiffMissingChannel, DiffAmountMismatch, DiffStateMismatch, DiffRefundMismatch} {
		i := &ReconItem{ID: "ri", BatchID: "rb", DiffType: string(dt)}
		if err := i.Validate(); err != nil {
			t.Errorf("diffType %s rejected: %v", dt, err)
		}
	}
}

func TestReconItem_Validate_MissingID(t *testing.T) {
	i := &ReconItem{BatchID: "rb", DiffType: string(DiffMissingLocal)}
	if err := i.Validate(); err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestReconItem_Validate_InvalidDiffType(t *testing.T) {
	i := &ReconItem{ID: "ri", BatchID: "rb", DiffType: "weird"}
	if err := i.Validate(); err == nil {
		t.Fatal("expected error for invalid diffType")
	}
}

// ─── ChannelError ──────────────────────────────────────────────

func TestChannelError_Error_UsesMessage(t *testing.T) {
	e := &ChannelError{Code: "INTERNAL", Message: "provider down", Category: ErrorCategoryRetryable}
	if got := e.Error(); got != "provider down" {
		t.Errorf("expected provider down, got %q", got)
	}
}

func TestChannelError_Error_FallsBackToCode(t *testing.T) {
	e := &ChannelError{Code: "TIMEOUT", Category: ErrorCategoryRetryable}
	if got := e.Error(); got != "TIMEOUT" {
		t.Errorf("expected TIMEOUT, got %q", got)
	}
}

// ─── CallbackEvent 结构完整性 ──────────────────────────────────

func TestCallbackEvent_FieldsPopulated(t *testing.T) {
	amt := 99.99
	ev := &CallbackEvent{
		EventKey:       "trade_success",
		OutTradeNo:     "out-123",
		ChannelOrderNo: "ch-456",
		RawStatus:      "TRADE_SUCCESS",
		Amount:         &amt,
		State:          AttemptStateSucceeded,
		Raw:            map[string]any{"sign": "abc"},
	}
	if ev.EventKey != "trade_success" || ev.OutTradeNo != "out-123" || *ev.Amount != 99.99 {
		t.Error("CallbackEvent fields not correctly populated")
	}
}

// ─── Payments 主状态机 IsValid / CanTransitionTo ───────────────

func TestPaymentsState_IsValid_AllSix(t *testing.T) {
	for _, s := range AllPaymentsState {
		if !IsValid(string(s)) {
			t.Errorf("state %s should be valid", s)
		}
	}
}

func TestPaymentsState_IsValid_EmptyAndUnknown(t *testing.T) {
	if IsValid("") {
		t.Error("empty string should be invalid")
	}
	if IsValid("pending") {
		t.Error("unknown state should be invalid")
	}
}

func TestPaymentsState_CanTransitionTo_ValidPaths(t *testing.T) {
	tests := []struct{ from, to PaymentsState }{
		{PaymentsStateCreated_State, PaymentsStateProcessing_State},
		{PaymentsStateCreated_State, PaymentsStateCancelled_State},
		{PaymentsStateProcessing_State, PaymentsStateSucceeded_State},
		{PaymentsStateProcessing_State, PaymentsStateFailed_State},
		{PaymentsStateSucceeded_State, PaymentsStateRefunded_State},
	}
	for _, tt := range tests {
		if !CanTransitionTo(tt.from, tt.to) {
			t.Errorf("%s → %s should be allowed", tt.from, tt.to)
		}
	}
}

func TestPaymentsState_CanTransitionTo_IllegalPaths(t *testing.T) {
	tests := []struct{ from, to PaymentsState }{
		{PaymentsStateCreated_State, PaymentsStateSucceeded_State},
		{PaymentsStateProcessing_State, PaymentsStateCancelled_State},
		{PaymentsStateFailed_State, PaymentsStateCreated_State},
		{PaymentsStateRefunded_State, PaymentsStateSucceeded_State},
		{PaymentsStateSucceeded_State, PaymentsStateCreated_State},
	}
	for _, tt := range tests {
		if CanTransitionTo(tt.from, tt.to) {
			t.Errorf("%s → %s should NOT be allowed", tt.from, tt.to)
		}
	}
}
