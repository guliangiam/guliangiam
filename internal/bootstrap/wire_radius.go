package bootstrap

import (
	"context"
	"log/slog"
	"strings"
	"time"

	appradiusaccountings "github.com/guliangiam/guliangiam/internal/app/radiusaccountings"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// newRadiusAccountingHandler builds the RADIUS accounting persistence callback
// (BE-B-13). Accounting packets carry "orgSlug/username"; the slug is resolved
// to a tenant under a controlled System scope (the packet context has no tenant
// GUC and organizations is FORCE RLS), then the accounting row is written under
// the resolved tenant's strict scope. Packets without an org slug cannot be
// attributed to a tenant and are dropped with a warning — never written to an
// arbitrary organization.
func newRadiusAccountingHandler(uow appuow.UnitOfWork, acctSvc appradiusaccountings.Service) func(ctx context.Context, username, sessionID, eventType string, bytesIn, bytesOut int64) {
	return func(ctx context.Context, username, sessionID, eventType string, bytesIn, bytesOut int64) {
		orgSlug, cleanUsername := splitRadiusUsername(username)
		if orgSlug == "" {
			slog.WarnContext(ctx, "radius accounting: packet without org slug dropped",
				"session_id", sessionID, "event_type", eventType)
			return
		}

		var organizationID string
		err := uow.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
			org, gerr := tx.Organizations().GetBySlug(ctx, orgSlug)
			if gerr != nil {
				return gerr
			}
			organizationID = org.ID
			return nil
		})
		if err != nil || organizationID == "" {
			slog.WarnContext(ctx, "radius accounting: organization slug not resolved; packet dropped",
				"org_slug", orgSlug, "session_id", sessionID)
			return
		}

		tenantCtx := tenantctx.WithTenantScope(ctx, organizationID, "", tenantctx.TenantFilterStrict)
		cmd := appradiusaccountings.CreateRadiusAccountingCommand{
			OrganizationID:  organizationID,
			Name:            eventType,
			Username:        cleanUsername,
			AcctSessionID:   sessionID,
			AcctInputTotal:  bytesIn,
			AcctOutputTotal: bytesOut,
			AcctStartTime:   time.Now(),
		}
		if err := acctSvc.CreateRadiusAccounting(tenantCtx, cmd); err != nil {
			slog.WarnContext(ctx, "radius accounting: persist failed",
				"org_slug", orgSlug, "session_id", sessionID, "event_type", eventType, "error", err)
		}
	}
}

// splitRadiusUsername splits "orgSlug/username" into its parts; a bare username
// yields an empty slug.
func splitRadiusUsername(username string) (orgSlug, cleanUsername string) {
	if parts := strings.SplitN(username, "/", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", username
}
