package rebac

import "github.com/guliangiam/guliangiam/internal/transport/http/presenter"

// ─── 基础复用类型 ─────────────────────────────────────────────────────────

type TupleKeyJSON struct {
	Subject         string  `json:"subject"`
	SubjectRelation *string `json:"subjectRelation,omitempty"`
	Relation        string  `json:"relation"`
	Object          string  `json:"object"`
}

type CreateTupleRequest struct {
	Subject         string  `json:"subject"`
	SubjectRelation *string `json:"subjectRelation"`
	Relation        string  `json:"relation"`
	Object          string  `json:"object"`
}

// ─── Namespace ────────────────────────────────────────────────────────────

type CreateNamespaceRequest struct {
	Name           string         `json:"name"`
	DisplayName    string         `json:"displayName"`
	OrganizationID string         `json:"-"` // 从 session/query 注入
	RelationDefs   map[string]any `json:"relationDefs"`
}

type CreateNamespaceResponse struct {
	Namespace NamespaceResponse `json:"namespace"`
}

type ListNamespacesRequest struct {
	OrganizationID string `json:"-"`
	Page           int    `json:"-"`
	PageSize       int    `json:"-"`
}

type ListNamespacesResponse = presenter.ListResponse[NamespaceResponse]

type NamespaceResponse struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organizationId"`
	Name           string         `json:"name"`
	DisplayName    string         `json:"displayName"`
	RelationDefs   map[string]any `json:"relationDefs"`
	Version        int            `json:"version"`
	CreatedAt      string         `json:"createdAt,omitempty"`
	UpdatedAt      string         `json:"updatedAt,omitempty"`
}

// ─── Tuple ────────────────────────────────────────────────────────────────

type WriteTuplesRequest struct {
	OrganizationID string         `json:"-"`
	NamespaceID    string         `json:"-"`
	Tuples         []TupleKeyJSON `json:"tuples"`
}

type WriteTuplesResponse struct {
	IDs []string `json:"ids"`
}

type TupleResponse struct {
	ID              string  `json:"id"`
	OrganizationID  string  `json:"organizationId"`
	NamespaceID     string  `json:"namespaceId"`
	Subject         string  `json:"subject"`
	SubjectRelation *string `json:"subjectRelation,omitempty"`
	Relation        string  `json:"relation"`
	Object          string  `json:"object"`
	Zookie          string  `json:"zookie"`
	CreatedAt       string  `json:"createdAt"`
}

// ─── ADR-024 Check / Expand / BatchCheck ───────────────────────────────────

// CheckRequest 的 Zookie 字段是可选的写后读一致性 token：
//   - 空 → 读取最新快照
//   - 非空 → 只看到此 zookie 之前的写入
type CheckRequest struct {
	OrganizationID string `json:"-"`
	NamespaceID    string `json:"namespaceId"`
	Subject        string `json:"subject"`
	Relation       string `json:"relation"`
	Object         string `json:"object"`
	DepthLimit     int    `json:"depthLimit,omitempty"`
	Zookie         string `json:"zookie,omitempty"`
}

type CheckResponse struct {
	Allowed bool     `json:"allowed"`
	Trace   []string `json:"trace,omitempty"`
	Zookie  string   `json:"zookie"`
}

type ExpandRequest struct {
	OrganizationID string `json:"-"`
	NamespaceID    string `json:"namespaceId"`
	Relation       string `json:"relation"`
	Object         string `json:"object"`
	DepthLimit     int    `json:"depthLimit,omitempty"`
	Zookie         string `json:"zookie,omitempty"`
}

type ExpandResponse struct {
	Root   ExpandNodeResponse `json:"root"`
	Zookie string             `json:"zookie"`
}

type ExpandNodeResponse struct {
	Subject    string               `json:"subject"`
	Expanded   bool                 `json:"expanded,omitempty"`
	SubjectRel *string              `json:"subjectRelation,omitempty"`
	Children   []ExpandNodeResponse `json:"children,omitempty"`
}

type BatchCheckItemJSON struct {
	Subject  string `json:"subject"`
	Relation string `json:"relation"`
	Object   string `json:"object"`
}

type BatchCheckRequest struct {
	OrganizationID string               `json:"-"`
	NamespaceID    string               `json:"namespaceId"`
	Items          []BatchCheckItemJSON `json:"items"`
	Zookie         string               `json:"zookie,omitempty"`
}

type BatchCheckItem struct {
	Subject  string `json:"subject"`
	Relation string `json:"relation"`
	Object   string `json:"object"`
	Allowed  bool   `json:"allowed"`
}

type BatchCheckResponse struct {
	Results []BatchCheckItem `json:"results"`
	Zookie  string           `json:"zookie"`
}
