import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { AGENTS, AGENT_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Agents
// ─────────────────────────────────────────────
export type IamAgentItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  host: string;
  port: number;
  url?: string;
  token?: string;
  applicationId?: string;
  isEnabled: boolean;
  lastPingAt?: string | null;
  createdAt: string;
  updatedAt: string;
  revision: number;
};
export type IamAgentDetail = IamAgentItem & {};
export type IamCreateAgentPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  host: string;
  port?: number;
  url?: string;
  token?: string;
  applicationId?: string;
  isEnabled?: boolean;
};
export type IamPatchAgentPayload = {
  displayName?: string;
  host?: string;
  port?: number;
  url?: string;
  token?: string;
  applicationId?: string;
  isEnabled?: boolean;
  revision: number;
};

export const getIamAgents = (params?: IamListParams) => iamList<IamAgentItem>(AGENTS, params);
export const getIamAgentsPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamAgentItem>(AGENTS, params);
export const getIamAgent = (id: string) => iamGet<IamAgentDetail>(AGENT_BY_ID(id));
export const createIamAgent = (p: IamCreateAgentPayload) => iamCreate(AGENTS, p);
export const patchIamAgent = (id: string, p: IamPatchAgentPayload) => iamPatch(AGENT_BY_ID(id), p);
export const deleteIamAgent = (id: string) => iamDelete(AGENT_BY_ID(id));
