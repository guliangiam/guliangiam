package aigateway

import rthttp "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes publishes the complete AI Gateway route contract (both management
// CRUD and data-plane protocol face).  The runtime router in bootstrap must
// register exactly these paths — RouteContractMatchesRuntimeRouter enforces
// the one-to-one mapping.
func Routes() []rthttp.Definition {
	return []rthttp.Definition{
		// ── Providers ───────────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/v1/ai/providers", OperationID: "ListAIGatewayProviders", Permission: "aigateway:providers", Surface: rthttp.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/ai/providers", OperationID: "CreateAIGatewayProvider", Permission: "aigateway:providers", Surface: rthttp.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/ai/providers/{id}", OperationID: "GetAIGatewayProvider", Permission: "aigateway:providers", Surface: rthttp.SurfaceManagement},
		{Method: "PATCH", Pattern: "/api/v1/ai/providers/{id}", OperationID: "UpdateAIGatewayProvider", Permission: "aigateway:providers", Surface: rthttp.SurfaceManagement},
		{Method: "DELETE", Pattern: "/api/v1/ai/providers/{id}", OperationID: "DeleteAIGatewayProvider", Permission: "aigateway:providers", Surface: rthttp.SurfaceManagement, SuccessStatus: 204},
		{Method: "POST", Pattern: "/api/v1/ai/providers/{id}/test", OperationID: "TestAIGatewayProvider", Permission: "aigateway:providers", Surface: rthttp.SurfaceManagement},

		// ── Models ─────────────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/v1/ai/models", OperationID: "ListAIGatewayModels", Permission: "aigateway:models", Surface: rthttp.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/ai/models", OperationID: "CreateAIGatewayModel", Permission: "aigateway:models", Surface: rthttp.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/ai/models/{id}", OperationID: "GetAIGatewayModel", Permission: "aigateway:models", Surface: rthttp.SurfaceManagement},
		{Method: "PATCH", Pattern: "/api/v1/ai/models/{id}", OperationID: "UpdateAIGatewayModel", Permission: "aigateway:models", Surface: rthttp.SurfaceManagement},
		{Method: "DELETE", Pattern: "/api/v1/ai/models/{id}", OperationID: "DeleteAIGatewayModel", Permission: "aigateway:models", Surface: rthttp.SurfaceManagement, SuccessStatus: 204},

		// ── Aliases ────────────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/v1/ai/aliases", OperationID: "ListAIGatewayAliases", Permission: "aigateway:aliases", Surface: rthttp.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/ai/aliases", OperationID: "CreateAIGatewayAlias", Permission: "aigateway:aliases", Surface: rthttp.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/ai/aliases/{id}", OperationID: "GetAIGatewayAlias", Permission: "aigateway:aliases", Surface: rthttp.SurfaceManagement},
		{Method: "PATCH", Pattern: "/api/v1/ai/aliases/{id}", OperationID: "UpdateAIGatewayAlias", Permission: "aigateway:aliases", Surface: rthttp.SurfaceManagement},
		{Method: "DELETE", Pattern: "/api/v1/ai/aliases/{id}", OperationID: "DeleteAIGatewayAlias", Permission: "aigateway:aliases", Surface: rthttp.SurfaceManagement, SuccessStatus: 204},

		// ── Routes ─────────────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/v1/ai/routes", OperationID: "ListAIGatewayRoutes", Permission: "aigateway:routes", Surface: rthttp.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/ai/routes", OperationID: "CreateAIGatewayRoute", Permission: "aigateway:routes", Surface: rthttp.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/ai/routes/{id}", OperationID: "GetAIGatewayRoute", Permission: "aigateway:routes", Surface: rthttp.SurfaceManagement},
		{Method: "PATCH", Pattern: "/api/v1/ai/routes/{id}", OperationID: "UpdateAIGatewayRoute", Permission: "aigateway:routes", Surface: rthttp.SurfaceManagement},
		{Method: "DELETE", Pattern: "/api/v1/ai/routes/{id}", OperationID: "DeleteAIGatewayRoute", Permission: "aigateway:routes", Surface: rthttp.SurfaceManagement, SuccessStatus: 204},

		// ── API Keys ───────────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/v1/ai/api-keys", OperationID: "ListAIGatewayAPIKeys", Permission: "aigateway:api_keys", Surface: rthttp.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/ai/api-keys", OperationID: "CreateAIGatewayAPIKey", Permission: "aigateway:api_keys", Surface: rthttp.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/ai/api-keys/{id}", OperationID: "GetAIGatewayAPIKey", Permission: "aigateway:api_keys", Surface: rthttp.SurfaceManagement},
		{Method: "PATCH", Pattern: "/api/v1/ai/api-keys/{id}", OperationID: "UpdateAIGatewayAPIKey", Permission: "aigateway:api_keys", Surface: rthttp.SurfaceManagement},
		{Method: "DELETE", Pattern: "/api/v1/ai/api-keys/{id}", OperationID: "DeleteAIGatewayAPIKey", Permission: "aigateway:api_keys", Surface: rthttp.SurfaceManagement, SuccessStatus: 204},
		{Method: "POST", Pattern: "/api/v1/ai/api-keys/{id}/revoke", OperationID: "RevokeAIGatewayAPIKey", Permission: "aigateway:api_keys", Surface: rthttp.SurfaceManagement, SuccessStatus: 204},

		// ── Quota Policies ─────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/v1/ai/quotas", OperationID: "ListAIGatewayQuotas", Permission: "aigateway:quotas", Surface: rthttp.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/ai/quotas", OperationID: "CreateAIGatewayQuota", Permission: "aigateway:quotas", Surface: rthttp.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/ai/quotas/{id}", OperationID: "GetAIGatewayQuota", Permission: "aigateway:quotas", Surface: rthttp.SurfaceManagement},
		{Method: "PATCH", Pattern: "/api/v1/ai/quotas/{id}", OperationID: "UpdateAIGatewayQuota", Permission: "aigateway:quotas", Surface: rthttp.SurfaceManagement},
		{Method: "DELETE", Pattern: "/api/v1/ai/quotas/{id}", OperationID: "DeleteAIGatewayQuota", Permission: "aigateway:quotas", Surface: rthttp.SurfaceManagement, SuccessStatus: 204},

		// ── Usage ───────────────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/v1/ai/usage", OperationID: "ListAIGatewayUsageLogs", Permission: "aigateway:usage", Surface: rthttp.SurfaceManagement},
		{Method: "GET", Pattern: "/api/v1/ai/usage/stats", OperationID: "GetAIGatewayUsageStats", Permission: "aigateway:usage", Surface: rthttp.SurfaceManagement},

		// ── Price Versions ─────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/v1/ai/prices", OperationID: "ListAIGatewayPrices", Permission: "aigateway:prices", Surface: rthttp.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/ai/prices", OperationID: "CreateAIGatewayPrice", Permission: "aigateway:prices", Surface: rthttp.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/ai/prices/{id}", OperationID: "GetAIGatewayPrice", Permission: "aigateway:prices", Surface: rthttp.SurfaceManagement},
		{Method: "PATCH", Pattern: "/api/v1/ai/prices/{id}", OperationID: "UpdateAIGatewayPrice", Permission: "aigateway:prices", Surface: rthttp.SurfaceManagement},
		{Method: "DELETE", Pattern: "/api/v1/ai/prices/{id}", OperationID: "DeleteAIGatewayPrice", Permission: "aigateway:prices", Surface: rthttp.SurfaceManagement, SuccessStatus: 204},

		// ── Provider Health (只读 — 状态由 GatewayService 自动写) ──────
		{Method: "GET", Pattern: "/api/v1/ai/health", OperationID: "ListAIGatewayHealth", Permission: "aigateway:health", Surface: rthttp.SurfaceManagement},
		{Method: "GET", Pattern: "/api/v1/ai/health/{id}", OperationID: "GetAIGatewayHealth", Permission: "aigateway:health", Surface: rthttp.SurfaceManagement},

		// ── Data Plane: OpenAI-compatible Protocol Face ────────────────
		{Method: "POST", Pattern: "/api/v1/ai/faces/openai/chat/completions", OperationID: "OpenAIChatCompletions", Permission: "aigateway:chat", Surface: rthttp.SurfaceProtocol},
		{Method: "POST", Pattern: "/api/v1/ai/faces/openai/embeddings", OperationID: "OpenAIEmbeddings", Permission: "aigateway:chat", Surface: rthttp.SurfaceProtocol},
		{Method: "GET", Pattern: "/api/v1/ai/faces/openai/models", OperationID: "OpenAIListModels", Permission: "aigateway:chat", Surface: rthttp.SurfaceProtocol},

		// ── Data Plane: Anthropic Messages Protocol Face ───────────────
		{Method: "POST", Pattern: "/api/v1/ai/faces/anthropic/chat/completions", OperationID: "AnthropicChatCompletions", Permission: "aigateway:chat", Surface: rthttp.SurfaceProtocol},
		{Method: "POST", Pattern: "/api/v1/ai/faces/anthropic/messages", OperationID: "AnthropicMessages", Permission: "aigateway:chat", Surface: rthttp.SurfaceProtocol},
		{Method: "POST", Pattern: "/api/v1/ai/faces/anthropic/embeddings", OperationID: "AnthropicEmbeddings", Permission: "aigateway:chat", Surface: rthttp.SurfaceProtocol},
		{Method: "GET", Pattern: "/api/v1/ai/faces/anthropic/models", OperationID: "AnthropicListModels", Permission: "aigateway:chat", Surface: rthttp.SurfaceProtocol},

		// ── Data Plane: Gemini generateContent Protocol Face ────────────
		{Method: "POST", Pattern: "/api/v1/ai/faces/gemini/chat/completions", OperationID: "GeminiChatCompletions", Permission: "aigateway:chat", Surface: rthttp.SurfaceProtocol},
		{Method: "POST", Pattern: "/api/v1/ai/faces/gemini/embeddings", OperationID: "GeminiEmbeddings", Permission: "aigateway:chat", Surface: rthttp.SurfaceProtocol},
		{Method: "GET", Pattern: "/api/v1/ai/faces/gemini/models", OperationID: "GeminiListModels", Permission: "aigateway:chat", Surface: rthttp.SurfaceProtocol},
	}
}
