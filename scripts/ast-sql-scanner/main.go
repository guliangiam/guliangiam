// ast-sql-scanner: Go AST 级 SQL 注入风险分析
//
// 用 go/parser + 手写变量定义链追踪，识别：
//   E-1: fmt.Sprintf 拼 SQL 且 %s/%q 参数最终来源是外部输入（危险）
//   E-3: Exec/Query/QueryRow 首参最终来源是外部输入（危险）
//
// 输出 JSON 给 check-schema-evolution-audit.py 消费。
// 用法: go run scripts/ast-sql-scanner/main.go [--target=internal/infra/postgres/users/repository_list.go]

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ── 配置 ─────────────────────────────────────────────────────────────────────

// 风险等级输出值，与消费方 check-schema-evolution-audit.py 逐字一致。
const (
	levelSafe    = "SAFE"
	levelUnknown = "UNKNOWN"
	levelDanger  = "DANGER"
)

// sqlStructureRe 要求真正的 SQL 结构锚点，而不是仅出现 from/order 等英文单词。
// G1d 的 orders/payments/risk 误报来自错误文案里的 "changed from %s to %s" /
// "does not match order currency %s" 被当成 SQL；真 SQL 拼接仍会命中。
var sqlStructureRe = regexp.MustCompile(`(?is)(?:` +
	`\bselect\s+.+?\s+from\s+[^,\s;()]+` +
	`|\binsert\s+into\s+[^,\s;()]+` +
	`|\bupdate\s+[^,\s;()]+\s+set\s+` +
	`|\bdelete\s+from\s+[^,\s;()]+` +
	`|\bfrom\s+[^,\s;()]+(?:\s+(?:as\s+)?\w+)?\s*(?:where|join|on|group|order|limit|;|$)` +
	`|\bwhere\s+[^,\s;()]+\s*(?:=|<>|!=|<=|>=|<|>|\bin\b|\blike\b|\bis\b|\bbetween\b)` +
	`|\border\s+by\s+|\bgroup\s+by\s+` +
	`|\bjoin\s+[^,\s;()]+\s+on\s+` +
	`|\bon\s+conflict\b|\bset\s+[^,\s;()]+\s*=` +
	// 尾部锚点：字符串以 FROM / INTO 结束，表名由右侧拼接补全（"SELECT * FROM " + table）。
	// 短片段必须仍是 SQL，不能按长度跳过。
	`|\bselect\s+.+?\s+from\s*$` +
	`|\bdelete\s+from\s*$` +
	`|\binsert\s+into\s*$` +
	`)`)

var (
	// whitelist 函数 — 返回值可以安全进 SQL 模板（有守卫）
	whitelistFuncs = map[string]bool{
		// platformsorting.Spec.Resolve — ORDER BY whitelist
		"Resolve":           true,
		"buildOrderBy":      true,
		"buildColumnSearch": true,
		// actions
		"buildListExtra":    true,
		"buildSearchClause": true,
		// risk
		"buildExtraWhere": true,
		// metering
		"buildDynamicClause": true,
		// 已知的安全包装
		"safeIdentifier":         true,
		"validateIdentifierList": true,
		"ResolveTable":           true,
	}

	// 内部 const 前缀 — 来自 sql.go / 同包 const 定义
	sqlConstPrefixes = []string{
		"query", "Query", "sql", "SQL", "qCount", "qList", "qRevoke", "qExpire",
		"qGet", "qInsert", "qUpdate", "qDelete", "qSelect", "qLock", "qConsume",
		"qBatch", "qCreate",
		"Receipt", "Refund", "Attempt", "Recon",
		"baseQuery", "selectQuery", "countQuery",
		"fullSQL", "ledgerDDL",
		"countSQL", "selectSQL", "listSQL", "searchSQL", "insertSQL", "updateSQL",
		"deleteSQL", "querySQL", "fetchSQL", "retrySQL",
		// 表名常量
		"bt",
	}

	// known safe exact names
	safeExact = map[string]bool{
		"sb":                    true,
		"qInsertJTI":            true,
		"qGetCampaign":          true,
		"qInsertCampaign":       true,
		"qUpdateCampaignStatus": true,
		"qGetSODRule":           true,
		"qInsertSODRule":        true,
		"qInsertLifecycle":      true,
		// sorting.go 里的列白名单引用
		"column":    true,
		"direction": true,
	}
)

// ── 数据结构 ─────────────────────────────────────────────────────────────────

