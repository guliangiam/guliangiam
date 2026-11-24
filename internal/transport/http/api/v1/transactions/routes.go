package transactions

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{Method: "GET", Pattern: "/api/v1/transactions", OperationID: "ListTransactions", Summary: "List transactions", Permission: "transactions:read", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/transactions", OperationID: "CreateTransaction", Summary: "Create a transaction", Permission: "transactions:write", Surface: route.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/transactions/{id}", OperationID: "GetTransaction", Summary: "Get a transaction by ID", Permission: "transactions:read", Surface: route.SurfaceManagement},
		{Method: "PATCH", Pattern: "/api/v1/transactions/{id}", OperationID: "PatchTransaction", Summary: "Patch a transaction", Permission: "transactions:write", Surface: route.SurfaceManagement},
		{Method: "DELETE", Pattern: "/api/v1/transactions/{id}", OperationID: "DeleteTransaction", Summary: "Delete a transaction", Permission: "transactions:admin", Surface: route.SurfaceManagement},
	}
}
