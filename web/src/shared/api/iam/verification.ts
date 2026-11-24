import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import { VERIFICATIONS, VERIFICATION_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type VerificationItem = z.infer<typeof verificationListItemSchema>;

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

const verificationListItemSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  type: z.string(),
  recipient: z.string(),
  code: z.string(),
  status: z.string(),
  expiresAt: z.string(),
  createdAt: z.string(),
});

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function getVerifications(
  organizationId?: string,
  filters?: { type?: string; status?: string; recipient?: string },
) {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) query.set("organizationId", organizationId);
  if (filters?.type) query.set("type", filters.type);
  if (filters?.status) query.set("status", filters.status);
  if (filters?.recipient) query.set("recipient", filters.recipient);
  const response = await requestDocumentJson(`${VERIFICATIONS}?${query.toString()}`);
  const parsed = z
    .object({
      items: z.array(verificationListItemSchema),
      page: z.object({ total: z.number(), page: z.number(), pageSize: z.number() }),
    })
    .parse(response);
  return parsed.items;
}

export type VerificationListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  organizationId?: string;
  type?: string;
  status?: string;
  recipient?: string;
  sortBy?: string;
  sortOrder?: string;
};

export async function getVerificationsPaginated(params?: VerificationListParams) {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
  });
  if (params?.search) query.set("keyword", params.search);
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  if (params?.type) query.set("type", params.type);
  if (params?.status) query.set("status", params.status);
  if (params?.recipient) query.set("recipient", params.recipient);
  const response = await requestDocumentJson(`${VERIFICATIONS}?${query.toString()}`);
  const parsed = z
    .object({
      items: z.array(verificationListItemSchema),
      page: z.object({ total: z.number(), page: z.number(), pageSize: z.number() }),
    })
    .parse(response);
  return {
    items: parsed.items,
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  };
}
export async function getVerification(id: string): Promise<VerificationItem> {
  const response = await requestDocumentJson(VERIFICATION_BY_ID(id));
  return verificationListItemSchema.parse(response);
}