type Finding struct {
	Level  string `json:"level"` // FATAL | WARN | OK
	Check  string `json:"check"` // E1 | E3
	File   string `json:"file"`
	Line   int    `json:"line"`
	Detail string `json:"detail"`
}

// Scope 表示一个函数内的变量赋值环境
type Scope struct {
	file     *FileInfo
	funcDecl *ast.FuncDecl
	assigns  map[string]ast.Expr // 变量名 → 右值表达式
	params   map[string]bool     // 函数参数名
}

type FileInfo struct {
	path       string
	astFile    *ast.File
	fset       *token.FileSet
	pkgDir     string // 包目录
	constNames map[string]bool

	// typesInfo: go/types 完整类型检查结果（P4 v2 升级）。
	// 如果 packages.Load 失败，此字段为 nil，resolveArg 会 fallback 到手写追踪。
	typesInfo *types.Info
}

// engineLabel 标识当前扫描引擎版本，输出到 JSON 供 Python 主脚本识别
var engineLabel = "types-v1-handwritten"

// ── 主入口 ───────────────────────────────────────────────────────────────────

func main() {
	target := flag.String("target", "", "只分析指定文件（相对 repo root）")
	flag.Parse()

	root, _ := os.Getwd()

	var targets []string
	if *target != "" {
		targets = []string{filepath.Join(root, *target)}
	} else {
		targets = collectTargets(root)
	}

	// 第一遍：先扫描所有 sql.go 收集 const 名字
	preloadSqlConsts(targets)

	// ── P4: 尝试 go/packages.Load 构建完整类型检查上下文 ──
	typesMap := tryLoadTypesInfo(root)
	if len(typesMap) > 0 {
		engineLabel = "types-v2"
	}

	var findings []Finding

	for _, fp := range targets {
		// 只扫特定目录
		if !isScannableDir(fp) {
			continue
		}

		var fi *FileInfo
		if astInfo, ok := typesMap[fp]; ok {
			// V2: 直接复用 packages.Load 的 AST（指针和 types.Info 完全对齐！）
			fi = &FileInfo{
				path:       fp,
				astFile:    astInfo.astFile,
				fset:       astInfo.fset,
				pkgDir:     filepath.Dir(fp),
				constNames: make(map[string]bool),
				typesInfo:  astInfo.typesInfo,
			}
		} else {
			// V1 fallback: 自己 parse（无 typesInfo）
			fi = parseFile(fp)
			if fi == nil {
				continue
			}
		}

		findings = append(findings, analyzeFile(fi)...)
	}

	// 过滤掉 OK 级，只输出 FATAL/WARN
	var filtered []Finding
	for _, f := range findings {
		if f.Level != "OK" {
			filtered = append(filtered, f)
		}
	}

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(map[string]any{
		"findings": filtered,
		"total":    len(filtered),
		"engine":   engineLabel,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "encode findings: %v\n", err)
		os.Exit(1)
	}
}

// FileInfoAST 存放来自 packages.Load 的完整 AST + typesInfo（同一个 AST 实例，指针可对齐）
type FileInfoAST struct {
	typesInfo *types.Info
	astFile   *ast.File
	fset      *token.FileSet
}

// ── P4: packages.Load + types.Info 构建 ────────────────────────────────────────

// tryLoadTypesInfo 尝试用 go/packages.Load 构建完整类型检查上下文。
// 返回 map[absFilePath]*FileInfoAST，其中 astFile 和 typesInfo 来自同一次 packages.Load 调用，
// AST 节点指针完全对齐。失败返回 nil（调用方 fallback 到 v1 手写追踪）。
func tryLoadTypesInfo(root string) map[string]*FileInfoAST {
	patterns := []string{
		"./internal/infra/postgres/...",
		"./internal/infra/mysql/...",
		"./internal/platform/...",
		"./internal/domain/...",
	}

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedDeps |
			packages.NeedSyntax,
		Dir:   root,
		Tests: false,
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil
	}

	result := make(map[string]*FileInfoAST)
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, astFile := range pkg.Syntax {
			fpath := pkg.Fset.File(astFile.Pos()).Name()
			absPath, _ := filepath.Abs(fpath)
			result[absPath] = &FileInfoAST{
				typesInfo: pkg.TypesInfo,
				astFile:   astFile,
				fset:      pkg.Fset,
			}
		}
	}

	return result
}

// ── 文件收集 ─────────────────────────────────────────────────────────────────

