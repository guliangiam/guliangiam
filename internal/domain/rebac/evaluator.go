package rebac

import "context"

// Evaluator 实现 ADR-024 定义的 ReBAC 关系元组求值语义：
//   - Check: 用户对对象是否有某关系（直接 + 递归继承 + subject_relation 链）
//   - Expand: 以某 (relation, object) 为根，展开所有能到达的 subject 树
//   - BatchCheck: 一次调用中验证多个 Check，复用 BFS frontier
//
// 求值器自身无持久化状态，通过 TupleStore 拉取元组快照后在内存中执行。
// 环检测 + 深度上限（默认 8 跳）保护避免用户数据形成循环时栈溢出。
//
// Zookie 机制（OpenFGA 兼容写后读一致性）：
//   每次写入 tuple 时，namespace 的 revision_counter 单调递增，该值作为 zookie
//   写入 tuple 并返回给调用方。后续 Check/Expand 请求携带此 zookie，
//   求值器只读取 zookie ≤ atZookie 的元组，保证"我看到了这次写入"。
//   空 zookie 表示读取当前最新快照。

// TupleKey 是 OpenFGA 风格的 (user, relation, object) 三元组。
// SubjectRelation 可选，表达 "user via group member" 这类 membership 链。
type TupleKey struct {
	Subject         string  `json:"subject"`
	SubjectRelation *string `json:"subjectRelation,omitempty"`
	Relation        string  `json:"relation"`
	Object          string  `json:"object"`
}

// CheckRequest 求值请求。
//
// AtZookie 为可选：
//   - 空字符串 → 读取当前最新快照
//   - 非空   → 只读取 zookie ≤ AtZookie 的元组（写后读一致）
type CheckRequest struct {
	OrganizationID string
	NamespaceID    string
	Tuple          TupleKey
	DepthLimit     int
	AtZookie       string
}

// CheckResult 返回求值结论。
//
// Zookie 是本次求值所基于的快照 revision（调用方可保存并在后续读请求中使用，
// 保证不读到比这个快照更新的写入）。
type CheckResult struct {
	Allowed bool     `json:"allowed"`
	Trace   []string `json:"trace,omitempty"`
	Zookie  string   `json:"zookie"`
}

// ExpandRequest 以 (relation, object) 为根，展开所有可达 subject。
// AtZookie 语义同 CheckRequest。
type ExpandRequest struct {
	OrganizationID string
	NamespaceID    string
	Relation       string
	Object         string
	DepthLimit     int
	AtZookie       string
}

// ExpandNode 是 Expand 结果树的一个节点，OpenFGA expand 响应的嵌套形状。
type ExpandNode struct {
	Subject    string       `json:"subject"`
	Expanded   bool         `json:"expanded,omitempty"`
	Children   []ExpandNode `json:"children,omitempty"`
	SubjectRel *string      `json:"subjectRelation,omitempty"`
}

// ExpandResult 是 Expand 求值的完整返回。
// Zookie 语义同 CheckResult。
type ExpandResult struct {
	Root   ExpandNode `json:"root"`
	Zookie string     `json:"zookie"`
}

// BatchCheckItem 是 BatchCheck 的单条请求/响应对。
type BatchCheckItem struct {
	Tuple  TupleKey    `json:"tuple"`
	Result CheckResult `json:"result"`
}

// TupleStore 是求值器从持久化层拉取元组的最小接口。
// Repository 已有 ListTuples 分页签名，求值器扩展一个批量查询方法。
//
// FetchAllTuples 的 atZookie 参数控制快照边界：
//   - 空串 → 返回全部元组（最新快照）
//   - 非空 → 只返回 zookie ≤ atZookie 的元组
//
// GetLatestZookie 返回 namespace 当前最新 revision_counter 值（字符串）。
type TupleStore interface {
	FetchAllTuples(ctx context.Context, organizationID, namespaceID, atZookie string) ([]*Tuple, error)
	GetLatestZookie(ctx context.Context, organizationID, namespaceID string) (string, error)
}

// Evaluator 是 ReBAC 核心求值器。
type Evaluator interface {
	Check(ctx context.Context, req CheckRequest) (*CheckResult, error)
	Expand(ctx context.Context, req ExpandRequest) (*ExpandResult, error)
	BatchCheck(ctx context.Context, orgID, nsID string, items []TupleKey, atZookie string) ([]CheckResult, error)
}

// DefaultMaxDepth 是求值器的 BFS 深度上限。
const DefaultMaxDepth = 8
