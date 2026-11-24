import {
  iamList,
  iamGet,
  iamCreate,
  iamDelete,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { RADIUS_ACCOUNTINGS, RADIUS_ACCOUNTING_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Radius Accountings
// ─────────────────────────────────────────────
export type IamRadiusAccountingItem = {
  id: string;
  organizationId: string;
  name: string;
  username: string;
  acctSessionId: string;
  acctStartTime: string;
  createdAt: string;
};
export type IamRadiusAccountingDetail = IamRadiusAccountingItem & {
  serviceType: number;
  nasId: string;
  nasIpAddr: string;
  acctSessionTime: number;
  acctInputTotal: number;
  acctOutputTotal: number;
  acctTerminateCause: number;
  acctStopTime: string | null;
  revision: number;
};
export type IamCreateRadiusAccountingPayload = {
  organizationId: string;
  name: string;
  username?: string;
  acctSessionId?: string;
  acctStartTime?: string;
};

export const getIamRadiusAccountings = (params?: IamListParams) =>
  iamList<IamRadiusAccountingItem>(RADIUS_ACCOUNTINGS, params);
export const getIamRadiusAccountingsPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamRadiusAccountingItem>(RADIUS_ACCOUNTINGS, params);
export const getIamRadiusAccounting = (id: string) =>
  iamGet<IamRadiusAccountingDetail>(RADIUS_ACCOUNTING_BY_ID(id));
export const createIamRadiusAccounting = (p: IamCreateRadiusAccountingPayload) =>
  iamCreate(RADIUS_ACCOUNTINGS, p);
export const deleteIamRadiusAccounting = (id: string) => iamDelete(RADIUS_ACCOUNTING_BY_ID(id));