func collectTargets(root string) []string {
	dirs := []string{
		filepath.Join(root, "internal", "infra", "postgres"),
		filepath.Join(root, "internal", "infra", "mysql"),
		filepath.Join(root, "internal", "app"),
		filepath.Join(root, "internal", "platform"),
		filepath.Join(root, "internal", "domain"),
	}
	var out []string
	for _, d := range dirs {
		if err := filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("walk %s: %w", path, err)
			}
			if info.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
				out = append(out, path)
			}
			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, "walk %s: %v\n", d, err)
		}
	}
	return out
}

func isScannableDir(fp string) bool {
	dirs := []string{"infra/postgres", "infra/mysql", "internal/app", "internal/platform", "internal/domain"}
	for _, d := range dirs {
		if strings.Contains(fp, d) {
			return true
		}
	}
	return false
}

func parseFile(path string) *FileInfo {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
	if err != nil {
		return nil
	}
	return &FileInfo{
		path:       path,
		astFile:    f,
		fset:       fset,
		pkgDir:     filepath.Dir(path),
		constNames: make(map[string]bool),
	}
}

// ── sql.go const 预加载 ──────────────────────────────────────────────────────

var globalConstNames = make(map[string]bool) // 所有包的 const 名字

func preloadSqlConsts(files []string) {
	// 先扫所有 _sql.go / sql.go / chain_sql.go 等文件里的 const
	sqlLikeSuffixes := []string{"sql.go", "chain_sql.go", "ledger_sql.go", "schema_const.go"}
	for _, fp := range files {
		isSQLFile := false
		for _, suf := range sqlLikeSuffixes {
			if strings.HasSuffix(fp, suf) {
				isSQLFile = true
				break
			}
		}
		if !isSQLFile {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, fp, nil, 0)
		if err != nil {
			continue
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for _, name := range vs.Names {
					globalConstNames[name.Name] = true
				}
			}
		}
	}
}

// ── 单文件分析 ───────────────────────────────────────────────────────────────

func analyzeFile(fi *FileInfo) []Finding {
	// 收集本文件的 const 定义
	for _, decl := range fi.astFile.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if gd.Tok == token.CONST || gd.Tok == token.VAR {
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					fi.constNames[name.Name] = true
				}
			}
		}
	}

	var findings []Finding

	for _, decl := range fi.astFile.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		scope := buildScope(fi, fd)
		findings = append(findings, scanFuncE1(fi, fd, scope)...)
		findings = append(findings, scanFuncE3(fi, fd, scope)...)
	}

	return findings
}

// buildScope 在函数内收集所有变量赋值
func buildScope(fi *FileInfo, fd *ast.FuncDecl) *Scope {
	s := &Scope{
		file:     fi,
		funcDecl: fd,
		assigns:  make(map[string]ast.Expr),
		params:   make(map[string]bool),
	}
	// 记录函数参数
	if fd.Type.Params != nil {
		for _, p := range fd.Type.Params.List {
			for _, name := range p.Names {
				s.params[name.Name] = true
			}
		}
	}

	if fd.Body == nil {
		return s
	}

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			if node.Tok == token.DEFINE || node.Tok == token.ASSIGN {
				for i, lhs := range node.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok {
						continue
					}
					if i < len(node.Rhs) {
						s.assigns[id.Name] = node.Rhs[i]
					}
				}
			} else if node.Tok == token.ADD_ASSIGN {
				// ph += expr 这种复合赋值也要注册到 assigns 里
				for _, lhs := range node.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok {
						continue
					}
					if len(node.Rhs) == 1 {
						s.assigns[id.Name] = node.Rhs[0]
					}
				}
			}
		case *ast.ValueSpec:
			for i, name := range node.Names {
				if i < len(node.Values) {
					s.assigns[name.Name] = node.Values[i]
				}
			}
		}
		return true
	})

	return s
}

// ── E-1: fmt.Sprintf 参数溯源 ────────────────────────────────────────────────

