package applications

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// queryOrganizationApplicationHosts 取同组织内所有未删除应用的 host 来源
// （主域名 + app_network_config.other_domains）。归一口径与 PG 实现一致，
// 共用 domain.NormalizeReverseProxyHost。
const queryOrganizationApplicationHosts = `
SELECT a.id, a.slug, COALESCE(a.domain, ''), COALESCE(n.other_domains, JSON_ARRAY())
FROM ` + btApp + ` a
LEFT JOIN ` + btNetworkConfig + ` n ON n.application_id = a.id
WHERE a.organization_id = ? AND a.deleted_at IS NULL AND a.id <> ?`

// FindHostConflicts 在同组织内（排除自身）查找已占用 hosts 的应用。
func (r Repository) FindHostConflicts(ctx context.Context, organizationID, excludeApplicationID string, hosts []string) ([]domain.HostConflict, error) {
	wanted := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		if host != "" {
			wanted[host] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return nil, nil
	}

	rows, err := r.db.QueryContext(ctx, queryOrganizationApplicationHosts, organizationID, excludeApplicationID)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "query application hosts")
	}
	defer rows.Close()

	seen := make(map[string]struct{}, len(wanted))
	conflicts := make([]domain.HostConflict, 0, len(wanted))
	for rows.Next() {
		var id, slug, primaryDomain string
		var rawOtherDomains []byte
		if err := rows.Scan(&id, &slug, &primaryDomain, &rawOtherDomains); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan application hosts")
		}
		var otherDomains []string
		if err := mysqlshared.UnmarshalJSON(rawOtherDomains, &otherDomains); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal other_domains")
		}
		for _, candidate := range append([]string{primaryDomain}, otherDomains...) {
			host := domain.NormalizeReverseProxyHost(candidate)
			if _, ok := wanted[host]; !ok {
				continue
			}
			key := id + "\x00" + host
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			conflicts = append(conflicts, domain.HostConflict{Host: host, ApplicationID: id, ApplicationSlug: slug})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternalf(err, "iterate application hosts")
	}
	return conflicts, nil
}
