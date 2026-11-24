package auth

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// passwordGateBypassEvent is the audit action recorded when the password gate
// deliberately lets an impersonation subject through.
const passwordGateBypassEvent = "auth.password_change.gate_bypass"

// PasswordGateBypassInput describes one gate bypass decision for the audit log.
type PasswordGateBypassInput struct {
	OrganizationID string
	SubjectUserID  string
	ImpersonatorID string
	SessionID      string
	Method         string
	Path           string
	RequestID      string
	TraceID        string
	IPAddress      string
	UserAgent      string
}

// ImpersonationInfo identifies an impersonation token's subject and admin.
type ImpersonationInfo struct {
	SubjectUserID  string
	ImpersonatorID string
	OrganizationID string
}

// ResolveImpersonation returns impersonation metadata for an opaque token.
// Signed JWTs cannot carry impersonation metadata, so they are skipped without
// a database read. Lookup failures are treated as "not impersonation": the
// request still goes through the normal gate and authorization path.
func (s *Service) ResolveImpersonation(ctx context.Context, accessToken string) (*ImpersonationInfo, error) {
	token := strings.TrimSpace(accessToken)
	if token == "" || strings.Count(token, ".") == 2 {
		return nil, nil
	}

	row, err := s.UOW.Tokens().GetByTokenHash(ctx, platformcrypto.HashTokenValue(token))
	if err != nil || row == nil {
		return nil, nil
	}
	if !metadataBool(row.Metadata, "impersonated") {
		return nil, nil
	}

	info := &ImpersonationInfo{
		SubjectUserID:  row.UserID,
		OrganizationID: row.OrganizationID,
	}
	if impersonator, ok := row.Metadata["impersonated_by"].(string); ok {
		info.ImpersonatorID = strings.TrimSpace(impersonator)
	}
	if info.SubjectUserID == "" {
		return nil, nil
	}
	return info, nil
}

// RecordPasswordGateBypass writes one audit record plus its outbox event in a
// single system-scoped transaction.
func (s *Service) RecordPasswordGateBypass(ctx context.Context, input PasswordGateBypassInput) error {
	actorID := input.ImpersonatorID
	if actorID == "" {
		actorID = input.SubjectUserID
	}

	return s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		return s.embedAuditEvent(
			tx, ctx,
			input.IPAddress, input.UserAgent, "authentication", input.RequestID, input.TraceID,
			input.OrganizationID, actorID,
			passwordGateBypassEvent, "password_change_gate", input.SubjectUserID,
			"password_change_gate", passwordGateBypassEvent,
			map[string]any{
				"decision":       "bypass",
				"reason":         "impersonation",
				"subjectUserId":  input.SubjectUserID,
				"impersonatorId": input.ImpersonatorID,
				"sessionId":      input.SessionID,
				"method":         input.Method,
				"path":           input.Path,
			},
		)
	})
}

func metadataBool(metadata map[string]any, key string) bool {
	if metadata == nil {
		return false
	}
	value, ok := metadata[key].(bool)
	return ok && value
}