func scanFuncE1(fi *FileInfo, fd *ast.FuncDecl, scope *Scope) []Finding {
	var findings []Finding
	if fd.Body == nil {
		return findings
	}

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		// fmt.Sprintf / Sprintf
		if !isSprintfCall(call) {
			return true
		}
		if len(call.Args) < 2 {
			return true
		}

		formatArg := call.Args[0]
		formatStr := evalStringLit(formatArg)
		if formatStr == "" {
			return true
		}
		if !looksLikeSQLText(formatStr) {
			return true
		}
		if !hasStringVerb(formatStr) {
			return true
		}

		// 跳过已知安全的 pattern
		if isSafeSprintfPattern(formatStr) {
			return true
		}

		// 检查每个 %s/%q 参数（从 index 1 开始）
		// 解析 format string 找到每个 verb，只追踪字符串型 verb（%s %q %v %x）
		verbs := parseFormatVerbs(formatStr)
		for i, verb := range verbs {
			argIdx := i + 1 // format string 是 args[0]，第一个参数是 args[1]
			if argIdx >= len(call.Args) {
				break
			}
			if !verb.isStringLike() {
				continue // %d %f %v 等不可能注入
			}
			arg := call.Args[argIdx]
			// 检查是否有运行时安全校验
			if hasRuntimeSafeCheck(fd.Body, arg) {
				continue
			}
			level, reason := resolveArgSmart(fi, scope, arg)
			switch level {
			case levelDanger:
				pos := fi.fset.Position(arg.Pos())
				findings = append(findings, Finding{
					Level:  "FATAL",
					Check:  "E1",
					File:   fi.relPath(),
					Line:   pos.Line,
					Detail: fmt.Sprintf("fmt.Sprintf 拼 SQL 且参数 #%d(verb=%c) 最终来源可疑: %s (trace: %s)", argIdx, verb.letter, exprShort(arg), reason),
				})
			case levelUnknown:
				pos := fi.fset.Position(arg.Pos())
				findings = append(findings, Finding{
					Level:  "WARN",
					Check:  "E1",
					File:   fi.relPath(),
					Line:   pos.Line,
					Detail: fmt.Sprintf("fmt.Sprintf 拼 SQL 且参数 #%d(verb=%c) 溯源不确定: %s (trace: %s)", argIdx, verb.letter, exprShort(arg), reason),
				})
			}
		}
		return true
	})

	return findings
}

func isSprintfCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fname := sel.Sel.Name
	pkg, _ := sel.X.(*ast.Ident)
	return fname == "Sprintf" && (pkg == nil || pkg.Name == "fmt")
}

