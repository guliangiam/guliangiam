import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  iamUpdate,
  iamPostRaw,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import {
  ENFORCERS,
  ENFORCER_ACTIONS,
  ENFORCER_BATCH_ENFORCE,
  ENFORCER_BY_ID,
  ENFORCER_ENFORCE,
  ENFORCER_IMPORT_POLICIES,
  ENFORCER_OBJECTS,
  ENFORCER_POLICIES,
  ENFORCER_REFRESH_ENGINES,
  ENFORCER_RELOAD,
  ENFORCER_ROLES,
  ENFORCER_SUBJECTS,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Enforcers (Casbin)
// ─────────────────────────────────────────────
export type IamEnforcerItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  modelId: string;
  adapterId: string;
  isEnabled: boolean;
  owner?: string;
  createdAt: string;
  updatedAt: string;
};
export type IamEnforcerDetail = IamEnforcerItem & {
  description?: string;
  revision: number;
};
export type IamCreateEnforcerPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  description?: string;
  modelId?: string;
  adapterId?: string;
  isEnabled?: boolean;
};
export type IamPatchEnforcerPayload = {
  owner?: string;
  displayName?: string;
  description?: string;
  modelId?: string;
  adapterId?: string;
  isEnabled?: boolean;
  revision: number;
};
export type IamPolicyRule = string[];

export const getIamEnforcers = (params?: IamListParams) =>
  iamList<IamEnforcerItem>(ENFORCERS, params);
export const getIamEnforcersPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamEnforcerItem>(ENFORCERS, params);
export const getIamEnforcer = (id: string) => iamGet<IamEnforcerDetail>(ENFORCER_BY_ID(id));
export const createIamEnforcer = (p: IamCreateEnforcerPayload) => iamCreate(ENFORCERS, p);
export const patchIamEnforcer = (id: string, p: IamPatchEnforcerPayload) =>
  iamPatch(ENFORCER_BY_ID(id), p);
export const deleteIamEnforcer = (id: string) => iamDelete(ENFORCER_BY_ID(id));

// Policy management
export const getIamEnforcerPolicies = async (id: string): Promise<IamPolicyRule[]> => {
  const res = await iamGet<{ data: IamPolicyRule[] }>(ENFORCER_POLICIES(id));
  return ((res as Record<string, unknown>).data as IamPolicyRule[]) ?? [];
};

export const addIamEnforcerPolicy = (id: string, pType: string, rules: string[]) =>
  iamCreate(ENFORCER_POLICIES(id), { pType, rules });

export const removeIamEnforcerPolicy = (id: string, pType: string, rules: string[]) =>
  iamDelete(ENFORCER_POLICIES(id), { pType, rules });

export const updateIamEnforcerPolicy = (id: string, oldPolicy: string[], newPolicy: string[]) =>
  iamUpdate(ENFORCER_POLICIES(id), { oldPolicy, newPolicy });

export const refreshIamEnforcerEngines = () => iamPostRaw(ENFORCER_REFRESH_ENGINES);

export const reloadIamEnforcerPolicy = (id: string) => iamPostRaw(ENFORCER_RELOAD(id));

// ─────────────────────────────────────────────
// Structured Policy API (Policy type)
// ─────────────────────────────────────────────
export type Policy = {
  ptype: string;
  subject: string;
  object: string;
  action: string;
};

export const getEnforcerPolicies = async (enforcerId: string): Promise<Policy[]> => {
  const raw = await getIamEnforcerPolicies(enforcerId);
  return raw.map((row) => ({
    ptype: row[0] ?? "p",
    subject: row[1] ?? "",
    object: row[2] ?? "",
    action: row[3] ?? "",
  }));
};

export const addEnforcerPolicy = (enforcerId: string, policy: Policy) =>
  iamCreate(ENFORCER_POLICIES(enforcerId), {
    pType: policy.ptype,
    rules: [policy.subject, policy.object, policy.action],
  });

