package auth

import (
	"context"
	"log/slog"
	"strings"

	"github.com/guliangiam/guliangiam/internal/app/sessionpolicy"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// recordSessionClientMismatch leaves evidence for a rejected client change.
// It runs after the session transaction has failed, so it uses its own system
// context; failure is logged but never replaces the 401 response.
func (s *Service) recordSessionClientMismatch(ctx context.Context, session *sessiondomain.Session, clientIP, clientUserAgent string, binding sessionpolicy.ClientBinding) {
	if session == nil || s.UOW == nil || s.UOW.Records() == nil {
		return
	}
	generator := s.UUIDGenerator
	if generator == nil {
		generator = platformuuid.V7Generator{}
	}
	id, err := generator.NewString()
	if err != nil {
		slog.WarnContext(ctx, "session client mismatch audit id", "error", err)
		return
	}
	ipChanged := strings.TrimSpace(session.IPAddress) != "" && strings.TrimSpace(clientIP) != "" && session.IPAddress != clientIP
	uaChanged := strings.TrimSpace(session.UserAgent) != "" && strings.TrimSpace(clientUserAgent) != "" && session.UserAgent != clientUserAgent
	record := &domainrecords.Record{
		ID:             id,
		OrganizationID: session.OrganizationID,
		ActorID:        session.UserID,
		Action:         "auth.session.client_mismatch",
		ResourceType:   "session",
		ResourceID:     session.ID,
		IP:             clientIP,
		UserAgent:      clientUserAgent,
		Surface:        domainrecords.SurfaceAuthentication,
		Metadata: map[string]any{
			"bindingIp":        binding.IP,
			"bindingUserAgent": binding.UserAgent,
			"ipChanged":        ipChanged,
			"userAgentChanged": uaChanged,
		},
		CreatedAt: s.Clock.Now().UTC(),
	}
	if err := s.UOW.Records().Create(tenantctx.System(ctx), record); err != nil {
		slog.WarnContext(ctx, "session client mismatch audit", "error", err)
	}
}