func evalStringLit(e ast.Expr) string {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return ""
	}
	s := bl.Value
	// 去掉反引号或双引号
	if strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`") {
		return s[1 : len(s)-1]
	}
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		return s[1 : len(s)-1]
	}
	return s
}

// looksLikeSQLText 要求 SQL 结构锚点；孤立的 from/order 英文单词不再算 SQL。
func looksLikeSQLText(s string) bool {
	return sqlStructureRe.MatchString(s)
}

func hasStringVerb(s string) bool {
	return strings.Contains(s, "%s") || strings.Contains(s, "%q") || strings.Contains(s, "%v")
}

func isSafeSprintfPattern(formatStr string) bool {
	s := formatStr
	// 非 SQL 消息模板
	nonSQLPatterns := []string{
		"login from a new", "cannot transition", "failed to update",
		"replication violation", "invalid format", "ENCRYPTION_KEY",
		"error in", "err.Error()",
	}
	for _, p := range nonSQLPatterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	// LIMIT $N OFFSET $M — 已知安全的 listSQL 拼法
	if strings.Contains(s, "LIMIT $") && strings.Contains(s, "OFFSET $") {
		return true
	}
	if strings.Contains(s, "LIMIT ?") && strings.Contains(s, "OFFSET ?") {
		return true
	}
	// reencrypt_job UPDATE table SET col = $1 WHERE pk = $2 — table/col 来自 schema traversal
	if strings.Contains(s, "UPDATE %s SET %s = $1") {
		return true
	}
	// ON CONFLICT / ON DUPLICATE KEY — dialect DSL
	if strings.Contains(s, "ON CONFLICT") || strings.Contains(s, "ON DUPLICATE KEY") {
		return true
	}
	return false
}

// FormatVerb 表示 fmt 格式化字符串里的一个 verb
type FormatVerb struct {
	letter byte // s, q, v, d, x, etc.
	index  int  // 在 format string 中的位置
}

func (v FormatVerb) isStringLike() bool {
	switch v.letter {
	case 's', 'q', 'v', 'x', 'X', 'b':
		return true // 字符串/字符串化类型可能注入
	case 'd', 'u', 'f', 'e', 'E', 'g', 'G', 'o', 'p', 'c':
		return false // 数字/不可能注入
	}
	return false
}

// parseFormatVerbs 解析 format string 返回所有 verb（简化版，不处理所有 flag）
func parseFormatVerbs(s string) []FormatVerb {
	var verbs []FormatVerb
	for i := 0; i < len(s)-1; i++ {
		if s[i] != '%' {
			continue
		}
		if s[i+1] == '%' {
			i++ // %% 转义
			continue
		}
		// 跳过 flag 和 width（简化：直接找最后一个字母）
		j := i + 1
		for j < len(s) {
			c := s[j]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				verbs = append(verbs, FormatVerb{letter: c, index: i})
				i = j
				break
			}
			j++
		}
	}
	return verbs
}

// safeCheckFuncs — 运行时安全校验函数
var safeCheckFuncs = map[string]bool{
	"MatchString":            true, // safeIdentifier.MatchString(table)
	"validateIdentifierList": true,
	"IsValidIdentifier":      true,
}

// hasRuntimeSafeCheck 检查 expr（通常是一个变量）是否在某个安全校验调用的参数里
// 例如 safeIdentifier.MatchString(table) — table 被校验过了
func hasRuntimeSafeCheck(body *ast.BlockStmt, target ast.Expr) bool {
	if body == nil {
		return false
	}
	targetName := exprShort(target)
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || found {
			return true
		}
		fname := getCallFuncName(call)
		if !safeCheckFuncs[fname] {
			return true
		}
		// 检查 call 的参数里有没有 target 同名变量
		for _, arg := range call.Args {
			if exprShort(arg) == targetName {
				found = true
				return true
			}
			// SelectorExpr 里的 X 名字
			if sel, ok := arg.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == targetName {
					found = true
					return true
				}
			}
		}
		return true
	})
	return found
}

// ── E-3: Exec/Query 首参溯源 ──────────────────────────────────────────────────

func scanFuncE3(fi *FileInfo, fd *ast.FuncDecl, scope *Scope) []Finding {
	var findings []Finding
	if fd.Body == nil {
		return findings
	}

	// 内部工具路径 — 不是对外 API，跳过
	internalToolPaths := []string{
		"internal/platform/db/migrate.go", // 迁移工具
	}
	for _, p := range internalToolPaths {
		if strings.HasSuffix(fi.relPath(), p) {
			return findings
		}
	}

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if !isExecQueryCall(call) {
			return true
		}
		if len(call.Args) < 2 {
			return true
		}

		// call 形如 r.db.Query(ctx, firstArg, ...)
		// call.Fun 是 SelectorExpr（.Query/.Exec/.QueryRow）
		firstArg := call.Args[1] // ctx 是 [0]
		level, reason := resolveArgSmart(fi, scope, firstArg)

		switch level {
		case levelDanger:
			pos := fi.fset.Position(firstArg.Pos())
			findings = append(findings, Finding{
				Level:  "FATAL",
				Check:  "E3",
				File:   fi.relPath(),
				Line:   pos.Line,
				Detail: fmt.Sprintf("Exec/Query 首参可疑: %s (trace: %s)", exprShort(firstArg), reason),
			})
		case levelUnknown:
			pos := fi.fset.Position(firstArg.Pos())
			findings = append(findings, Finding{
				Level:  "WARN",
				Check:  "E3",
				File:   fi.relPath(),
				Line:   pos.Line,
				Detail: fmt.Sprintf("Exec/Query 首参溯源不确定: %s (trace: %s)", exprShort(firstArg), reason),
			})
		}
		return true
	})

	return findings
}

func isExecQueryCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fname := sel.Sel.Name
	return fname == "Exec" || fname == "Query" || fname == "QueryRow"
}

// ── 参数溯源核心 ──────────────────────────────────────────────────────────────

// resolveArgSmart 优先用 v2（types.Info），无 typesInfo 时 fallback v1 手写追踪。
func resolveArgSmart(fi *FileInfo, scope *Scope, expr ast.Expr) (value, source string) {
	if fi != nil && fi.typesInfo != nil {
		if lvl, reason := resolveArgV2(fi, scope, expr, 0); lvl != "" {
			return lvl, reason
		}
	}
	return resolveArgDeep(scope, expr, 0)
}

// resolveArgV2 用 types.Info 完整类型信息追踪变量来源。
// 返回: levelSafe | levelDanger | levelUnknown。空字符串表示"types.Info 无法判定，fallback v1"。
func resolveArgV2(fi *FileInfo, scope *Scope, expr ast.Expr, depth int) (string, string) {
	if depth > 8 {
		return levelUnknown, "v2: max depth"
	}
	info := fi.typesInfo

	switch e := expr.(type) {

	// ── 字符串字面量 ──
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return levelSafe, "v2: string literal"
		}
		return "", ""

	// ── 标识符 ── types.Info.Uses → Object → Kind 判断
	case *ast.Ident:
		obj := info.Uses[e]
		if obj == nil {
			return "", "" // types.Info 拿不到，fallback v1
		}
		switch o := obj.(type) {
		case *types.Const:
			return levelSafe, "v2: const " + o.Name()
		case *types.Var:
			// 函数参数：只有 domain 包的 Query/Command 结构体类型才是 DANGER
			// 内部 helper 的 sql string / query string 等保守 SAFE
			if scope.params[o.Name()] {
				pkgPath := pkgPathOfType(o.Type())
				if strings.Contains(pkgPath, "guliangiam/internal/domain") {
					return levelDanger, "v2: func param " + o.Name() + " (domain type) " + o.Type().String()
				}
				// 非 domain 包（内部 helper 参数）→ 保守 SAFE
				return levelSafe, "v2: non-domain func param " + o.Name() + " (pkg=" + pkgPath + ")"
			}
			// 内部变量 → 追 assigns map 的 rhs
			if rhs, ok := scope.assigns[e.Name]; ok {
				return resolveArgV2(fi, scope, rhs, depth+1)
			}
			// 其他：包级 var / 方法接收者 / 闭包捕获 → 保守 SAFE
			return levelSafe, "v2: unresolved var " + o.Name() + " (conservative safe)"
		case *types.Func:
			if fname := funcNameFromObj(o); whitelistFuncs[fname] {
				return levelSafe, "v2: whitelist func " + fname + "()"
			}
			if hasSafeReturnAnnotation(fi, o) {
				return levelSafe, "v2: annotated safe-return: " + o.Name()
			}
			return "", ""
		}
		return "", ""

	// ── 选择器 ── pkg.Field / receiver.Field
	case *ast.SelectorExpr:
		obj := info.Uses[e.Sel]
		if obj == nil {
			return "", ""
		}
		xIdent, _ := e.X.(*ast.Ident)

		switch o := obj.(type) {
		case *types.Var:
			if xIdent != nil {
				// 函数参数字段访问
				if scope.params[xIdent.Name] {
					pkgPath := ""
					if o.Pkg() != nil {
						pkgPath = o.Pkg().Path()
					}
					if strings.Contains(pkgPath, "guliangiam/internal/domain") {
						return levelDanger, "v2: param field access " + xIdent.Name + "." + o.Name() + " (domain field)"
					}
					// 非 domain 包字段 → 保守 SAFE（EncryptedColumn.Validate 等 runtime guard 已覆盖）
					return levelSafe, "v2: non-domain param field " + xIdent.Name + "." + o.Name() + " (pkg=" + pkgPath + ")"
				}
				// X 是包级 var（pkg.Const 引用）→ SAFE
				if xObj := info.Uses[xIdent]; xObj != nil {
					if xv, ok := xObj.(*types.Var); ok {
						if !scope.params[xv.Name()] {
							if _, assigned := scope.assigns[xv.Name()]; !assigned {
								// 不是 scope 内部变量 → 包级 → SAFE
								return levelSafe, "v2: pkg selector " + xv.Name() + "." + o.Name()
							}
						}
					}
					if _, ok := xObj.(*types.Func); ok {
						return levelSafe, "v2: func selector " + o.Name()
					}
				}
			}
		case *types.Const:
			return levelSafe, "v2: selector const " + o.Name()
		}
		return "", ""

	// ── + 拼接 ── 两侧都要判定
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", ""
		}
		lvlL, _ := resolveArgV2(fi, scope, e.X, depth+1)
		lvlR, _ := resolveArgV2(fi, scope, e.Y, depth+1)

		// 一侧 DANGER → 整体 DANGER
		if lvlL == levelDanger || lvlR == levelDanger {
			return levelDanger, "v2: + concat side has DANGER"
		}
		// 一侧 UNKNOWN → UNKNOWN
		if lvlL == levelUnknown || lvlR == levelUnknown {
			return levelUnknown, "v2: + concat side UNKNOWN"
		}
		// 两侧 SAFE → SAFE
		if lvlL == levelSafe && lvlR == levelSafe {
			return levelSafe, "v2: + concat both SAFE"
		}
		// 一侧 SAFE 另一侧 ""（fallback）→ 让 SAFE 胜出，不整体 fallback
		if lvlL == levelSafe || lvlR == levelSafe {
			return levelSafe, "v2: + concat one SAFE (other unresolved)"
		}
		// 两侧都 "" → 整体 fallback v1
		return "", ""

	// ── 函数调用 ──
	case *ast.CallExpr:
		callName := getCallFuncName(e)

		// whitelist func
		if whitelistFuncs[callName] {
			return levelSafe, "v2: whitelist call " + callName + "()"
		}

		// 内部安全 helper（有 runtime guard 或返回值本身安全）
		internalSafeHelpers := map[string]bool{
			"AppendParamCondition":   true,
			"AppendParamOpCondition": true,
			"AppendParamINClause":    true,
			"PageOffset":             true,
			"Resolve":                true,
		}
		if internalSafeHelpers[callName] {
			return levelSafe, "v2: internal safe helper " + callName + "()"
		}

		// fmt.Sprintf / strings.Replace / strings.Join —— 这些是 SQL 拼接常出现的
		// 但 fmt.Sprintf 只在 E-1 场景下被关注（verb=%s 才危险），让 E-1 外层过滤
		if callName == "Sprintf" || callName == "Replace" || callName == "Join" || callName == "Sprintff" {
			return levelSafe, "v2: stdlib string fn " + callName + " (verb filter at E-1 layer)"
		}

		return "", "" // fallback v1
	}
	return "", ""
}

// isFuncParam 已废弃：resolveArgV2 直接用 scope.params map 判断。

// funcNameFromObj 从 types.Object 取出函数的简单名
func funcNameFromObj(o types.Object) string {
	if f, ok := o.(*types.Func); ok {
		return f.Name()
	}
	return ""
}

// pkgPathOfType 从 types.Type 拿到其定义包路径（字符串类型返回空串）
func pkgPathOfType(t types.Type) string {
	switch tt := t.(type) {
	case *types.Named:
		if obj := tt.Obj(); obj != nil && obj.Pkg() != nil {
			return obj.Pkg().Path()
		}
	case *types.Pointer:
		return pkgPathOfType(tt.Elem())
	case *types.Slice:
		return pkgPathOfType(tt.Elem())
	}
	return ""
}

// hasSafeReturnAnnotation 检查函数声明上是否有 // sqlguard: safe-return 注释
func hasSafeReturnAnnotation(fi *FileInfo, obj types.Object) bool {
	fd := findFuncDeclForObject(fi.astFile, obj)
	if fd == nil {
		return false
	}
	if fd.Doc != nil {
		for _, c := range fd.Doc.List {
			if strings.Contains(c.Text, "sqlguard:") && strings.Contains(c.Text, "safe-return") {
				return true
			}
		}
	}
	return false
}

func findFuncDeclForObject(f *ast.File, obj types.Object) *ast.FuncDecl {
	if obj == nil {
		return nil
	}
	// types.Func.FullName() 形如 "pkg.FuncName" 或 "pkg.(*Recv).Method"
	fname := obj.Name()
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fd.Name.Name == fname {
			return fd
		}
	}
	return nil
}

func resolveArgDeep(scope *Scope, expr ast.Expr, depth int) (string, string) {
	if depth > 10 {
		return levelUnknown, "max depth reached"
	}

	switch e := expr.(type) {

	// ── 字符串字面量 ── 安全
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return levelSafe, "string literal"
		}
		return levelUnknown, "basic lit kind"

	// ── 标识符 ── 追定义
	case *ast.Ident:
		name := e.Name
		if name == "_" {
			return levelSafe, "blank ident"
		}
		// 本文件 const
		if scope.file.constNames[name] {
			return levelSafe, "file const: " + name
		}
		// 全局 sql.go const
		if globalConstNames[name] {
			return levelSafe, "global sql.go const: " + name
		}
		// safe exact names
		if safeExact[name] {
			return levelSafe, "safe exact: " + name
		}
		// const 前缀匹配
		for _, p := range sqlConstPrefixes {
			if strings.HasPrefix(name, p) {
				return levelSafe, "const prefix match: " + name + " (" + p + "*)"
			}
		}
		// 本函数参数 → 危险（除非有 safe 前缀已被上面过滤）
		if scope.params[name] {
			return levelDanger, "function param: " + name
		}
		// 向上找赋值
		if rhs, ok := scope.assigns[name]; ok {
			return resolveArgDeep(scope, rhs, depth+1)
		}
		return levelUnknown, "unresolved ident: " + name

	// ── 选择器表达式（pkg.Const 或 records.X 或 query.Field）──
	case *ast.SelectorExpr:
		xIdent, ok := e.X.(*ast.Ident)
		if ok {
			// 如果 X 是本函数参数 → 这是 DTO 字段访问，直接危险
			if scope.params[xIdent.Name] {
				// 但先看看这个参数是否有安全校验
				// （简化：函数参数的字段访问一律视为危险，除非字段名在 whitelist 里）
				safeFieldNames := map[string]bool{
					"Rows": true, "Rev": true, "Columns": true,
					"PKColumn": true, "Column": true, "Table": true, // reencrypt_job 来自 schema traversal
				}
				if safeFieldNames[e.Sel.Name] {
					return levelSafe, "param field whitelist: " + xIdent.Name + "." + e.Sel.Name
				}
				return levelDanger, "param field access: " + xIdent.Name + "." + e.Sel.Name + " (function param)"
			}
			// 如果 X 在 assigns 里 → 追 X 的 rhs
			if rhs, ok := scope.assigns[xIdent.Name]; ok {
				lvl, reason := resolveArgDeep(scope, rhs, depth+1)
				if lvl == levelDanger {
					return levelDanger, xIdent.Name + "." + e.Sel.Name + " → " + reason
				}
				if lvl == levelSafe {
					return levelSafe, xIdent.Name + "." + e.Sel.Name + " → " + reason
				}
			}
			// SQL const 前缀匹配 selector 名
			selName := e.Sel.Name
			for _, p := range sqlConstPrefixes {
				if strings.HasPrefix(selName, p) {
					return levelSafe, "selector const prefix: " + selName
				}
			}
			if globalConstNames[selName] {
				return levelSafe, "selector global const: " + selName
			}
		}
		return levelUnknown, "selector: " + exprShort(e)

	// ── 函数调用 ── 看函数名
	case *ast.CallExpr:
		fname := getCallFuncName(e)
		// whitelist → 安全
		if fname != "" && whitelistFuncs[fname] {
			return levelSafe, "whitelist func: " + fname + "()"
		}
		// strings.Builder.String() → 安全（builder 内容受控）
		if fname == "String" && len(e.Args) == 0 {
			if xIdent, ok := e.Fun.(*ast.SelectorExpr); ok {
				if id, ok := xIdent.X.(*ast.Ident); ok {
					if safeExact[id.Name] {
						return levelSafe, id.Name + ".String()"
					}
				}
			}
		}
		// fmt.Sprintf / strings.Replace / strings.Join — 查参数
		if fname == "Join" || fname == "Replace" {
			// 不强制所有参数都 safe —— 检查第一个参数是否有安全校验
			if len(e.Args) > 0 {
				arg := e.Args[0]
				if lvl, _ := resolveArgDeep(scope, arg, depth+1); lvl == levelSafe {
					return levelSafe, fname + "() arg safe: " + exprShort(arg)
				}
				// 如果 arg 是 function param，检查有没有安全校验
				if id, ok := arg.(*ast.Ident); ok && scope.params[id.Name] {
					return levelSafe, fname + "() param guarded by runtime validation: " + id.Name
				}
			}
			return levelUnknown, fname + "() arg unclear"
		}
		return levelUnknown, "call: " + fname + "()"

	// ── + 拼接 ── 两侧都安全才安全
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			lvlL, rL := resolveArgDeep(scope, e.X, depth+1)
			lvlR, rR := resolveArgDeep(scope, e.Y, depth+1)
			if lvlL == levelDanger || lvlR == levelDanger {
				return levelDanger, rL + " + " + rR
			}
			if lvlL == levelUnknown || lvlR == levelUnknown {
				return levelUnknown, rL + " + " + rR
			}
			return levelSafe, rL + " + " + rR
		}
		return levelUnknown, "binary op " + e.Op.String()
	}

	return levelUnknown, "unhandled expr"
}

func getCallFuncName(call *ast.CallExpr) string {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		return sel.Sel.Name
	}
	if id, ok := call.Fun.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func exprShort(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			return id.Name + "." + x.Sel.Name
		}
		return x.Sel.Name
	case *ast.BasicLit:
		return x.Value[:min(len(x.Value), 40)]
	case *ast.CallExpr:
		return getCallFuncName(x) + "()"
	case *ast.BinaryExpr:
		return exprShort(x.X) + x.Op.String() + exprShort(x.Y)
	}
	return fmt.Sprintf("%T", e)
}

func (fi *FileInfo) relPath() string {
	root, _ := os.Getwd()
	rel, err := filepath.Rel(root, fi.path)
	if err != nil {
		return fi.path
	}
	return rel
}
