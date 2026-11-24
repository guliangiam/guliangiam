import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import { pageMetaSchema } from "./schemas";
import type { PaginatedResult } from "./client";
import { TOKENS, TOKENS_REVOKE, TOKEN_BY_ID, TOKEN_REVOKE_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type TokenItem = {
  id: string;
  userId: string;
  username: string;
  organizationId: string;
  organizationSlug: string;
  applicationId: string;
  applicationSlug: string;
  tokenPrefix: string;
  tokenHashSet: boolean;
  tokenHashMasked: string;
  scopes: string[];
  audience: string[];
  expiresAt: string;
  revokedAt: string | null;
  createdAt: string;
  updatedAt: string;
};

export type TokenDetail = TokenItem & {
  metadata: Record<string, unknown>;
  revision: number;
};

export type TokenCreatePayload = {
  organizationId: string;
  userId: string;
  applicationId: string;
  scopes: string[];
  audience?: string[];
  metadata?: Record<string, unknown>;
  expiresInSeconds: number;
};

export type CreatedToken = {
  status: "created";
  tokenValue: string;
  token: TokenDetail;
};

export type TokenFilters = {
  organizationId?: string;
  applicationId?: string;
};

export type RevokedTokens = {
  status: "revoked";
  revokedCount: number;
};

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

const tokenListItemSchema = z.object({
  id: z.string(),
  userId: z.string(),
  username: z.string(),
  organizationId: z.string(),
  organizationSlug: z.string(),
  applicationId: z.string(),
  applicationSlug: z.string(),
  tokenPrefix: z.string(),
  // BE-D-19: reads never return the raw hash, only the paired masked projection.
  tokenHashSet: z.boolean(),
  tokenHashMasked: z.string(),
  scopes: z.array(z.string()).catch([]),
  audience: z.array(z.string()).catch([]),
  expiresAt: z.string(),
  revokedAt: z.string().nullable().optional(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const tokenDetailSchema = tokenListItemSchema.extend({
  metadata: z.record(z.string(), z.unknown()).default({}),
  revision: z.number(),
});

const createTokenResponseSchema = z.object({
  status: z.literal("created"),
  tokenValue: z.string(),
  token: tokenDetailSchema,
});

const revokeTokensResponseSchema = z.object({
  status: z.literal("revoked"),
  revokedCount: z.number(),
});

const tokenListResponseSchema = z.object({
  items: z.array(tokenListItemSchema),
  page: pageMetaSchema,
});

// ─────────────────────────────────────────────
// Mappers
// ─────────────────────────────────────────────

function toTokenItem(item: z.infer<typeof tokenListItemSchema>): TokenItem {
  return {
    id: item.id,
    userId: item.userId,
    username: item.username,
    organizationId: item.organizationId,
    organizationSlug: item.organizationSlug,
    applicationId: item.applicationId,
    applicationSlug: item.applicationSlug,
    tokenPrefix: item.tokenPrefix,
    tokenHashSet: item.tokenHashSet,
    tokenHashMasked: item.tokenHashMasked,
    scopes: item.scopes,
    audience: item.audience,
    expiresAt: item.expiresAt,
    revokedAt: item.revokedAt ?? null,
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
  };
}

function toTokenDetail(item: z.infer<typeof tokenDetailSchema>): TokenDetail {
  return {
    ...toTokenItem(item),
    metadata: item.metadata,
    revision: item.revision,
  };
}

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function getTokens(filters?: string | TokenFilters) {
  const organizationId = typeof filters === "string" ? filters : filters?.organizationId;
  const applicationId = typeof filters === "string" ? undefined : filters?.applicationId;
  const query = new URLSearchParams({
    page: "1",
    pageSize: "50",
    activeOnly: "true",
  });
  if (organizationId) {
    query.set("organizationId", organizationId);
  }
  if (applicationId) {
    query.set("applicationId", applicationId);
  }

  const response = await requestDocumentJson(`${TOKENS}?${query.toString()}`);
  return tokenListResponseSchema.parse(response).items.map(toTokenItem);
}

export type TokenListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  organizationId?: string;
  applicationId?: string;
  activeOnly?: boolean;
};

export async function getTokensPaginated(params?: TokenListParams) {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
    activeOnly: String(params?.activeOnly ?? true),
  });
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  if (params?.applicationId) query.set("applicationId", params.applicationId);
  // `keyword` is the name the token list handler reads; `search` is not a
  // parameter of this API and was silently ignored.
  if (params?.search) query.set("keyword", params.search);

  const response = await requestDocumentJson(`${TOKENS}?${query.toString()}`);
  const parsed = tokenListResponseSchema.parse(response);
  return {
    items: parsed.items.map(toTokenItem),
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  } satisfies PaginatedResult<TokenItem>;
}

export async function getToken(id: string): Promise<TokenDetail> {
  const response = await requestDocumentJson(`${TOKEN_BY_ID(id)}`);
  return toTokenDetail(tokenDetailSchema.parse(response));
}

export async function createToken(payload: TokenCreatePayload): Promise<CreatedToken> {
  const response = await requestDocumentJson(`${TOKENS}`, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });

  const parsed = createTokenResponseSchema.parse(response);
  return {
    status: parsed.status,
    tokenValue: parsed.tokenValue,
    token: toTokenDetail(parsed.token),
  };
}

export async function revokeToken(id: string) {
  await requestDocumentJson(`${TOKEN_REVOKE_BY_ID(id)}`, {
    method: "POST",
  });
}

export async function revokeTokensByOrganization(organizationId: string): Promise<RevokedTokens> {
  const response = await requestDocumentJson(`${TOKENS_REVOKE}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ organizationId }),
  });

  return revokeTokensResponseSchema.parse(response);
}

export async function deleteToken(id: string) {
  await requestDocumentJson(`${TOKEN_BY_ID(id)}`, {
    method: "DELETE",
  });
}
