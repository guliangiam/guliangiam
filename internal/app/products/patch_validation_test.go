package products

import (
	"context"
	"testing"
	"time"

	productdomain "github.com/guliangiam/guliangiam/internal/domain/products"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TestPatchProduct_RevalidatesAfterMerge pins BE-C-14: a PATCH that pushes the
// merged aggregate past a domain invariant (negative price / negative stock)
// must be rejected before the write, exactly like the create path.
func TestPatchProduct_RevalidatesAfterMerge(t *testing.T) {
	negativePrice := -5.0
	negativeQuantity := -1
	okPrice := 12.5
	tests := []struct {
		name    string
		cmd     PatchProductCommand
		wantErr bool
	}{
		{name: "negative price rejected", cmd: PatchProductCommand{ID: "x-1", Price: &negativePrice, Revision: 1}, wantErr: true},
		{name: "negative quantity rejected", cmd: PatchProductCommand{ID: "x-1", Quantity: &negativeQuantity, Revision: 1}, wantErr: true},
		{name: "valid price accepted", cmd: PatchProductCommand{ID: "x-1", Price: &okPrice, Revision: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			repo := &mockProductRepo{products: []*productdomain.Product{{
				ID: "x-1", OrganizationID: "org-1", Name: "test", State: productdomain.StateActive,
				Price: 10, Revision: 1, CreatedAt: now, UpdatedAt: now,
			}}}
			svc := newSvc(repo)
			err := svc.PatchProduct(context.Background(), "org-1", tt.cmd)
			if tt.wantErr {
				if err == nil {
					t.Fatal("PatchProduct() error = nil, want domain validation rejection")
				}
				if code := apperrors.FromError(err).Code; code != apperrors.CodeUnprocessableEntity {
					t.Fatalf("error code = %s, want %s", code, apperrors.CodeUnprocessableEntity)
				}
				if repo.products[0].Revision != 1 {
					t.Errorf("Revision = %d, want 1 (no write on rejection)", repo.products[0].Revision)
				}
				return
			}
			if err != nil {
				t.Fatalf("PatchProduct() error = %v", err)
			}
		})
	}
}
