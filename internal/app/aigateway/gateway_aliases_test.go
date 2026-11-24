package aigateway

import (
	"context"
	"encoding/json"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
)

// emptyAliasesRepo answers an empty list, which is the case that used to leak
// a nil slice through GatewayService.ListAliases.
type emptyAliasesRepo struct{}

func (emptyAliasesRepo) GetByID(context.Context, string) (*domain.LlmModelAlias, error) {
	return nil, nil
}
func (emptyAliasesRepo) GetByName(context.Context, string, string) (*domain.LlmModelAlias, error) {
	return nil, nil
}
func (emptyAliasesRepo) List(context.Context, string) ([]*domain.LlmModelAlias, error) {
	return nil, nil
}
func (emptyAliasesRepo) ListAll(context.Context) ([]*domain.LlmModelAlias, error) {
	return nil, nil
}
func (emptyAliasesRepo) Create(context.Context, *domain.LlmModelAlias) error { return nil }
func (emptyAliasesRepo) Update(context.Context, *domain.LlmModelAlias) error { return nil }
func (emptyAliasesRepo) Delete(context.Context, string) error                { return nil }

func TestGatewayService_ListAliases_EmptyIsNonNil(t *testing.T) {
	svc := &GatewayService{Aliases: emptyAliasesRepo{}}
	aliases, err := svc.ListAliases(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("ListAliases() error = %v", err)
	}
	if aliases == nil {
		t.Fatal("ListAliases() returned nil, want empty slice")
	}
	raw, err := json.Marshal(aliases)
	if err != nil {
		t.Fatalf("marshal aliases: %v", err)
	}
	if string(raw) != "[]" {
		t.Fatalf("ListAliases() JSON = %s, want []", raw)
	}
}
