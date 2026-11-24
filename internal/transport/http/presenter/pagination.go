package presenter

type PageMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type ListResponse[T any] struct {
	Items []T      `json:"items"`
	Page  PageMeta `json:"page"`
}

func NewListResponse[T any](items []T, page, pageSize int, total int64) ListResponse[T] {
	// A nil slice marshals as `"items":null`, which is not the list contract:
	// clients iterate `items` unconditionally. Every empty result must serialize
	// as [] regardless of how the repository represented "no rows".
	if items == nil {
		items = make([]T, 0)
	}
	return ListResponse[T]{
		Items: items,
		Page: PageMeta{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}
}

// ActionResponse is the standardized success envelope for write operations
// (create/update/delete/action). See §4.1.2 of the developer handbook — the
// spec mandates `{status, msg}` for operation results; handler code must not
// return bare map[string]any literals.
type ActionResponse struct {
	Status string `json:"status"`
	Msg    string `json:"msg"`
	// Data carries operation-specific details (for example the IDV provider
	// redirect URL). It is omitted for plain status responses.
	Data map[string]any `json:"data,omitempty"`
}

func NewActionResponse(status, msg string) ActionResponse {
	return ActionResponse{Status: status, Msg: msg}
}

func NewActionResponseWithData(status, msg string, data map[string]any) ActionResponse {
	return ActionResponse{Status: status, Msg: msg, Data: data}
}
