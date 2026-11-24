// =============================================================================
// Commerce Channel Configs API client
// =============================================================================
//
// 后端端点: /api/v1/pay-channel-configs (RegisterChannelConfigChiRoutes)
//
//   GET    /api/v1/pay-channel-configs               List by organizationId
//   POST   /api/v1/pay-channel-configs               Create (auto-versions per org+channel+env)
//   GET    /api/v1/pay-channel-configs/:id           Get by ID
//   POST   /api/v1/pay-channel-configs/:id:transition  State-machine transition
//
// 设计文档: G6 §P2 版本化渠道配置管理
// =============================================================================

import { requestDocumentJson } from "@/shared/api/http";
import type { Paginated } from "@/shared/api/pagination";
import { iamList, iamListWithPagination, type IamListParams } from "@/shared/api/iam/client";
import {
  PAY_CHANNELS,
  PAY_CHANNEL_CONFIGS,
  PAY_CHANNEL_CONFIG_BY_ID,
  PAY_CHANNEL_CONFIG_TRANSITION,
  PAY_RECON,
  PAY_RECON_BATCHES,
  PAY_RECON_BATCH_ITEMS,
  PAY_RECON_ITEM_APPROVE,
  PAY_RECON_ITEM_RESOLVE,
  PAYMENT_ATTEMPTS,
  PAY_ATTEMPT_BY_ID,
  PAYMENT_REFUNDS,
  PAY_REFUND_BY_ID,
  PAY_CALLBACK_RECEIPTS,
} from "@/shared/api/paths";

// ── Types ────────────────────────────────────────────────────────────────────

export type ChannelConfigStatus = "draft" | "testing" | "active" | "disabled";

export interface ChannelConfigResponse {
  id: string;
  organizationId: string;
  channel: string;
  environment: string; // sandbox | production
  configFingerprint: string;
  configSet: string[];
  versionNo: number;
  status: ChannelConfigStatus;
  testPassedAt?: string | null;
  activatedAt?: string | null;
  disabledAt?: string | null;
  createdAt: string;
  updatedAt: string;
  revision: number;
}

export interface CreateChannelConfigBody {
  id?: string;
  organizationId: string;
  channel: string;
  environment?: string;
  /**
   * Plaintext JSON object. The server canonicalizes it, derives configSet from
   * its top-level keys and seals it; clients never send a pre-encrypted blob.
   */
  config: Record<string, unknown>;
}

export interface TransitionChannelConfigBody {
  status: ChannelConfigStatus;
}

// ── List ─────────────────────────────────────────────────────────────────────

export function listChannelConfigs(
  params?: IamListParams,
): Promise<{ items: ChannelConfigResponse[]; total: number } | undefined> {
  const p = { ...params };
  // 这个端点是无分页的 List —— 全部返回
  return iamListWithPagination<ChannelConfigResponse>(PAY_CHANNEL_CONFIGS, p);
}

// ── Create ──────────────────────────────────────────────────────────────────

export function createChannelConfig(
  body: CreateChannelConfigBody,
): Promise<ChannelConfigResponse | undefined> {
  return requestDocumentJson<ChannelConfigResponse>(PAY_CHANNEL_CONFIGS, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

// ── Get ─────────────────────────────────────────────────────────────────────

export function getChannelConfig(id: string): Promise<ChannelConfigResponse | undefined> {
  return requestDocumentJson<ChannelConfigResponse>(PAY_CHANNEL_CONFIG_BY_ID(id));
}

// ── Transition (state machine) ──────────────────────────────────────────────

export function transitionChannelConfig(
  id: string,
  status: ChannelConfigStatus,
): Promise<ChannelConfigResponse | undefined> {
  return requestDocumentJson<ChannelConfigResponse>(PAY_CHANNEL_CONFIG_TRANSITION(id), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ status }),
  });
}

// ── Channel metadata (read-only capability catalog) ────────────────────────

// The capability matrix lives in Go (infra/payment/channel_catalog.go) and is
// exposed through GET /api/v1/pay-channels. Do not hand-maintain a second list.
export interface PayChannelCapability {
  channel: string;
  displayName: string;
  supportsCharge: boolean;
  supportsRefund: boolean;
  supportsQuery: boolean;
  supportsClose: boolean;
  supportsCallback: boolean;
}

export function listPayChannels(): Promise<PayChannelCapability[]> {
  return iamList<PayChannelCapability>(PAY_CHANNELS);
}

