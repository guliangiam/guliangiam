package shared

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const defaultBatchSize = 100

// safeIdentifier matches SQL identifiers (letters, digits, underscores, dots for schema.table).
// Rejects anything that could be used for injection (quotes, comments, operators, etc.).
var safeIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*$`)

// validateIdentifierList returns nil iff every identifier in names passes safeIdentifier.
func validateIdentifierList(names []string, kind string) error {
	for _, n := range names {
		if !safeIdentifier.MatchString(n) {
			return apperrors.NewInvalidArgumentf("unsafe %s identifier: %q", kind, n)
		}
	}
	return nil
}

func BatchInsert(ctx context.Context, db DBTX, table string, columns []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	if !safeIdentifier.MatchString(table) {
		return apperrors.NewInvalidArgumentf("unsafe table name: %q", table)
	}
	if err := validateIdentifierList(columns, "column"); err != nil {
		return err
	}

	colList := strings.Join(columns, ", ")
	numCols := len(columns)

	for offset := 0; offset < len(rows); offset += defaultBatchSize {
		end := offset + defaultBatchSize
		if end > len(rows) {
			end = len(rows)
		}

		batch := rows[offset:end]
		var sb strings.Builder
		fmt.Fprintf(&sb, "INSERT INTO %s (%s) VALUES ", table, colList)

		args := make([]any, 0, len(batch)*numCols)
		for i, row := range batch {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("(")
			for j := 0; j < numCols; j++ {
				if j > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(&sb, "$%d", len(args)+1)
				args = append(args, row[j])
			}
			sb.WriteString(")")
		}

		_, err := db.Exec(ctx, sb.String(), args...)
		if err != nil {
			return apperrors.WrapInternalf(err, "batch insert rows %d-%d", offset, end-1)
		}
	}

	return nil
}

func BatchInsertWithPool(ctx context.Context, pool DBTX, table string, columns []string, rows [][]any) error {
	return BatchInsert(ctx, pool, table, columns, rows)
}
