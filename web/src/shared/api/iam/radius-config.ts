import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { RADIUS_CONFIGS, RADIUS_CONFIG_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// RADIUS Configs
// ─────────────────────────────────────────────
export type IamRadiusConfigItem = {
  id: string;
  organizationId: string;
  name: string;
  host: string;
  port: number;
  authProtocol: string;
  isEnabled: boolean;
  createdAt: string;
  updatedAt: string;
  revision: number;
};
export type IamRadiusConfigDetail = IamRadiusConfigItem & {
  sharedSecretSet: boolean;
  sharedSecretMasked?: string;
};
export type IamCreateRadiusConfigPayload = {
  organizationId: string;
  name: string;
  host: string;
  port?: number;
  sharedSecret?: string;
  authProtocol?: string;
  isEnabled?: boolean;
};
export type IamPatchRadiusConfigPayload = {
  host?: string;
  port?: number;
  sharedSecret?: string;
  authProtocol?: string;
  isEnabled?: boolean;
  revision: number;
};

export const getIamRadiusConfigs = (params?: IamListParams) =>
  iamList<IamRadiusConfigItem>(RADIUS_CONFIGS, params);
export const getIamRadiusConfigsPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamRadiusConfigItem>(RADIUS_CONFIGS, params);
export const getIamRadiusConfig = (id: string) =>
  iamGet<IamRadiusConfigDetail>(RADIUS_CONFIG_BY_ID(id));
export const createIamRadiusConfig = (p: IamCreateRadiusConfigPayload) =>
  iamCreate(RADIUS_CONFIGS, p);
export const patchIamRadiusConfig = (id: string, p: IamPatchRadiusConfigPayload) =>
  iamPatch(RADIUS_CONFIG_BY_ID(id), p);
export const deleteIamRadiusConfig = (id: string) => iamDelete(RADIUS_CONFIG_BY_ID(id));
