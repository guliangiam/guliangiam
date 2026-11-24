import { requestDocumentJson } from "@/shared/api/http";
import type { IamListResponse } from "./client";
import {
  ACTIONS,
  ACTION_BY_ID,
  ACTION_EXECUTION_LOGS,
  ACTION_RUN,
  ACTION_VERSIONS,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Identity Actions (Sandboxed Executions)
// Domain: identity_actions + action_versions + execution_logs
//
// 字段与 `internal/app/actions/service.go` 的 DTO 对齐：
// Action 用 triggerType/status/revision；版本只回传 code；执行日志用
// version/executedAt/error（不再有 sandboxType/latestVersion/exitCode/durationMs）。
// ─────────────────────────────────────────────

export type ActionStatus = "draft" | "active" | "disabled" | "deleted";

export type ActionTriggerType = "pre_login" | "pre_token" | "post_registration" | "consent" | "mfa";

export type ActionItem = {
  id: string;
  organizationId: string;
  name: string;
  triggerType: ActionTriggerType | string;
  status: ActionStatus | string;
  createdAt: string;
  updatedAt: string;
  revision: number;
};

export type ActionVersionItem = {
  id: string;
  actionId: string;
  version: number;
  code: string;
  createdAt: string;
};

export type ActionExecutionLogItem = {
  id: string;
  actionId: string;
  version: number;
  status: string;
  input?: unknown;
  output?: unknown;
  error?: string;
  executedAt?: string;
  createdAt: string;
};

export type CreateActionRequest = {
  name: string;
  triggerType: ActionTriggerType | string;
  code: string;
};

export type RunActionRequest = {
  input?: Record<string, unknown>;
};

export function listActions() {
  return requestDocumentJson<IamListResponse<ActionItem>>(ACTIONS);
}

export function createAction(input: CreateActionRequest) {
  return requestDocumentJson<ActionItem>(ACTIONS, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function getAction(id: string) {
  return requestDocumentJson<ActionItem>(ACTION_BY_ID(id));
}

export function updateActionStatus(actionId: string, status: ActionStatus) {
  return requestDocumentJson<ActionItem>(`${ACTION_BY_ID(actionId)}/status`, {
    method: "PATCH",
    body: JSON.stringify({ status }),
  });
}

export function listActionVersions(actionId: string) {
  return requestDocumentJson<IamListResponse<ActionVersionItem>>(ACTION_VERSIONS(actionId));
}

export function publishActionVersion(actionId: string, code: string) {
  return requestDocumentJson<ActionVersionItem>(ACTION_VERSIONS(actionId), {
    method: "POST",
    body: JSON.stringify({ code }),
  });
}

export function listActionExecutionLogs(actionId: string) {
  return requestDocumentJson<IamListResponse<ActionExecutionLogItem>>(
    ACTION_EXECUTION_LOGS(actionId),
  );
}

export function runAction(actionId: string, input?: RunActionRequest) {
  return requestDocumentJson<ActionExecutionLogItem>(ACTION_RUN(actionId), {
    method: "POST",
    body: input ? JSON.stringify(input) : undefined,
  });
}
