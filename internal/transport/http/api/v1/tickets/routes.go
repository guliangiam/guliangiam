package tickets

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{Method: "GET", Pattern: "/api/v1/tickets", OperationID: "ListTickets", Summary: "List tickets", Permission: "tickets:read", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/tickets", OperationID: "CreateTicket", Summary: "Create a ticket", Permission: "tickets:write", Surface: route.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/tickets/{id}", OperationID: "GetTicket", Summary: "Get a ticket by ID", Permission: "tickets:read", Surface: route.SurfaceManagement},
		{Method: "PATCH", Pattern: "/api/v1/tickets/{id}", OperationID: "PatchTicket", Summary: "Patch a ticket", Permission: "tickets:write", Surface: route.SurfaceManagement},
		{Method: "DELETE", Pattern: "/api/v1/tickets/{id}", OperationID: "DeleteTicket", Summary: "Delete a ticket", Permission: "tickets:admin", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/tickets/{id}/messages", OperationID: "AddTicketMessage", Summary: "Add a message to a ticket", Permission: "tickets:write", Surface: route.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/tickets/{id}/messages", OperationID: "ListTicketMessages", Summary: "List messages of a ticket", Permission: "tickets:read", Surface: route.SurfaceManagement},
	}
}
