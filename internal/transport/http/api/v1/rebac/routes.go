package rebac

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes 定义 ReBAC OpenFGA 兼容 API 的路由表。
// Endpoint 设计对齐 OpenFGA public API 形状（/stores/{store_id}/check → /rebac/namespaces/{id}/check）。
// Organization 作为租户上下文从 request metadata 注入，不在 URL path 中暴露。
func Routes() []route.Definition {
	return []route.Definition{
		// Namespace CRUD
		{
			Method:      "GET",
			Pattern:     "/api/v1/rebac/namespaces",
			OperationID: "ListRebacNamespaces",
			Summary:     "List ReBAC relation namespaces",
			Permission:  "rebac:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/rebac/namespaces",
			OperationID: "CreateRebacNamespace",
			Summary:     "Create a new relation namespace",
			Permission:  "rebac:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus:   201,
			ResponseHeaders: []string{"Location"},
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/rebac/namespaces/{id}",
			OperationID: "GetRebacNamespace",
			Summary:     "Get a relation namespace by ID",
			Permission:  "rebac:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/rebac/namespaces/{id}",
			OperationID: "DeleteRebacNamespace",
			Summary:     "Delete a relation namespace",
			Permission:  "rebac:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 204,
		},

		// Tuple CRUD (per namespace)
		{
			Method:      "GET",
			Pattern:     "/api/v1/rebac/namespaces/{namespaceId}/tuples",
			OperationID: "ListRebacTuples",
			Summary:     "List relation tuples in a namespace",
			Permission:  "rebac:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/rebac/namespaces/{namespaceId}/tuples",
			OperationID: "WriteRebacTuples",
			Summary:     "Write (create or idempotent) relation tuples",
			Permission:  "rebac:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/rebac/tuples/{id}",
			OperationID: "DeleteRebacTuple",
			Summary:     "Delete a relation tuple",
			Permission:  "rebac:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 204,
		},

		// ADR-024 核心求值接口
		{
			Method:      "POST",
			Pattern:     "/api/v1/rebac/namespaces/{namespaceId}/check",
			OperationID: "RebacCheck",
			Summary:     "Check if subject has relation on object",
			Permission:  "rebac:check",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/rebac/namespaces/{namespaceId}/expand",
			OperationID: "RebacExpand",
			Summary:     "Expand all subjects that reach a (relation, object)",
			Permission:  "rebac:check",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/rebac/namespaces/{namespaceId}/batch-check",
			OperationID: "RebacBatchCheck",
			Summary:     "Batch-check multiple (subject, relation, object) tuples",
			Permission:  "rebac:check",
			Surface:     route.SurfaceManagement,
		},
	}
}
