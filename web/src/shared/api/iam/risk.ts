import { requestDocumentJson } from "@/shared/api/http";
import { RISK_DECISION_ACTIONS, RISK_GOVERNANCE_ACTIONS } from "@/shared/constants/enums";
import type { IamListResponse } from "./client";
import {
  RISK_DECISIONS,
  RISK_DECISION_REVIEW,
  RISK_SESSION_GOVERNANCE,
  RISK_SIGNALS,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Adaptive Risk
// Domain: risk_signals + risk_decisions + session_governance
// ─────────────────────────────────────────────

// 字段与后端 DTO 一一对应（internal/app/risk/service.go SignalDTO/DecisionDTO/
// SessionGovernanceDTO）。旧类型里的 severity/source/occurredAt/userKey 等字段
// 服务端从未返回，曾导致列表渲染 Invalid Date 与空列。
export type RiskSignalItem = {
  id: string;
  organizationId: string;
  userId?: string;
  signalType: string;
  signalValue: unknown;
  ip?: string;
  createdAt: string;
};

// The decision and action unions are derived from the server's closed sets
// (`@enumset` in shared/constants/enums.ts). The console used to offer
// `step_up` as a decision, which the server has never accepted (手册 4.14.3:
// an unknown value is 422, so every review submitted from the console failed).
export type RiskDecisionAction = (typeof RISK_DECISION_ACTIONS)[number];
export type RiskGovernanceAction = (typeof RISK_GOVERNANCE_ACTIONS)[number];

export type DecisionReview = {
  status: string;
  actorId?: string;
  reason?: string;
  fromDecision?: RiskDecisionAction;
  toDecision?: RiskDecisionAction;
  reviewedAt?: string;
};

export type RiskDecisionItem = {
  id: string;
  organizationId: string;
  userId?: string;
  score: number;
  decision: RiskDecisionAction;
  factors: unknown;
  createdAt: string;
  revision: number;
  workflow: {
    revision: number;
    review: DecisionReview;
    appeal: { status: string; reason?: string };
    history: unknown[];
  };
};

export type SessionGovernanceItem = {
  id: string;
  organizationId: string;
  sessionId?: string;
  riskScore: number;
  action: RiskGovernanceAction;
  reason?: string;
  createdAt: string;
  revision: number;
};

// 后端 review 契约：reason 与 revision 均必填（validation.go validateReason /
// validateExpectedRevision），revision 取列表 DTO 顶层当前值做乐观并发。
export type ReviewRiskDecisionRequest = {
  decision: RiskDecisionAction;
  reason: string;
  revision: number;
};

export function listRiskSignals() {
  return requestDocumentJson<IamListResponse<RiskSignalItem>>(RISK_SIGNALS);
}

export function listRiskDecisions() {
  return requestDocumentJson<IamListResponse<RiskDecisionItem>>(RISK_DECISIONS);
}

export function reviewRiskDecision(id: string, input: ReviewRiskDecisionRequest) {
  return requestDocumentJson<RiskDecisionItem>(RISK_DECISION_REVIEW(id), {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function listSessionGovernance() {
  return requestDocumentJson<IamListResponse<SessionGovernanceItem>>(RISK_SESSION_GOVERNANCE);
}
