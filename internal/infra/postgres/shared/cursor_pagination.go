package shared

import (
	"fmt"

	"github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// ApplyCursorPagination appends cursor-based pagination conditions to the query.
// It returns the parameterized query string (using PostgreSQL $N placeholders)
// and a slice of argument values to pass to the query executor.
//
// Security: cursor values (SortValue, ID) are never interpolated into SQL;
// they are passed as parameterized $N placeholders to prevent SQL injection.
func ApplyCursorPagination(query string, cursor *pagination.CursorToken, sortCol string, limit int, args ...any) (queryOut string, argsOut []any) {
	if cursor == nil {
		return fmt.Sprintf("%s LIMIT %d", query, limit), args
	}

	// Calculate parameter indices based on existing args length.
	// SortValue is used twice (for > comparison and = comparison), ID once.
	nextIdx := len(args) + 1
	whereClause := fmt.Sprintf(
		"(%s > $%d OR (%s = $%d AND id > $%d))",
		sortCol, nextIdx,
		sortCol, nextIdx+1,
		nextIdx+2,
	)

	paginatedQuery := fmt.Sprintf("%s AND %s LIMIT %d", query, whereClause, limit)
	args = append(args, cursor.SortValue, cursor.SortValue, cursor.ID)

	return paginatedQuery, args
}
