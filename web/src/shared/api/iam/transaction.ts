import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  iamPostRaw,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import {
  COUPONS_VALIDATE,
  ORDER_CANCEL,
  ORDER_PAY,
  PRODUCT_BUY,
  TRANSACTIONS,
  TRANSACTION_BY_ID,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Transactions
// ─────────────────────────────────────────────
export type IamTransactionItem = {
  id: string;
  organizationId: string;
  userId?: string;
  orderId?: string;
  transactionType: string;
  amount: number;
  currency?: string;
  status: string;
  providerType?: string;
  createdAt: string;
  updatedAt: string;
};
export type IamTransactionDetail = IamTransactionItem & {
  paymentId?: string;
  providerId?: string;
  metadata?: Record<string, unknown>;
  revision: number;
};
export type IamCreateTransactionPayload = {
  organizationId: string;
  transactionType: string;
  amount: number;
  currency?: string;
  status?: string;
  orderId?: string;
  userId?: string;
  paymentId?: string;
  providerType?: string;
  providerId?: string;
  metadata?: Record<string, unknown>;
};
export type IamPatchTransactionPayload = {
  status?: string;
  metadata?: Record<string, unknown>;
  revision: number;
};

export const getIamTransactions = (params?: IamListParams) =>
  iamList<IamTransactionItem>(TRANSACTIONS, params);
export const getIamTransactionsPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamTransactionItem>(TRANSACTIONS, params);
export const getIamTransaction = (id: string) =>
  iamGet<IamTransactionDetail>(`${TRANSACTION_BY_ID(id)}`);
export const createIamTransaction = (p: IamCreateTransactionPayload) => iamCreate(TRANSACTIONS, p);
export const patchIamTransaction = (id: string, p: IamPatchTransactionPayload) =>
  iamPatch(`${TRANSACTION_BY_ID(id)}`, p);
export const deleteIamTransaction = (id: string) => iamDelete(`${TRANSACTION_BY_ID(id)}`);

// Business logic actions
export const cancelExistingOrder = (orderId: string) => iamPostRaw(`${ORDER_CANCEL(orderId)}`);
export const payExistingOrder = (orderId: string, paymentRef?: string) =>
  iamPostRaw(`${ORDER_PAY(orderId)}`, paymentRef ? { paymentRef } : undefined);
export const buyProduct = (productId: string) => iamPostRaw(`${PRODUCT_BUY(productId)}`);
export const validateTransactionCoupon = (payload: {
  code: string;
  organizationId: string;
  userId?: string;
  productNames?: string[];
  orderAmount?: number;
}) => iamCreate(`${COUPONS_VALIDATE}`, payload);
