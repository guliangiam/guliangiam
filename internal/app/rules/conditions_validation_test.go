package rules

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/rules"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BE-D-22a：写路径必须按生效类型校验条件，任意 JSON 不得落库。

func conditionsTestService(repo *mockRuleRepo) Service {
	return Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "019test-rule-id"},
		UOW:   testUnitOfWork{ruleRepo: repo},
	}
}

func assertUnprocessable(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeUnprocessableEntity {
		t.Errorf("error code = %v, want %v", code, apperrors.CodeUnprocessableEntity)
	}
}

func TestService_CreateRule_RejectsEmptyConditionObject(t *testing.T) {
	repo := &mockRuleRepo{}
	svc := conditionsTestService(repo)

	err := svc.CreateRule(context.Background(), CreateRuleCommand{
		OrganizationID: "org-1",
		Name:           "Bad Rule",
		Scope:          "org",
		RuleType:       "IP",
		Conditions:     []map[string]any{{}},
		Action:         "Block",
	})
	assertUnprocessable(t, err)
	if len(repo.rules) != 0 {
		t.Errorf("expected no rule persisted, got %d", len(repo.rules))
	}
}

func TestService_CreateRule_RejectsInvalidIPValue(t *testing.T) {
	repo := &mockRuleRepo{}
	svc := conditionsTestService(repo)

	err := svc.CreateRule(context.Background(), CreateRuleCommand{
		OrganizationID: "org-1",
		Name:           "Bad IP",
		Scope:          "org",
		RuleType:       "IP",
		Conditions:     []map[string]any{{"field": "ip", "operator": "eq", "value": "not-an-ip"}},
		Action:         "Block",
	})
	assertUnprocessable(t, err)
}

func TestService_CreateRule_AcceptsIPRange(t *testing.T) {
	repo := &mockRuleRepo{}
	svc := conditionsTestService(repo)

	err := svc.CreateRule(context.Background(), CreateRuleCommand{
		OrganizationID: "org-1",
		Name:           "Range Rule",
		Scope:          "org",
		RuleType:       "IP",
		Conditions:     []map[string]any{{"field": "ip", "operator": "range", "value": "1.2.3.4-1.2.3.10"}},
		Action:         "Block",
	})
	if err != nil {
		t.Fatalf("CreateRule() error = %v", err)
	}
}

func TestService_CreateRule_RejectsInvalidIPRangeEndpoint(t *testing.T) {
	repo := &mockRuleRepo{}
	svc := conditionsTestService(repo)

	err := svc.CreateRule(context.Background(), CreateRuleCommand{
		OrganizationID: "org-1",
		Name:           "Bad Range",
		Scope:          "org",
		RuleType:       "IP",
		Conditions:     []map[string]any{{"field": "ip", "operator": "range", "value": "1.2.3.4-bad"}},
		Action:         "Block",
	})
	assertUnprocessable(t, err)
}

func TestService_CreateRule_AcceptsValidWAFPattern(t *testing.T) {
	repo := &mockRuleRepo{}
	svc := conditionsTestService(repo)

	err := svc.CreateRule(context.Background(), CreateRuleCommand{
		OrganizationID: "org-1",
		Name:           "WAF Rule",
		Scope:          "org",
		RuleType:       "WAF",
		Conditions:     []map[string]any{{"pattern": "union\\s+select", "target": "args"}},
		Action:         "Block",
	})
	if err != nil {
		t.Fatalf("CreateRule() error = %v", err)
	}
}

func TestService_CreateRule_RejectsEmptyWAFPattern(t *testing.T) {
	repo := &mockRuleRepo{}
	svc := conditionsTestService(repo)

	err := svc.CreateRule(context.Background(), CreateRuleCommand{
		OrganizationID: "org-1",
		Name:           "Empty WAF",
		Scope:          "org",
		RuleType:       "WAF",
		Conditions:     []map[string]any{{"pattern": "", "target": "args"}},
		Action:         "Block",
	})
	assertUnprocessable(t, err)
}

func TestService_PatchRule_RevalidatesAfterTypeChange(t *testing.T) {
	repo := &mockRuleRepo{
		rules: []*domain.Rule{
			{
				ID:             "rule-1",
				OrganizationID: "org-1",
				Name:           "Email Rule",
				RuleType:       "condition",
				Conditions:     []map[string]any{{"field": "email", "operator": "eq", "value": "a@b.c"}},
				Revision:       1,
			},
		},
	}
	svc := conditionsTestService(repo)

	// 改类型为 IP 但不改条件：既有 email 值不是合法 IP，必须拒绝。
	ipType := "IP"
	err := svc.PatchRule(context.Background(), "org-1", PatchRuleCommand{
		ID:       "rule-1",
		RuleType: &ipType,
		Revision: 1,
	})
	assertUnprocessable(t, err)
	if repo.rules[0].RuleType != "condition" {
		t.Errorf("RuleType = %q, want unchanged %q", repo.rules[0].RuleType, "condition")
	}
	if repo.rules[0].Revision != 1 {
		t.Errorf("Revision = %d, want unchanged 1", repo.rules[0].Revision)
	}
}

func TestService_PatchRule_RejectsInvalidNewConditions(t *testing.T) {
	repo := &mockRuleRepo{
		rules: []*domain.Rule{
			{
				ID:             "rule-1",
				OrganizationID: "org-1",
				Name:           "IP Rule",
				RuleType:       "IP",
				Conditions:     []map[string]any{{"field": "ip", "operator": "eq", "value": "1.2.3.4"}},
				Revision:       1,
			},
		},
	}
	svc := conditionsTestService(repo)

	err := svc.PatchRule(context.Background(), "org-1", PatchRuleCommand{
		ID:         "rule-1",
		Conditions: []map[string]any{{"field": "ip", "operator": "eq", "value": "999.1.1.1"}},
		Revision:   1,
	})
	assertUnprocessable(t, err)
}

func TestService_PatchRule_AcceptsValidNewConditions(t *testing.T) {
	repo := &mockRuleRepo{
		rules: []*domain.Rule{
			{
				ID:             "rule-1",
				OrganizationID: "org-1",
				Name:           "IP Rule",
				RuleType:       "IP",
				Conditions:     []map[string]any{{"field": "ip", "operator": "eq", "value": "1.2.3.4"}},
				Revision:       1,
			},
		},
	}
	svc := conditionsTestService(repo)

	err := svc.PatchRule(context.Background(), "org-1", PatchRuleCommand{
		ID:         "rule-1",
		Conditions: []map[string]any{{"field": "ip", "operator": "cidr", "value": "10.0.0.0/8"}},
		Revision:   1,
	})
	if err != nil {
		t.Fatalf("PatchRule() error = %v", err)
	}
	if repo.rules[0].Revision != 2 {
		t.Errorf("Revision = %d, want 2", repo.rules[0].Revision)
	}
}
