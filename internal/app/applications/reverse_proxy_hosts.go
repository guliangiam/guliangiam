package applications

import (
	"context"
	"slices"
	"strings"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// hostRef 记录某个归一 host 来自本应用的哪个字段，用于把跨应用冲突落到具体输入项。
type hostRef struct {
	field string
	index int
}

// validateReverseProxyHostUniqueness 执行 R3d：同组织内 domain / otherDomains 不得与
// 其它应用已占用的 host 重合。与 R1–R4 一样只在相关字段真正变化时才查库（R6），
// 避免历史脏数据把无关保存拦死。
func validateReverseProxyHostUniqueness(ctx context.Context, repository applicationdomain.Repository, current *applicationdomain.Application, command PatchApplicationCommand) error {
	domainChanged := command.Domain != nil && strings.TrimSpace(*command.Domain) != strings.TrimSpace(current.Domain)
	otherDomainsChanged := command.OtherDomains != nil &&
		!slices.Equal(normalizeStringList(*command.OtherDomains), normalizeStringList(current.OtherDomains))
	if !domainChanged && !otherDomainsChanged {
		return nil
	}

	domain := current.Domain
	if command.Domain != nil {
		domain = *command.Domain
	}
	otherDomains := current.OtherDomains
	if command.OtherDomains != nil {
		otherDomains = normalizeStringList(*command.OtherDomains)
	}

	refs := make(map[string]hostRef, len(otherDomains)+1)
	if host := applicationdomain.NormalizeReverseProxyHost(domain); host != "" {
		refs[host] = hostRef{field: "domain"}
	}
	for index, raw := range otherDomains {
		host := applicationdomain.NormalizeReverseProxyHost(raw)
		if host == "" {
			continue
		}
		if _, ok := refs[host]; !ok {
			refs[host] = hostRef{field: "otherDomains", index: index}
		}
	}
	if len(refs) == 0 {
		return nil
	}

	hosts := make([]string, 0, len(refs))
	for host := range refs {
		hosts = append(hosts, host)
	}
	conflicts, err := repository.FindHostConflicts(ctx, current.OrganizationID, current.ID, hosts)
	if err != nil {
		return err
	}
	for _, conflict := range conflicts {
		ref, ok := refs[conflict.Host]
		if !ok {
			continue
		}
		meta := map[string]any{
			metaKeyField:       ref.field,
			metaKeyRule:        "R3",
			metaKeyValue:       conflict.Host,
			metaKeyHost:        conflict.Host,
			metaKeyConflictID:  conflict.ApplicationID,
			metaKeyConflictKey: conflict.ApplicationSlug,
		}
		if ref.field == "otherDomains" {
			meta[metaKeyIndex] = ref.index
		}
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"host "+conflict.Host+" is already used by application "+conflict.ApplicationSlug,
			meta,
		)
	}
	return nil
}
