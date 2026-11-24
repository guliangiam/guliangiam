import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { requestDocumentJson } from "@/shared/api/http";
import { ADAPTERS, ADAPTER_BY_ID, ADAPTER_POLICIES } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Adapters (Casbin)
// ─────────────────────────────────────────────
export type IamAdapterItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  server?: string;
  database?: string;
  owner?: string;
  useSameDb: boolean;
  isEnabled: boolean;
  createdAt: string;
  updatedAt: string;
  revision: number;
};
export type IamAdapterDetail = IamAdapterItem & {
  description?: string;
  tableName?: string;
  databaseType?: string;
  host?: string;
  port?: number;
  dbUser?: string;
  dbPasswordSet: boolean;
  dbPasswordMasked?: string;
  databaseName?: string;
  policy: string;
};
export type IamCreateAdapterPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  policy?: string;
  isEnabled?: boolean;
};
export type IamPatchAdapterPayload = {
  owner?: string;
  displayName?: string;
  policy?: string;
  tableName?: string;
  databaseType?: string;
  useSameDb?: boolean;
  host?: string;
  port?: number;
  dbUser?: string;
  dbPassword?: string;
  databaseName?: string;
  isEnabled?: boolean;
  revision: number;
};

export const getIamAdapters = (params?: IamListParams) => iamList<IamAdapterItem>(ADAPTERS, params);
export const getIamAdaptersPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamAdapterItem>(ADAPTERS, params);
export const getIamAdapter = (id: string) => iamGet<IamAdapterDetail>(`${ADAPTER_BY_ID(id)}`);
export const createIamAdapter = (p: IamCreateAdapterPayload) => iamCreate(ADAPTERS, p);
export const patchIamAdapter = (id: string, p: IamPatchAdapterPayload) =>
  iamPatch(`${ADAPTER_BY_ID(id)}`, p);
export const deleteIamAdapter = (id: string) => iamDelete(`${ADAPTER_BY_ID(id)}`);

// ─── Policy operations ──────────────────────────────────────────
export type PolicyRule = string[];

export async function getAdapterPolicies(
  id: string,
  fieldIndex?: number,
  fieldValue?: string[],
): Promise<PolicyRule[]> {
  const params = new URLSearchParams();
  if (fieldIndex !== undefined) params.set("fieldIndex", String(fieldIndex));
  if (fieldValue?.length) fieldValue.forEach((v) => params.append("fieldValue", v));
  const qs = params.toString();
  const parsed = await requestDocumentJson<PolicyRule[]>(
    `${ADAPTER_POLICIES(id)}${qs ? `?${qs}` : ""}`,
  );
  return (parsed ?? []) as PolicyRule[];
}

export async function addAdapterPolicy(id: string, pType: string, rules: string[]) {
  await requestDocumentJson(`${ADAPTER_POLICIES(id)}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ pType, rules }),
  });
}

export async function removeAdapterPolicy(id: string, pType: string, rules: string[]) {
  await requestDocumentJson(`${ADAPTER_POLICIES(id)}`, {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ pType, rules }),
  });
}

export async function updateAdapterPolicy(id: string, oldPolicy: string[], newPolicy: string[]) {
  await requestDocumentJson(`${ADAPTER_POLICIES(id)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ oldPolicy, newPolicy }),
  });
}
