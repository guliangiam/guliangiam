package scim

import (
	"sort"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// scimMaxCount 与 discovery filter.maxResults 同源（RFC 7644 §3.4.2.4）。
const scimMaxCount = 100

// scimUserSortPaths 把 SCIM 属性路径映射到 users.ListQuery.SortField 白名单键。
// 键一律小写：RFC 7643 §2.1 属性名大小写不敏感，userName/USERNAME 都命中。
// 集合 = User 排序白名单 ∪ meta.created/meta.lastModified（OPEN-API-87 关闭条件）。
var scimUserSortPaths = map[string]string{
	"id":                "id",
	"username":          "username",
	"usertype":          "userType",
	"status":            "status",
	"isadmin":           "isAdmin",
	"isforbidden":       "isForbidden",
	"score":             "score",
	"lastsigninat":      "lastSigninAt",
	"createdat":         "createdAt",
	"updatedat":         "updatedAt",
	"meta.created":      "createdAt",
	"meta.lastmodified": "updatedAt",
}

// scimGroupSortPaths 对应 groups.ListQuery.SortField 白名单（infra groupSortSpec）。
var scimGroupSortPaths = map[string]string{
	"id":                "id",
	"displayname":       "displayName",
	"createdat":         "createdAt",
	"updatedat":         "updatedAt",
	"meta.created":      "createdAt",
	"meta.lastmodified": "updatedAt",
}

// resolveScimSort 解析 RFC 7644 §3.4.2.3 的 sortBy/sortOrder：
//   - sortBy 缺省默认 id（升序），显式路径必须命中白名单，否则 400；
//   - sortOrder 仅 ascending/descending（大小写不敏感），缺省 ascending；
//   - 返回 infra 排序白名单的 API 键与 asc/desc，由仓储 Resolve 生成 ORDER BY。
func resolveScimSort(sortBy, sortOrder string, paths map[string]string) (field, order string, err error) {
	switch strings.ToLower(strings.TrimSpace(sortOrder)) {
	case "", "ascending":
		order = "asc"
	case "descending":
		order = "desc"
	default:
		return "", "", apperrors.NewInvalidArgument("sortOrder must be ascending or descending")
	}

	sortBy = strings.TrimSpace(sortBy)
	if sortBy == "" {
		return "id", order, nil
	}
	field, ok := paths[strings.ToLower(sortBy)]
	if !ok {
		return "", "", apperrors.NewInvalidArgumentf("sortBy must be one of: %s", strings.Join(sortedPathKeys(paths), ", "))
	}
	return field, order, nil
}

func sortedPathKeys(paths map[string]string) []string {
	keys := make([]string, 0, len(paths))
	for k := range paths {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// normalizeScimCount 执行 count 协议语义：超上限返回 scimType=tooMany 的 400
// （RFC 7644 §3.4.2.4），不再静默改写；count < 1 保持归一为上限。
func normalizeScimCount(count int) (int, error) {
	if count > scimMaxCount {
		return 0, apperrors.NewWithMeta(
			apperrors.CodeInvalidArgument,
			"count exceeds the maximum of 100",
			map[string]any{"scimType": "tooMany"},
		)
	}
	if count < 1 {
		count = scimMaxCount
	}
	return count, nil
}
