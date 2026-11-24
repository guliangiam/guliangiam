import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { FORMS, FORM_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Forms
// ─────────────────────────────────────────────
export type IamFormItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  isEnabled: boolean;
  createdAt: string;
  updatedAt: string;
};
export type IamFormDetail = IamFormItem & {
  properties: string;
  application: string;
  revision: number;
};
export type IamCreateFormPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  type?: string;
  properties?: string;
  isEnabled?: boolean;
};
export type IamPatchFormPayload = {
  displayName?: string;
  type?: string;
  properties?: string;
  isEnabled?: boolean;
  revision: number;
};

export const getIamForms = (params?: IamListParams) => iamList<IamFormItem>(FORMS, params);
export const getIamFormsPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamFormItem>(FORMS, params);
export const getIamForm = (id: string) => iamGet<IamFormDetail>(FORM_BY_ID(id));
export const createIamForm = (p: IamCreateFormPayload) => iamCreate(FORMS, p);
export const patchIamForm = (id: string, p: IamPatchFormPayload) => iamPatch(FORM_BY_ID(id), p);
export const deleteIamForm = (id: string) => iamDelete(FORM_BY_ID(id));
