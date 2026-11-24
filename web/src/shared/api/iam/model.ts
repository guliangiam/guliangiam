import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { MODELS, MODEL_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Models (Casbin)
// ─────────────────────────────────────────────
/** 与后端 app/models.ModelListItem 一致：没有 `type`，模型正文叫 modelText。 */
export type IamModelItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  description?: string;
  owner?: string;
  isEnabled: boolean;
  createdAt: string;
  updatedAt: string;
};
export type IamModelDetail = IamModelItem & {
  modelText: string;
  revision: number;
};
export type IamCreateModelPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  description?: string;
  modelText?: string;
  owner?: string;
  isEnabled?: boolean;
};
export type IamPatchModelPayload = {
  displayName?: string;
  description?: string;
  modelText?: string;
  owner?: string;
  isEnabled?: boolean;
  revision: number;
};

export const getIamModels = (params?: IamListParams) => iamList<IamModelItem>(MODELS, params);
export const getIamModelsPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamModelItem>(MODELS, params);
export const getIamModel = (id: string) => iamGet<IamModelDetail>(MODEL_BY_ID(id));
export const createIamModel = (p: IamCreateModelPayload) => iamCreate(MODELS, p);
export const patchIamModel = (id: string, p: IamPatchModelPayload) => iamPatch(MODEL_BY_ID(id), p);
export const deleteIamModel = (id: string) => iamDelete(MODEL_BY_ID(id));
