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
  PAYMENTS,
  PAYMENT_BY_ID,
  PAYMENT_RESULT,
  PAYMENT_INVOICE,
  USER_PAYMENTS,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Payments
// ─────────────────────────────────────────────
// Mirrors app/payments.PaymentListItem: provider/user/price/currency are on
// the list DTO as well as the detail DTO.
export type IamPaymentItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  provider?: string;
  type: string;
  user?: string;
  price: number;
  currency?: string;
  state: string;
  createdAt: string;
  updatedAt: string;
};
export type IamPaymentDetail = IamPaymentItem & {
  user: string;
  provider: string;
  providerName: string;
  product: string;
  productName: string;
  productDisplayName: string;
  detail: string;
  price: number;
  currency: string;
  personName: string;
  personIdCard: string;
  personEmail: string;
  personPhone: string;
  invoiceType: string;
  invoiceTitle: string;
  invoiceTaxId: string;
  invoiceRemark: string;
  invoiceUrl: string;
  payUrl: string;
  successUrl: string;
  outOrderId: string;
  order: string;
  message: string;
  revision: number;
};
export type IamCreatePaymentPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  user?: string;
  provider?: string;
  type?: string;
  products?: string[];
  price?: number;
  currency?: string;
};
export type IamPatchPaymentPayload = {
  displayName?: string;
  state?: string;
  revision: number;
};

export const getIamPayments = (params?: IamListParams) => iamList<IamPaymentItem>(PAYMENTS, params);
export const getIamPaymentsPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamPaymentItem>(PAYMENTS, params);
export const getIamPayment = (id: string) => iamGet<IamPaymentDetail>(`${PAYMENT_BY_ID(id)}`);
export const createIamPayment = (p: IamCreatePaymentPayload) => iamCreate(PAYMENTS, p);
export const patchIamPayment = (id: string, p: IamPatchPaymentPayload) =>
  iamPatch(`${PAYMENT_BY_ID(id)}`, p);
export const deleteIamPayment = (id: string) => iamDelete(`${PAYMENT_BY_ID(id)}`);

// ─────────────────────────────────────────────
// Payment Result (user-facing)
// ─────────────────────────────────────────────
export type IamPaymentResult = {
  id: string;
  state: string;
  price: number;
  currency: string;
  productName: string;
  providerName: string;
  payUrl: string;
  createdAt: string;
  updatedAt: string;
};

export const getIamPaymentResult = (id: string) =>
  iamGet<IamPaymentResult>(`${PAYMENT_RESULT(id)}`);

// ─────────────────────────────────────────────
// Payment Invoice
// ─────────────────────────────────────────────
export type InvoicePaymentPayload = {
  invoiceType?: string;
  invoiceTitle?: string;
  invoiceTaxId?: string;
  invoiceRemark?: string;
  personName?: string;
  personIdCard?: string;
  personEmail?: string;
  personPhone?: string;
  revision: number;
};

export const invoicePayment = (id: string, payload: InvoicePaymentPayload) =>
  iamPostRaw(`${PAYMENT_INVOICE(id)}`, payload);

// ─────────────────────────────────────────────
// Payment Notify URL
// ─────────────────────────────────────────────

/** Returns the payment notify URL for the current host. */
export function getPaymentNotifyUrl(): string {
  if (typeof window === "undefined") {
    return "";
  }
  return `${window.location.origin}/api/v1/payments/notify`;
}

/** List payments for a specific user by user ID (user-scoped endpoint). */
export type UserPaymentsResult = {
  items: IamPaymentItem[];
  page: { page: number; pageSize: number; total: number };
};

export const getUserPaymentsByUserId = (
  userId: string,
  params?: { organizationId?: string; page?: number; pageSize?: number },
) => {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 20),
  });
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  return iamGet<UserPaymentsResult>(`${USER_PAYMENTS(userId)}?${query.toString()}`);
};
