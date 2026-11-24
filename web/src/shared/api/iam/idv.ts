import { requestDocumentJson } from "@/shared/api/http";
import type { IamListResponse } from "./client";
import { IDV_VERIFICATIONS, IDV_VERIFICATION_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// IDV (Identity Verification)
// Domain: idv_verifications (internal/app/idv/service.go)
// ─────────────────────────────────────────────

export type IDVVerificationItem = {
  id: string;
  organizationId: string;
  userId?: string;
  provider: string;
  status: string;
  redirectUrl?: string;
  // session_token is write-only on the API: reads expose only the set flag and
  // a masked display value, never the provider session token itself.
  sessionTokenSet?: boolean;
  sessionTokenMasked?: string;
  providerRef?: string;
  firstName?: string;
  lastName?: string;
  dateOfBirth?: string;
  nationality?: string;
  idType?: string;
  // id_number is PII: reads expose only a set flag and a masked display value.
  idNumberSet: boolean;
  idNumberMasked?: string;
  confidence?: number;
  reason?: string;
  rawResultSet: boolean;
  rawResultMasked?: string;
  initiatedAt: string;
  completedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type IDVListQuery = {
  organizationId: string;
  userId?: string;
  provider?: string;
  status?: string;
  page?: number;
  pageSize?: number;
};

export async function listIDVVerifications(
  q: IDVListQuery,
): Promise<IamListResponse<IDVVerificationItem> | undefined> {
  const params = new URLSearchParams();
  params.set("organizationId", q.organizationId);
  if (q.userId) params.set("userId", q.userId);
  if (q.provider) params.set("provider", q.provider);
  if (q.status) params.set("status", q.status);
  params.set("page", String(q.page ?? 1));
  params.set("pageSize", String(q.pageSize ?? 20));

  return requestDocumentJson<IamListResponse<IDVVerificationItem>>(
    `${IDV_VERIFICATIONS}?${params.toString()}`,
    { method: "GET" },
  );
}

export async function getIDVVerification(
  id: string,
  organizationId: string,
): Promise<IDVVerificationItem | undefined> {
  return requestDocumentJson<IDVVerificationItem>(
    `${IDV_VERIFICATION_BY_ID(id)}?organizationId=${encodeURIComponent(organizationId)}`,
    { method: "GET" },
  );
}

export async function deleteIDVVerification(id: string, organizationId: string): Promise<void> {
  await requestDocumentJson(IDV_VERIFICATION_BY_ID(id), {
    method: "DELETE",
    body: JSON.stringify({ organizationId }),
  });
}
