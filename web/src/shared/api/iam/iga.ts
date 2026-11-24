import { requestDocumentJson } from "@/shared/api/http";
import { buildQueryString, type IamListResponse } from "./client";
import {
  IGA_ACCESS_CAMPAIGN,
  IGA_ACCESS_CAMPAIGNS,
  IGA_ACCESS_LIFECYCLE,
  IGA_ACCESS_REQUEST_APPROVE,
  IGA_ACCESS_REQUEST_BY_ID,
  IGA_ACCESS_REQUEST_PROVISION,
  IGA_ACCESS_REQUEST_REJECT,
  IGA_ACCESS_REQUESTS,
  IGA_ACCESS_REQUEST_SUBMIT,
  IGA_ACCESS_REQUEST_WITHDRAW,
  IGA_PRIVILEGED_SESSIONS,
  IGA_SOD_RULE,
  IGA_SOD_RULES,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Identity Governance & Administration
// Domain: access_campaigns + sod_rules + access_lifecycle
//
// 字段与 `internal/transport/http/api/v1/iga/handler.go` 的请求/响应 DTO 一一对应。
// 后端 DecodeJSON 拒绝未知字段，因此前端请求体不能携带服务端不认识的键。
// ─────────────────────────────────────────────

export type AccessCampaignStatus = "draft" | "active" | "completed" | "cancelled";

export type AccessCampaignItem = {
  id: string;
  organizationId: string;
  name: string;
  status: AccessCampaignStatus;
  startDate?: string | null;
  endDate?: string | null;
  createdAt: string;
  updatedAt: string;
  revision: number;
};

export type SODRuleItem = {
  id: string;
  organizationId: string;
  name: string;
  roleAId: string;
  roleBId: string;
  createdAt: string;
  updatedAt: string;
  revision: number;
};

export type AccessLifecycleItem = {
  id: string;
  organizationId: string;
  userId: string;
  roleId: string;
  status: string;
  grantedAt: string;
  expiresAt: string;
  createdAt: string;
  revision: number;
};

export type AccessRequestStatus =
  | "submitted"
  | "pending_manager"
  | "pending_sap"
  | "approved"
  | "provisioned"
  | "rejected"
  | "withdrawn";

export type AccessRequestStage = "manager" | "sap";

export type AccessRequestItem = {
  id: string;
  organizationId: string;
  requesterId: string;
  roleId: string;
  applicationId: string;
  enforcerId: string;
  resource: string;
  action: string;
  reason: string;
  status: AccessRequestStatus;
  startsAt: string;
  expiresAt: string;
  managerApproverId: string;
  sapApproverId: string;
  lifecycleId: string;
  createdAt: string;
  updatedAt: string;
  revision: number;
};

export type AccessRequestMessage = {
  id: string;
  ticketId: string;
  authorId: string;
  action: string;
  content: string;
  createdAt: string;
};

export type AccessRequestDetail = {
  request: AccessRequestItem;
  messages: AccessRequestMessage[];
  approvalChain: AccessRequestStage[];
};

export type PrivilegedSessionItem = {
  sessionId: string;
  lifecycleId: string;
  ticketId: string;
  privileged: boolean;
  sessionState: string;
  expiresAt: string;
};

export type CreateAccessRequestRequest = {
  roleId: string;
  applicationId: string;
  enforcerId?: string;
  resource: string;
  action: string;
  reason?: string;
  startsAt: string;
  expiresAt: string;
};

export type AccessRequestCommentInput = {
  revision: number;
  comment?: string;
};

export type AccessRequestStageInput = AccessRequestCommentInput & {
  stage: AccessRequestStage;
};

export type CreateAccessCampaignRequest = {
  name: string;
  startDate?: string;
  endDate?: string;
};

export type CreateSODRuleRequest = {
  name: string;
  roleAId: string;
  roleBId: string;
};

function requireJson<T>(result: T | undefined): T {
  if (result === undefined) throw new Error("The server returned an empty JSON response");
  return result;
}

export function listAccessCampaigns() {
  return requireJson(
    requestDocumentJson<IamListResponse<AccessCampaignItem>>(IGA_ACCESS_CAMPAIGNS),
  );
}

export function createAccessCampaign(input: CreateAccessCampaignRequest) {
  return requireJson(
    requestDocumentJson<AccessCampaignItem>(IGA_ACCESS_CAMPAIGNS, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  );
}

export function getAccessCampaign(id: string) {
  return requireJson(requestDocumentJson<AccessCampaignItem>(IGA_ACCESS_CAMPAIGN(id)));
}

export function setAccessCampaignStatus(id: string, status: AccessCampaignStatus) {
  return requireJson(
    requestDocumentJson<AccessCampaignItem>(`${IGA_ACCESS_CAMPAIGN(id)}/status`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
    }),
  );
}

export function deleteAccessCampaign(id: string) {
  return requestDocumentJson<void>(IGA_ACCESS_CAMPAIGN(id), { method: "DELETE" });
}

export function listSODRules() {
  return requireJson(requestDocumentJson<IamListResponse<SODRuleItem>>(IGA_SOD_RULES));
}

export function createSODRule(input: CreateSODRuleRequest) {
  return requireJson(
    requestDocumentJson<SODRuleItem>(IGA_SOD_RULES, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  );
}

export function getSODRule(id: string) {
  return requireJson(requestDocumentJson<SODRuleItem>(IGA_SOD_RULE(id)));
}

export function updateSODRule(id: string, input: CreateSODRuleRequest) {
  return requireJson(
    requestDocumentJson<SODRuleItem>(IGA_SOD_RULE(id), {
      method: "PATCH",
      body: JSON.stringify(input),
    }),
  );
}

export function deleteSODRule(id: string) {
  return requestDocumentJson<void>(IGA_SOD_RULE(id), { method: "DELETE" });
}

export function listAccessLifecycle() {
  return requireJson(
    requestDocumentJson<IamListResponse<AccessLifecycleItem>>(IGA_ACCESS_LIFECYCLE),
  );
}

// ─────────────────────────────────────────────
// Access requests + privileged sessions
// ─────────────────────────────────────────────

export type ListAccessRequestsParams = {
  requesterId?: string;
  status?: AccessRequestStatus;
  page?: number;
  pageSize?: number;
};

export function listAccessRequests(params: ListAccessRequestsParams = {}) {
  const filters: Record<string, string> = {};
  if (params.requesterId) filters.requesterId = params.requesterId;
  const query = buildQueryString({
    page: params.page,
    pageSize: params.pageSize,
    status: params.status,
    filters,
  });
  return requireJson(
    requestDocumentJson<IamListResponse<AccessRequestItem>>(`${IGA_ACCESS_REQUESTS}?${query}`),
  );
}

export function createAccessRequest(input: CreateAccessRequestRequest) {
  return requireJson(
    requestDocumentJson<AccessRequestItem>(IGA_ACCESS_REQUESTS, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  );
}

export function getAccessRequest(id: string) {
  return requireJson(requestDocumentJson<AccessRequestDetail>(IGA_ACCESS_REQUEST_BY_ID(id)));
}

export function submitAccessRequest(id: string, input: AccessRequestCommentInput) {
  return requireJson(
    requestDocumentJson<AccessRequestItem>(IGA_ACCESS_REQUEST_SUBMIT(id), {
      method: "POST",
      body: JSON.stringify({ revision: input.revision, comment: input.comment ?? "" }),
    }),
  );
}

export function approveAccessRequest(id: string, input: AccessRequestStageInput) {
  return requireJson(
    requestDocumentJson<AccessRequestItem>(IGA_ACCESS_REQUEST_APPROVE(id), {
      method: "POST",
      body: JSON.stringify({
        stage: input.stage,
        revision: input.revision,
        comment: input.comment ?? "",
      }),
    }),
  );
}

export function rejectAccessRequest(id: string, input: AccessRequestStageInput) {
  return requireJson(
    requestDocumentJson<AccessRequestItem>(IGA_ACCESS_REQUEST_REJECT(id), {
      method: "POST",
      body: JSON.stringify({
        stage: input.stage,
        revision: input.revision,
        comment: input.comment ?? "",
      }),
    }),
  );
}

export function withdrawAccessRequest(id: string, input: AccessRequestCommentInput) {
  return requireJson(
    requestDocumentJson<AccessRequestItem>(IGA_ACCESS_REQUEST_WITHDRAW(id), {
      method: "POST",
      body: JSON.stringify({ revision: input.revision, comment: input.comment ?? "" }),
    }),
  );
}

export function provisionAccessRequest(id: string, input: AccessRequestCommentInput) {
  return requireJson(
    requestDocumentJson<AccessRequestItem>(IGA_ACCESS_REQUEST_PROVISION(id), {
      method: "POST",
      body: JSON.stringify({ revision: input.revision }),
    }),
  );
}

export type ListPrivilegedSessionsParams = {
  page?: number;
  pageSize?: number;
};

export function listPrivilegedSessions(params: ListPrivilegedSessionsParams = {}) {
  const query = buildQueryString({ page: params.page, pageSize: params.pageSize });
  return requireJson(
    requestDocumentJson<IamListResponse<PrivilegedSessionItem>>(
      `${IGA_PRIVILEGED_SESSIONS}?${query}`,
    ),
  );
}

export function startPrivilegedSession(input: { requestId: string; revision: number }) {
  return requireJson(
    requestDocumentJson<PrivilegedSessionItem>(IGA_PRIVILEGED_SESSIONS, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  );
}
