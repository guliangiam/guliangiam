package platformadmins

import (
	"strings"
	"testing"
)

// 口径一致性回归：Gate 7 与列表查询必须逐字复用同一个 scopeValidity 常量，
// 不允许各自复制一份有效性判定。
func TestScopeQueriesShareSingleValidityPredicate(t *testing.T) {
	for _, fragment := range []string{"expires_at > now()", "is_bootstrap", "approved_by IS NOT NULL"} {
		if !strings.Contains(scopeValidity, fragment) {
			t.Fatalf("scopeValidity must encode %q, got: %s", fragment, scopeValidity)
		}
	}

	queries := map[string]string{
		"list":  queryListScopesByAdmin,
		"gate7": queryHasActiveScopeForTenant,
	}
	for name, query := range queries {
		if !strings.Contains(query, scopeValidity) {
			t.Fatalf("%s query must embed scopeValidity verbatim, got: %s", name, query)
		}
		// 判定片段在整条 SQL 中只允许出现一次（全部来自 scopeValidity）。
		if got := strings.Count(query, "approved_by IS NOT NULL"); got != 1 {
			t.Fatalf("%s query repeats the approval predicate %d times, want 1", name, got)
		}
		if got := strings.Count(query, "expires_at > now()"); got != 1 {
			t.Fatalf("%s query repeats the expiry predicate %d times, want 1", name, got)
		}
	}
}
