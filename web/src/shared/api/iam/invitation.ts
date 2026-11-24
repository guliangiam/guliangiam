import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import {
  INVITATIONS,
  INVITATIONS_VERIFY,
  INVITATION_BY_ID,
  INVITATION_RESEND,
  INVITATION_REVOKE,
  INVITATION_SEND,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type InvitationDetail = z.infer<typeof invitationDetailSchema>;
export type CreateInvitationPayload = {
  organizationId: string;
  email: string;
  roleIds?: string[];
  groupIds?: string[];
  invitedBy?: string;
  validDays?: number;
};

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

const invitationListItemSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  email: z.string(),
  code: z.string().optional(),
  roleIds: z.array(z.string()),
  groupIds: z.array(z.string()).optional(),
  status: z.string(),
  expiresAt: z.string(),
  acceptedAt: z.string().nullable().optional(),
  invitedBy: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
  revision: z.number(),
});

const invitationListResponseSchema = z.object({
  items: z.array(invitationListItemSchema),
  page: z.object({ total: z.number(), page: z.number(), pageSize: z.number() }),
});

const invitationDetailSchema = invitationListItemSchema;

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function getInvitations(organizationId?: string, status?: string, keyword?: string) {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) query.set("organizationId", organizationId);
  if (status) query.set("status", status);
  if (keyword) query.set("keyword", keyword);
  const response = await requestDocumentJson(`${INVITATIONS}?${query.toString()}`);
  return invitationListResponseSchema.parse(response).items;
}

export type InvitationListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  organizationId?: string;
  status?: string;
};

export async function getInvitationsPaginated(params?: InvitationListParams) {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
  });
  if (params?.search) query.set("keyword", params.search);
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  if (params?.status) query.set("status", params.status);
  const response = await requestDocumentJson(`${INVITATIONS}?${query.toString()}`);
  const parsed = invitationListResponseSchema.parse(response);
  return {
    items: parsed.items,
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  };
}

export async function getInvitation(id: string): Promise<InvitationDetail> {
  const response = await requestDocumentJson(`${INVITATION_BY_ID(id)}`);
  return invitationDetailSchema.parse(response);
}

export async function createInvitation(payload: CreateInvitationPayload) {
  await requestDocumentJson(`${INVITATIONS}`, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
}

export async function revokeInvitation(id: string) {
  await requestDocumentJson(`${INVITATION_REVOKE(id)}`, {
    method: "POST",
  });
}

export async function resendInvitation(id: string) {
  await requestDocumentJson(`${INVITATION_RESEND(id)}`, {
    method: "POST",
  });
}

// ─────────────────────────────────────────────
// Verify & Send (for registration flow)
// ─────────────────────────────────────────────

export type VerifyInvitationResult = {
  organizationId: string;
  organizationSlug?: string;
  email: string;
  roleIds: string[];
  groupIds: string[];
  expiresAt: string;
};

export async function verifyInvitation(code: string): Promise<VerifyInvitationResult> {
  const response = await requestDocumentJson(
    `${INVITATIONS_VERIFY}?code=${encodeURIComponent(code)}`,
  );
  return response as VerifyInvitationResult;
}

export async function sendInvitation(id: string): Promise<void> {
  await requestDocumentJson(`${INVITATION_SEND(id)}`, {
    method: "POST",
  });
}
