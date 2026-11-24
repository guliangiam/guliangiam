import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import { pageMetaSchema } from "./schemas";
import { ORGANIZATION_DOMAINS } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// B2B 域名治理 API — functional.md §15 MODULE-B2B
// 后端：internal/transport/http/api/v1/b2b/request.go
// ─────────────────────────────────────────────

// ── Schema ────────────────────────────────────

export const domainStatusSchema = z.enum(["pending", "verified", "failed"]);

export const domainResponseSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  domain: z.string(),
  verificationToken: z.string(),
  status: domainStatusSchema,
  verifiedAt: z.string().optional(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const listDomainsResponseSchema = z.object({
  items: z.array(domainResponseSchema),
  page: pageMetaSchema,
});

export type DomainStatus = z.infer<typeof domainStatusSchema>;
export type DomainResponse = z.infer<typeof domainResponseSchema>;
export type ListDomainsResponse = z.infer<typeof listDomainsResponseSchema>;

// ── Types ─────────────────────────────────────

export type ListDomainsParams = {
  organizationId: string;
  page?: number;
  pageSize?: number;
  status?: DomainStatus;
};

export type ClaimDomainInput = {
  organizationId: string;
  domain: string;
};

// ── API functions ─────────────────────────────

export async function listDomains(params: ListDomainsParams): Promise<ListDomainsResponse> {
  const search = new URLSearchParams();
  if (params.page !== undefined) search.set("page", String(params.page));
  if (params.pageSize !== undefined) search.set("pageSize", String(params.pageSize));
  if (params.status) search.set("status", params.status);
  const url = `${ORGANIZATION_DOMAINS(params.organizationId)}${search.toString() ? `?${search.toString()}` : ""}`;
  const data = await requestDocumentJson(url, { method: "GET" });
  return listDomainsResponseSchema.parse(data);
}

export async function claimDomain(input: ClaimDomainInput): Promise<DomainResponse> {
  const data = await requestDocumentJson(ORGANIZATION_DOMAINS(input.organizationId), {
    method: "POST",
    body: JSON.stringify({ domain: input.domain }),
  });
  const wrapped = z.object({ domain: domainResponseSchema }).parse(data);
  return wrapped.domain;
}

export async function deleteDomain(organizationId: string, domainId: string): Promise<void> {
  await requestDocumentJson(`${ORGANIZATION_DOMAINS(organizationId)}/${domainId}`, {
    method: "DELETE",
  });
}

export async function verifyDomain(
  organizationId: string,
  domainId: string,
): Promise<{ domain: DomainResponse; verified: boolean }> {
  const data = await requestDocumentJson(
    `${ORGANIZATION_DOMAINS(organizationId)}/${domainId}/verify`,
    {
      method: "POST",
    },
  );
  return z.object({ domain: domainResponseSchema, verified: z.boolean() }).parse(data);
}