export const removeEnforcerPolicy = (enforcerId: string, policy: Policy) =>
  // The rule is identified in the body (`PolicyRequest`), matching
  // addEnforcerPolicy. There is no per-index endpoint.
  iamDelete(ENFORCER_POLICIES(enforcerId), {
    pType: policy.ptype,
    rules: [policy.subject, policy.object, policy.action],
  });

export const updateEnforcerPolicy = (enforcerId: string, oldPolicy: Policy, newPolicy: Policy) =>
  // `oldPolicy`/`newPolicy` are the full Casbin rows, ptype included: the
  // handler passes them straight to Enforcer.UpdatePolicy, which matches on the
  // whole rule. Sending only [subject, object, action] would match nothing.
  iamUpdate(ENFORCER_POLICIES(enforcerId), {
    oldPolicy: [oldPolicy.ptype, oldPolicy.subject, oldPolicy.object, oldPolicy.action],
    newPolicy: [newPolicy.ptype, newPolicy.subject, newPolicy.object, newPolicy.action],
  });

/**
 * Atomically replace all policies of an enforcer with the given list.
 *
 * Backed by `POST /api/v1/enforcers/{id}/import-policies`, which is the only
 * endpoint with replace-all semantics. Rows are the full Casbin rules (ptype
 * first) because the service hands each row straight to Casbin.AddPolicy.
 * `PUT /api/v1/enforcers/{id}/policies` is *not* replace-all: it decodes
 * `{oldPolicy, newPolicy}` and renames a single rule.
 */
export const replaceEnforcerPolicies = (enforcerId: string, policies: Policy[]) =>
  iamCreate(ENFORCER_IMPORT_POLICIES(enforcerId), {
    policies: policies.map((p) => [p.ptype, p.subject, p.object, p.action]),
  });

// ─────────────────────────────────────────────
// Enforce & Batch Enforce
// ─────────────────────────────────────────────

export type EnforceRequest = {
  pType?: string;
  v0: string;
  v1: string;
  v2?: string;
  v3?: string;
  v4?: string;
  v5?: string;
};

export type BatchEnforceRequest = {
  requests: EnforceRequest[];
};

export const enforceIamEnforcer = async (id: string, req: EnforceRequest): Promise<boolean> => {
  const res = await iamPostRaw(ENFORCER_ENFORCE(id), req);
  return (res as Record<string, unknown>).data === true || res === true;
};

export const batchEnforceIamEnforcer = async (
  id: string,
  req: BatchEnforceRequest,
): Promise<boolean[]> => {
  const res = await iamPostRaw(ENFORCER_BATCH_ENFORCE(id), req);
  return ((res as Record<string, unknown>).data as boolean[]) ?? [];
};

// ─────────────────────────────────────────────
// Enforcer metadata (subjects, objects, actions, roles)
// ─────────────────────────────────────────────

export const getIamEnforcerSubjects = async (id: string): Promise<string[]> => {
  const res = await iamGet<{ data: string[] }>(ENFORCER_SUBJECTS(id));
  return ((res as Record<string, unknown>).data as string[]) ?? [];
};

export const getIamEnforcerObjects = async (id: string): Promise<string[]> => {
  const res = await iamGet<{ data: string[] }>(ENFORCER_OBJECTS(id));
  return ((res as Record<string, unknown>).data as string[]) ?? [];
};

export const getIamEnforcerActions = async (id: string): Promise<string[]> => {
  const res = await iamGet<{ data: string[] }>(ENFORCER_ACTIONS(id));
  return ((res as Record<string, unknown>).data as string[]) ?? [];
};

export const getIamEnforcerRoles = async (id: string): Promise<string[]> => {
  const res = await iamGet<{ data: string[] }>(ENFORCER_ROLES(id));
  return ((res as Record<string, unknown>).data as string[]) ?? [];
};
