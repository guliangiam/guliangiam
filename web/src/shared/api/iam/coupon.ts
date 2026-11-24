import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  iamPost,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { COUPONS, COUPON_BY_ID, COUPONS_VALIDATE } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Coupons
// ─────────────────────────────────────────────
export type IamCouponItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  code: string;
  discountType: string;
  discount: number;
  state: string;
  createdAt: string;
  updatedAt: string;
};
export type IamCouponDetail = IamCouponItem & {
  description: string;
  maxDiscount: number;
  scope: string;
  products: string[];
  users: string[];
  quantity: number;
  usedCount: number;
  maxUsagePerUser: number;
  startTime: string;
  expireTime: string;
  minOrderAmount: number;
  currency: string;
  revision: number;
};
export type IamCreateCouponPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  code: string;
  discountType?: string;
  discount?: number;
  maxDiscount?: number;
  scope?: string;
  products?: string[];
  users?: string[];
  quantity?: number;
  maxUsagePerUser?: number;
  startTime?: string;
  expireTime?: string;
  minOrderAmount?: number;
  currency?: string;
  state?: string;
};
export type IamPatchCouponPayload = {
  displayName?: string;
  description?: string;
  scope?: string;
  startTime?: string;
  expireTime?: string;
  currency?: string;
  state?: string;
  discount?: number;
  maxDiscount?: number;
  minOrderAmount?: number;
  products?: string[];
  users?: string[];
  quantity?: number;
  maxUsagePerUser?: number;
  revision: number;
};

export const getIamCoupons = (params?: IamListParams) => iamList<IamCouponItem>(COUPONS, params);
export const getIamCouponsPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamCouponItem>(COUPONS, params);
export const getIamCoupon = (id: string) => iamGet<IamCouponDetail>(`${COUPON_BY_ID(id)}`);
export const createIamCoupon = (p: IamCreateCouponPayload) => iamCreate(COUPONS, p);
export const patchIamCoupon = (id: string, p: IamPatchCouponPayload) =>
  iamPatch(`${COUPON_BY_ID(id)}`, p);
export const deleteIamCoupon = (id: string) => iamDelete(`${COUPON_BY_ID(id)}`);

// ─────────────────────────────────────────────
// Coupon Validation
// ─────────────────────────────────────────────
export type ValidateCouponResult = {
  isValid: boolean;
  discount: number;
  discountType: string;
  maxDiscount: number;
  message: string;
};

export const validateCoupon = (params: {
  organizationId: string;
  code: string;
  user?: string;
  products?: string[];
  orderAmount?: number;
}) => iamPost<ValidateCouponResult>(`${COUPONS_VALIDATE}`, params);
// ─────────────────────────────────────────────
// Coupon Usage Tracking
// ─────────────────────────────────────────────
//
// Removed: `getCouponUsages` / `IamCouponUsageItem` targeted
// `GET /api/v1/coupons/{id}/usages`, which no handler serves — every call would
// have 404ed. There were no consumers. Tracking coupon redemptions needs a
// backend capability first; registered as 开放项注册表 OPEN-FE-01.
