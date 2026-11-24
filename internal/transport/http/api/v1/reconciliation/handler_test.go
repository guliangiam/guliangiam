package reconciliation

import (
	"context"
	"net/http"
	"testing"

	appreconciliation "github.com/guliangiam/guliangiam/internal/app/reconciliation"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

type recordingService struct {
	got appreconciliation.ListQuery
}

func (s *recordingService) List(_ context.Context, query appreconciliation.ListQuery) (*appreconciliation.ListResponse, error) {
	s.got = query
	return &appreconciliation.ListResponse{Page: appreconciliation.PageMeta{Page: query.Page, PageSize: query.PageSize}}, nil
}

// The report store is tenant-scoped: the handler must take the organization
// from the validated session context, never from the request query.
func TestList_TakesOrganizationFromSessionNotRequest(t *testing.T) {
	svc := &recordingService{}
	handler := NewHandler(svc)
	ctx := authtransport.WithSessionContext(context.Background(), authtransport.SessionContext{
		OrganizationID: "org-from-session",
	})

	resp, err := handler.List(ctx, ListRequest{Page: 2, PageSize: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if svc.got.OrganizationID != "org-from-session" {
		t.Errorf("organization = %q, want session organization", svc.got.OrganizationID)
	}
	if svc.got.Page != 2 || svc.got.PageSize != 10 {
		t.Errorf("paging = (%d,%d), want (2,10)", svc.got.Page, svc.got.PageSize)
	}
	if resp == nil || resp.Page.Page != 2 || resp.Page.PageSize != 10 {
		t.Errorf("response = %#v, want page 2 size 10", resp)
	}
}

func TestList_WithoutSessionOrganizationPassesEmptyScope(t *testing.T) {
	svc := &recordingService{}
	handler := NewHandler(svc)

	if _, err := handler.List(context.Background(), ListRequest{Page: 1, PageSize: 20}); err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if svc.got.OrganizationID != "" {
		t.Errorf("organization = %q, want empty (the repository must reject it)", svc.got.OrganizationID)
	}
}

func TestList_NilServiceIsUnavailable(t *testing.T) {
	handler := NewHandler(nil)

	_, err := handler.List(context.Background(), ListRequest{Page: 1, PageSize: 20})
	if err == nil {
		t.Fatal("expected unavailable error, got nil")
	}
	if got := apperrors.FromError(err).HTTPStatus(); got != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", got, http.StatusServiceUnavailable)
	}
}
