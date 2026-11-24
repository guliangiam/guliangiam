package shared

const SoftDeleteFilter = "AND deleted_at IS NULL"

func SoftDeleteClause(column string) string {
	return column + " = NOW()"
}
