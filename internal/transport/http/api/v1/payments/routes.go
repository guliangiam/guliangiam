package payments

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes returns the Casbin permission definitions for payment endpoints.
func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/payments",
			OperationID: "ListPayments",
			Summary:     "List payments",
			Permission:  "payments:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/payments",
			OperationID: "CreatePayment",
			Summary:     "Create a payment configuration",
			Permission:  "payments:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/payments/{id}",
			OperationID: "GetPayment",
			Summary:     "Get a payment by ID",
			Permission:  "payments:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:         "POST",
			Pattern:        "/api/v1/payments/{id}/charge",
			OperationID:    "ChargePayment",
			Summary:        "Initiate a payment charge",
			Permission:     "payments:write",
			Surface:        route.SurfaceManagement,
			ErrorResponses: []int{422, 503},
		},
		{
			Method:         "POST",
			Pattern:        "/api/v1/payments/{id}/refund",
			OperationID:    "RefundPayment",
			Summary:        "Initiate a payment refund",
			Permission:     "payments:write",
			Surface:        route.SurfaceManagement,
			SuccessStatus:  201,
			ErrorResponses: []int{422, 503},
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/payments/{id}",
			OperationID: "PatchPayment",
			Summary:     "Patch a payment configuration",
			Permission:  "payments:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/payments/{id}",
			OperationID: "DeletePayment",
			Summary:     "Delete a payment configuration",
			Permission:  "payments:admin",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/payments/notify/{providerType}",
			OperationID: "NotifyPayment",
			Summary:     "Handle payment gateway callback notification",
			Permission:  "public", // public endpoint - no auth required
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/payments/{id}/invoice",
			OperationID: "InvoicePayment",
			Summary:     "Issue invoice for a payment",
			Permission:  "payments:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/payments/{id}/result",
			OperationID: "GetPaymentResult",
			Summary:     "Get payment result",
			Permission:  "payments:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/payments/user-payments",
			OperationID: "GetUserPayments",
			Summary:     "List payments for the current user",
			Permission:  "payments:read",
			Surface:     route.SurfaceManagement,
		},

		// ── 对账 (design doc §8.2) ──────────────────────────────────────
		{
			Method:      "GET",
			Pattern:     "/api/v1/pay-recon-batches",
			OperationID: "ListReconBatches",
			Summary:     "List reconciliation batches",
			Permission:  "payments:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/pay-recon-batches/{id}/items",
			OperationID: "ListReconItems",
			Summary:     "List reconciliation diff items for a batch",
			Permission:  "payments:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/pay-recon-items/{id}:resolve",
			OperationID: "ResolveReconItem",
			Summary:     "Manually resolve a reconciliation diff item (approval flow)",
			Permission:  "reconciliation:resolve",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/pay-recon-items/{id}:approve",
			OperationID: "ApproveReconItem",
			Summary:     "Approve a resolved reconciliation diff item (second-person review)",
			Permission:  "pay-recon-items:approve",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/pay-recon:run",
			OperationID: "RunRecon",
			Summary:     "Trigger reconciliation run manually",
			Permission:  "reconciliation:run",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 202,
		},

		// ── 渠道能力只读目录 ────────────────────────────────────────────
		{
			Method:      "GET",
			Pattern:     "/api/v1/pay-channels",
			OperationID: "ListPayChannels",
			Summary:     "List payment channel capabilities",
			Permission:  "payments:read",
			Surface:     route.SurfaceManagement,
		},

		// ── 渠道配置 (design doc G6 §P2) ────────────────────────────────
		{
			Method:      "GET",
			Pattern:     "/api/v1/pay-channel-configs",
			OperationID: "ListChannelConfigs",
			Summary:     "List channel configurations",
			Permission:  "payments:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/pay-channel-configs",
			OperationID: "CreateChannelConfig",
			Summary:     "Create a new channel configuration version",
			Permission:  "payments:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/pay-channel-configs/{id}",
			OperationID: "GetChannelConfig",
			Summary:     "Get a channel configuration by ID",
			Permission:  "payments:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/pay-channel-configs/{id}:transition",
			OperationID: "TransitionChannelConfig",
			Summary:     "Transition a channel config status (draft→testing→active/disabled)",
			Permission:  "payments:write",
			Surface:     route.SurfaceManagement,
		},

		// ── Payments History (design doc G5) ────────────────────────────
		{
			Method:      "GET",
			Pattern:     "/api/v1/payments/{paymentId}/attempts",
			OperationID: "ListAttemptsByPayment",
			Summary:     "List channel attempts for a payment (timeline)",
			Permission:  "payments:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/pay-attempts/{id}",
			OperationID: "GetAttemptByID",
			Summary:     "Get a single channel attempt by ID",
			Permission:  "payments:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/payments/{paymentId}/refunds",
			OperationID: "ListRefundsByPayment",
			Summary:     "List refunds for a payment",
			Permission:  "payments:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/pay-refunds/{id}",
			OperationID: "GetRefundByID",
			Summary:     "Get a single refund by ID",
			Permission:  "payments:read",
			Surface:     route.SurfaceManagement,
		},

		// ── Callback Receipts (design doc G7) ────────────────────────────
		{
			Method:      "GET",
			Pattern:     "/api/v1/pay-callback-receipts",
			OperationID: "ListCallbackReceipts",
			Summary:     "List raw channel callback receipts (audit log)",
			Permission:  "payments:read",
			Surface:     route.SurfaceManagement,
		},
	}
}
