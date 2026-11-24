import { requestDocumentJson } from "@/shared/api/http";
import type { IamListResponse } from "./client";
import {
  REBAC_NAMESPACES,
  REBAC_NAMESPACE_BY_ID,
  REBAC_NAMESPACE_CHECK,
  REBAC_NAMESPACE_EXPAND,
  REBAC_NAMESPACE_TUPLES,
  REBAC_TUPLE_BY_ID,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// ReBAC (Relationship-Based Access Control)
// Domain: relation_namespaces + relation_tuples (ADR-024)
//
// 字段形状与 `internal/transport/http/api/v1/rebac/requests.go` 的 DTO 对齐：
// namespace 用 name/displayName/relationDefs；tuple 用 OpenFGA 的三元组
// subject/relation/object（对象与主体都是字符串，不再拆 objectType/objectId）。
// ─────────────────────────────────────────────

export type ReBACNamespaceItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName?: string;
  relationDefs?: Record<string, unknown>;
  version?: number;
  createdAt?: string;
  updatedAt?: string;
};

export type ReBACTupleItem = {
  id: string;
  organizationId: string;
  namespaceId: string;
  subject: string;
  subjectRelation?: string | null;
  relation: string;
  object: string;
  zookie?: string;
  createdAt: string;
};

export type CreateReBACNamespaceRequest = {
  name: string;
  displayName?: string;
  relationDefs?: Record<string, unknown>;
};

export type CreateReBACTupleRequest = {
  subject: string;
  subjectRelation?: string;
  relation: string;
  object: string;
};

export type ReBACCheckRequest = {
  subject: string;
  relation: string;
  object: string;
  depthLimit?: number;
  zookie?: string;
};

export type ReBACCheckResponse = {
  allowed: boolean;
  trace?: string[];
  zookie: string;
};

export type ReBACExpandRequest = {
  relation: string;
  object: string;
  depthLimit?: number;
  zookie?: string;
};

export type ReBACExpandNode = {
  subject: string;
  expanded?: boolean;
  subjectRelation?: string | null;
  children?: ReBACExpandNode[];
};

export type ReBACExpandResponse = {
  root: ReBACExpandNode;
  zookie: string;
};

export function listReBACNamespaces() {
  return requestDocumentJson<IamListResponse<ReBACNamespaceItem>>(REBAC_NAMESPACES);
}

export function createReBACNamespace(input: CreateReBACNamespaceRequest) {
  return requestDocumentJson<{ namespace: ReBACNamespaceItem }>(REBAC_NAMESPACES, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function getReBACNamespace(id: string) {
  return requestDocumentJson<ReBACNamespaceItem>(REBAC_NAMESPACE_BY_ID(id));
}

export function deleteReBACNamespace(id: string) {
  return requestDocumentJson<void>(REBAC_NAMESPACE_BY_ID(id), { method: "DELETE" });
}

export function listReBACTuples(namespaceId: string) {
  return requestDocumentJson<IamListResponse<ReBACTupleItem>>(REBAC_NAMESPACE_TUPLES(namespaceId));
}

// 后端 POST /rebac/namespaces/{id}/tuples 只接受扁平单条 CreateTupleRequest
// （DisallowUnknownFields），响应为单个 TupleResponse；批量 WriteTuples 未注册路由。
export function createReBACTuple(namespaceId: string, input: CreateReBACTupleRequest) {
  const body: CreateReBACTupleRequest = {
    subject: input.subject,
    relation: input.relation,
    object: input.object,
  };
  if (input.subjectRelation) body.subjectRelation = input.subjectRelation;

  return requestDocumentJson<ReBACTupleItem>(REBAC_NAMESPACE_TUPLES(namespaceId), {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function deleteReBACTuple(id: string) {
  return requestDocumentJson<void>(REBAC_TUPLE_BY_ID(id), { method: "DELETE" });
}

export function checkReBAC(namespaceId: string, input: ReBACCheckRequest) {
  return requestDocumentJson<ReBACCheckResponse>(REBAC_NAMESPACE_CHECK(namespaceId), {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function expandReBAC(namespaceId: string, input: ReBACExpandRequest) {
  return requestDocumentJson<ReBACExpandResponse>(REBAC_NAMESPACE_EXPAND(namespaceId), {
    method: "POST",
    body: JSON.stringify(input),
  });
}
