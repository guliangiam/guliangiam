package radiusconfigs

import (
	"context"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BE-B-17: port must be within 1..65535 on create; invalid input is rejected
// before any UUID/clock/persistence work (zero-value Service proves it).
func TestCreateRadiusConfig_PortValidation(t *testing.T) {
	for _, port := range []int{0, -1, 65536} {
		svc := Service{}
		err := svc.CreateRadiusConfig(context.Background(), CreateRadiusConfigCommand{
			OrganizationID: "org-1", Name: "freeradius", Host: "radius.example.com",
			Port: port, SharedSecret: "shared-secret", AuthProtocol: "PAP",
		})
		if err == nil {
			t.Fatalf("port %d must be rejected", port)
		}
		if apperrors.FromError(err).Code != apperrors.CodeUnprocessableEntity {
			t.Fatalf("port %d: code = %v, want UnprocessableEntity", port, apperrors.FromError(err).Code)
		}
	}
}

// BE-B-17: patch validates the port pointer before opening a transaction.
func TestPatchRadiusConfig_PortValidationBeforeTransaction(t *testing.T) {
	svc := Service{} // no UOW: any transaction attempt would panic
	for _, port := range []int{0, -1, 65536} {
		p := port
		err := svc.PatchRadiusConfig(context.Background(), "org-1", PatchRadiusConfigCommand{ID: "config-1", Port: &p})
		if err == nil {
			t.Fatalf("port %d must be rejected", port)
		}
		if apperrors.FromError(err).Code != apperrors.CodeUnprocessableEntity {
			t.Fatalf("port %d: code = %v, want UnprocessableEntity", port, apperrors.FromError(err).Code)
		}
	}
}

func TestValidateRadiusPort_Boundaries(t *testing.T) {
	if err := validateRadiusPort(1); err != nil {
		t.Fatalf("port 1 rejected: %v", err)
	}
	if err := validateRadiusPort(65535); err != nil {
		t.Fatalf("port 65535 rejected: %v", err)
	}
}