export const CHANNEL_STATUS_OPTIONS: Array<{
  value: ChannelConfigStatus;
  label: string;
  color: string;
}> = [
  { value: "draft", label: "Draft", color: "default" },
  { value: "testing", label: "Testing", color: "gold" },
  { value: "active", label: "Active", color: "green" },
  { value: "disabled", label: "Disabled", color: "red" },
];

export const ENVIRONMENT_OPTIONS = [
  { value: "sandbox", label: "Sandbox", color: "gold" },
  { value: "production", label: "Production", color: "blue" },
];

// =============================================================================
// Commerce — Reconciliation (对账)
// =============================================================================
//
// 后端端点: RegisterReconChiRoutes (design doc §8.2)
//
//   GET    /api/v1/pay-recon-batches               批次列表（分页）
//   GET    /api/v1/pay-recon-batches/:id/items     指定批次差异项列表
//   POST   /api/v1/pay-recon-items/:id:resolve     人工处置一条差异项
//   POST   /api/v1/pay-recon:run                   手动触发对账
// =============================================================================

export type ReconBatchStatus = "pending" | "running" | "completed" | "failed";
export type ReconDiffType =
  | "local_only"
  | "channel_only"
  | "amount_mismatch"
  | "state_mismatch"
  | "channel_unknown";

export interface ReconBatchListItem {
  id: string;
  organizationId: string;
  channel: string;
  batchDate: string; // "YYYY-MM-DD"
  status: ReconBatchStatus;
  diffCount: number;
  totalLocal: number;
  totalChannel: number;
  createdAt: string;
  updatedAt: string;
}

export interface ReconItemView {
  id: string;
  batchId: string;
  diffType: ReconDiffType;
  channelOrderNo?: string | null;
  localPaymentId?: string | null;
  channelAmount?: number | null;
  localAmount?: number | null;
  channelState?: string | null;
  localState?: string | null;
  resolved: boolean;
  resolutionState?: string | null;
  approvedBy?: string | null;
  resolvedBy?: string | null;
  createdAt: string;
}

// 与 Go internal/transport/http/presenter.ListResponse 保持一致:
//   { items: [...], page: { page, pageSize, total } }
// 类型定义唯一源在 shared/api/pagination.ts。
export type ListResponse<T> = Paginated<T>;

export interface ResolveReconItemBody {
  operator: string;
  note?: string;
}

// ── List batches ──────────────────────────────────────────────────────────

export function listReconBatches(params?: {
  organizationId?: string;
  page?: number;
  pageSize?: number;
}): Promise<ListResponse<ReconBatchListItem> | undefined> {
  const qs = new URLSearchParams();
  if (params?.organizationId) qs.set("organizationId", params.organizationId);
  qs.set("page", String(params?.page ?? 1));
  qs.set("pageSize", String(params?.pageSize ?? 20));
  return requestDocumentJson<ListResponse<ReconBatchListItem>>(
    `${PAY_RECON_BATCHES}?${qs.toString()}`,
  );
}

// ── List items (diff details for one batch) ────────────────────────────────

export function listReconItems(batchId: string): Promise<ReconItemView[] | undefined> {
  return requestDocumentJson<ReconItemView[]>(PAY_RECON_BATCH_ITEMS(batchId));
}

// ── Resolve one item ──────────────────────────────────────────────────────

