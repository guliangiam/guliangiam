package metering

import (
	"context"
)

// QuotaKeyEvents is the tenant_quotas key consumed per ingested metering event.
// Additional keys (mau, ai_tokens, ...) are defined by their real aggregation
// windows; this gate only enforces the per-event counter.
const QuotaKeyEvents = "events"

// QuotaStatus is the outcome of one atomic quota consumption.
type QuotaStatus struct {
	Allowed bool
	Used    int64
	Limit   int64
}

// QuotaConsumer is the narrow capability the metering service asserts on the
// UoW-scoped metering repository. It is a separate interface (not part of
// Repository) so non-PostgreSQL mirrors and test fakes keep compiling; when the
// capability is absent the ingest path simply does not enforce quotas.
type QuotaConsumer interface {
	ConsumeQuota(ctx context.Context, organizationID, quotaKey string, delta int64) (QuotaStatus, error)
}
