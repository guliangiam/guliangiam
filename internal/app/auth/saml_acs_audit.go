package auth

import (
	"context"
	"strings"

	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// SAMLAcsFailureCommand describes an unauthenticated SAML ACS rejection. The
// record carries only the public organization slug/provider id and the failure
// class; assertion material is never persisted.
type SAMLAcsFailureCommand struct {
	OrganizationSlug string
	ProviderID       string
	Reason           string
	IPAddress        string
	UserAgent        string
	RequestID        string
	TraceID          string
}

// RecordSAMLAcsFailure appends the rejection audit before the protocol error is
// returned. A missing organization cannot be mapped to a tenant and therefore
// leaves no row; the caller must not fail the response because audit is best
// effort here (the ACS is unauthenticated).
func (s *Service) RecordSAMLAcsFailure(ctx context.Context, command SAMLAcsFailureCommand) error {
	orgSlug := strings.TrimSpace(command.OrganizationSlug)
	if orgSlug == "" || s.UOW == nil {
		return nil
	}
	org, err := s.UOW.Organizations().GetBySlugPublic(ctx, orgSlug)
	if err != nil {
		return nil
	}
	id, err := s.UUIDGenerator.NewString()
	if err != nil {
		return apperrors.WrapInternal(err, "generate saml acs record id")
	}
	ctx = tenantctx.WithTenantScope(ctx, org.ID, org.Slug, tenantctx.TenantFilterOptional)
	record := &domainrecords.Record{
		ID:             id,
		OrganizationID: org.ID,
		ActorID:        "",
		Action:         "protocol.saml.assertion_rejected",
		ResourceType:   "saml_provider",
		ResourceID:     strings.TrimSpace(command.ProviderID),
		IP:             command.IPAddress,
		UserAgent:      command.UserAgent,
		Surface:        domainrecords.SurfaceProtocol,
		RequestID:      command.RequestID,
		TraceID:        command.TraceID,
		Metadata:       map[string]any{"reason": strings.TrimSpace(command.Reason)},
		CreatedAt:      s.Clock.Now().UTC(),
	}
	if err := s.UOW.Records().Create(ctx, record); err != nil {
		return apperrors.WrapInternal(err, "create saml acs audit record")
	}
	return nil
}
