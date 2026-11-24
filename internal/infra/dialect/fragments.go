package dialect

// 多个方言实现共享的 SQL 片段与物理类型名。同一片段重复书写时，任何一次改动
// 都可能只改到其中一半方言；集中后由编译器保证一致。常量值即历史输出，改值
// 等于改生成的 SQL。
const (
	boolLiteralTrue  = "TRUE"
	boolLiteralFalse = "FALSE"

	forUpdateClause           = "FOR UPDATE"
	forUpdateSkipLockedClause = "FOR UPDATE SKIP LOCKED"

	physicalTypeCLOB = "CLOB"
	physicalTypeINT  = "INT"

	jsonArrayFuncOpen = "JSON_ARRAY("
)
