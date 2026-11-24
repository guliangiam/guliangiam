import { z } from "zod";

// ─────────────────────────────────────────────
// Shared Zod schemas used across multiple domain modules
// ─────────────────────────────────────────────

/** Standard pagination metadata returned by all list endpoints. */
export const pageMetaSchema = z.object({
  page: z.number(),
  pageSize: z.number(),
  total: z.number(),
});

/** Flexible key-value object schema for dynamic server responses. */
export const flexibleObjectSchema = z.record(z.string(), z.unknown()).catch({});

/** Array of flexible key-value objects. */
export const flexibleObjectArraySchema = z.array(flexibleObjectSchema).catch([]);

/** Standard envelope for action responses (status + msg). */
export const actionResponseSchema = z.object({
  status: z.literal("ok"),
  msg: z.string().optional(),
});

/** Error response envelope from the API. */
export const errorResponseSchema = z.object({
  error: z.object({
    code: z.string(),
    message: z.string(),
  }),
});

/** Generic paged response wrapper for iamListWithPagination results. */
export function pagedResponseSchema<T extends z.ZodTypeAny>(itemSchema: T) {
  return z.object({
    items: z.array(itemSchema),
    page: pageMetaSchema,
  });
}

// ─────────────────────────────────────────────
// Shared TypeScript types
// ─────────────────────────────────────────────

/** Lightweight identity reference for an organization. */
export type OrganizationIdentity = {
  id: string;
  slug: string;
};

/** Standard pagination state for server-side paginated lists. */
export type PaginationState = {
  page: number;
  pageSize: number;
  total: number;
};

/** Parameters for server-side list queries. */
export type ListQueryParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  organizationId?: string;
  status?: string;
};

/**
 * Error codes as defined by the backend (错误码注册表 §1, the 17 constants in
 * `internal/platform/errors`). `bad_request` and `internal_error` are NOT part <!-- naming-allow：本行仅声明该字面量已废弃，不得在代码或契约中使用 -->
 * of that set — the canonical names are `invalid_argument` and `internal` — and
 * using them here made the type disagree with every response the server sends.
 * `unknown` is client-only, for responses that carry no code at all.
 */
export type ErrorCode =
  | "invalid_argument"
  | "unauthorized"
  | "forbidden"
  | "password_expired"
  | "mfa_required"
  | "not_found"
  | "conflict"
  | "already_exists"
  | "invitation_used"
  | "invitation_expired"
  | "gone"
  | "account_locked"
  | "too_many_requests"
  | "internal"
  | "unimplemented"
  | "unavailable"
  | "unknown";

// ─────────────────────────────────────────────
// Form validation schemas for create pages
// ─────────────────────────────────────────────

export const userCreateSchema = z.object({
  organizationId: z.string().trim().min(1, "Organization is required"),
  username: z
    .string()
    .trim()
    .min(2, "Username must be at least 2 characters")
    .max(100, "Username must be under 100 characters"),
  displayName: z
    .string()
    .trim()
    .min(1, "Display name is required")
    .max(200, "Display name must be under 200 characters"),
  email: z.string().trim().email("Invalid email format").optional().or(z.literal("")),
  homepage: z.string().trim().url("Invalid homepage URL").optional().or(z.literal("")),
  phone: z.string().optional(),
  // 密码最小长度由组织策略在表单/后端判定，schema 只保留与策略无关的上限。
  password: z
    .string()
    .max(128, "Password must be no more than 128 characters")
    .optional()
    .or(z.literal("")),
  userType: z.string().min(1, "User type is required"),
  status: z.string().min(1, "Status is required"),
});

export const roleCreateSchema = z.object({
  organizationId: z.string().min(1, "Organization is required"),
  name: z
    .string()
    .min(2, "Name must be at least 2 characters")
    .max(100, "Name must be under 100 characters"),
  displayName: z
    .string()
    .min(1, "Display name is required")
    .max(200, "Display name must be under 200 characters"),
  description: z.string().max(500, "Description must be under 500 characters").optional(),
  isEnabled: z.boolean().optional(),
});

export const permissionCreateSchema = z.object({
  organizationId: z.string().min(1, "Organization is required"),
  name: z
    .string()
    .min(2, "Name must be at least 2 characters")
    .max(100, "Name must be under 100 characters"),
  displayName: z
    .string()
    .min(1, "Display name is required")
    .max(200, "Display name must be under 200 characters"),
  description: z.string().max(500, "Description must be under 500 characters").optional(),
  resourceType: z.string().min(1, "Resource type is required"),
  // The backend canonicalises effect to lowercase (`normalizeEffect`) and the
  // create form's options emit allow/deny; PascalCase is not accepted here.
  effect: z.union([z.literal("allow"), z.literal("deny")]),
  actions: z.array(z.string()).min(1, "At least one action is required"),
  // permissions.sorter is a text column and the transport DTO is *string.
  sorter: z.string().optional(),
  isEnabled: z.boolean().optional(),
});
