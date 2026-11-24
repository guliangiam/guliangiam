import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { RESOURCES, RESOURCE_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Resources
// ─────────────────────────────────────────────
// Mirrors `appresources.ResourceListItem`. The console used to declare
// `application` and `isEnabled`; the server sends `applicationId` and has no
// enabled flag at all (the `resources` table has no `is_enabled` column), so the
// list column and the filter built on it could never work.
export type IamResourceItem = {
  id: string;
  organizationId: string;
  applicationId?: string;
  name: string;
  displayName: string;
  type: string;
  tag?: string;
  filePath: string;
  fileSize: number;
  mimeType?: string;
  createdAt: string;
  updatedAt: string;
  revision: number;
};
export type IamResourceDetail = IamResourceItem;
export type IamCreateResourcePayload = {
  organizationId: string;
  name: string;
  displayName: string;
  type?: string;
  applicationId?: string;
  tag?: string;
  filePath?: string;
  fileSize?: number;
  mimeType?: string;
};
export type IamPatchResourcePayload = {
  displayName?: string;
  tag?: string;
  mimeType?: string;
  revision: number;
};

export const getIamResources = (params?: IamListParams) =>
  iamList<IamResourceItem>(RESOURCES, params);
export const getIamResourcesPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamResourceItem>(RESOURCES, params);
export const getIamResource = (id: string) => iamGet<IamResourceDetail>(RESOURCE_BY_ID(id));
export const createIamResource = (p: IamCreateResourcePayload) => iamCreate(RESOURCES, p);
export const patchIamResource = (id: string, p: IamPatchResourcePayload) =>
  iamPatch(RESOURCE_BY_ID(id), p);
export const deleteIamResource = (id: string) => iamDelete(RESOURCE_BY_ID(id));