export function approveReconItem(
  id: string,
  body: { reason?: string } = {},
): Promise<{ status: string } | undefined> {
  return requestDocumentJson<{ status: string }>(PAY_RECON_ITEM_APPROVE(id), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function resolveReconItem(
  id: string,
  body: ResolveReconItemBody,
): Promise<{ status: string } | undefined> {
  return requestDocumentJson<{ status: string }>(PAY_RECON_ITEM_RESOLVE(id), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

// ── Payments History (attempts + refunds) ────────────────────────────────
// 后端端点 (design doc G5):
//   GET /api/v1/payments/{paymentId}/attempts   Attempts 时间线
//   GET /api/v1/pay-attempts/{id}              单个 attempt 详情
//   GET /api/v1/payments/{paymentId}/refunds   退款列表
//   GET /api/v1/pay-refunds/{id}               单个退款详情

export interface AttemptListItem {
  id: string;
  paymentId: string;
  refundId?: string | null;
  channel: string;
  attemptType: string;
  attemptNo: number;
  channelOrderNo?: string | null;
  channelRequestId?: string | null;
  state: string;
  amount?: number | null;
  currency?: string | null;
  errorCode?: string | null;
  errorMessage?: string | null;
  latencyMs?: number | null;
  occurredAt: string;
}

export interface RefundListItem {
  id: string;
  paymentId: string;
  outRefundNo: string;
  channel: string;
  amount: number;
  currency: string;
  state: string;
  reason: string;
  channelRefundNo: string;
  errorCode?: string | null;
  errorMessage?: string | null;
  requestedAt: string;
  completedAt?: string | null;
  createdAt: string;
  updatedAt: string;
}

export function listAttemptsByPayment(
  paymentId: string,
  params?: { page?: number; pageSize?: number },
): Promise<ListResponse<AttemptListItem> | undefined> {
  const qs = new URLSearchParams();
  qs.set("page", String(params?.page ?? 1));
  qs.set("pageSize", String(params?.pageSize ?? 20));
  return requestDocumentJson<ListResponse<AttemptListItem>>(
    `${PAYMENT_ATTEMPTS(paymentId)}?${qs.toString()}`,
  );
}

export function getAttempt(id: string): Promise<AttemptListItem | undefined> {
  return requestDocumentJson<AttemptListItem>(PAY_ATTEMPT_BY_ID(id));
}

export function listRefundsByPayment(
  paymentId: string,
  params?: { page?: number; pageSize?: number },
): Promise<ListResponse<RefundListItem> | undefined> {
  const qs = new URLSearchParams();
  qs.set("page", String(params?.page ?? 1));
  qs.set("pageSize", String(params?.pageSize ?? 20));
  return requestDocumentJson<ListResponse<RefundListItem>>(
    `${PAYMENT_REFUNDS(paymentId)}?${qs.toString()}`,
  );
}

export function getRefund(id: string): Promise<RefundListItem | undefined> {
  return requestDocumentJson<RefundListItem>(PAY_REFUND_BY_ID(id));
}

// ── Run reconciliation ────────────────────────────────────────────────────
// 两种模式:
//   - batchId 指定 → 触发已存在 pending batch 立即执行
//   - batchId 空 + organizationId + channel + batchDate → 先 CreateBatch 再执行

export function runRecon(params: {
  batchId?: string;
  organizationId?: string;
  channel?: string;
  batchDate?: string;
}): Promise<{ status: string; batchId: string } | undefined> {
  return requestDocumentJson<{ status: string; batchId: string }>(`${PAY_RECON}:run`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params),
  });
}

// ── Static metadata ───────────────────────────────────────────────────────

export const RECON_BATCH_STATUS: Array<{
  value: ReconBatchStatus;
  label: string;
  color: string;
}> = [
  { value: "pending", label: "Pending", color: "default" },
  { value: "running", label: "Running", color: "processing" },
  { value: "completed", label: "Completed", color: "success" },
  { value: "failed", label: "Failed", color: "error" },
];

export const RECON_DIFF_TYPE: Array<{
  value: ReconDiffType;
  label: string;
  color: string;
}> = [
  { value: "local_only", label: "Local Only", color: "blue" },
  { value: "channel_only", label: "Channel Only", color: "orange" },
  { value: "amount_mismatch", label: "Amount Mismatch", color: "red" },
  { value: "state_mismatch", label: "State Mismatch", color: "purple" },
  { value: "channel_unknown", label: "Channel Unknown", color: "default" },
];

// ── Callback Receipts (G7 渠道回调审计) ─────────────────────────────
// 后端: GET /api/v1/pay-callback-receipts

export interface CallbackReceiptListItem {
  id: string;
  channel: string;
  payloadSha256: string;
  receivedAt: string;
  clientIp?: string | null;
  processState: string;
  eventKey?: string | null;
  outTradeNo?: string | null;
  errorMessage?: string | null;
  headers?: Record<string, unknown>;
}

export function listCallbackReceipts(params: {
  organizationId: string;
  page?: number;
  pageSize?: number;
}): Promise<ListResponse<CallbackReceiptListItem> | undefined> {
  const qs = new URLSearchParams();
  qs.set("organizationId", params.organizationId);
  qs.set("page", String(params.page ?? 1));
  qs.set("pageSize", String(params.pageSize ?? 20));
  return requestDocumentJson<ListResponse<CallbackReceiptListItem>>(
    `${PAY_CALLBACK_RECEIPTS}?${qs.toString()}`,
  );
}
