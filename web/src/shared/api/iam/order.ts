import {
  iamList,
  iamGet,
  iamGetIfChanged,
  iamCreate,
  iamPatch,
  iamDelete,
  iamPost,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import {
  ORDERS,
  ORDERS_USER_ORDERS,
  ORDER_BY_ID,
  ORDER_CANCEL,
  ORDER_NOTIFY_PAID,
  ORDER_PAY,
  ORDER_PAY_URL,
  ORDER_PLACE,
  USER_ORDERS,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Orders
// ─────────────────────────────────────────────

/**
 * Product snapshot embedded in an order.
 *
 * Mirrors `domain.ProductInfo`; the fields are recorded at placement time so an
 * order stays readable even after the product is renamed or repriced.
 */
export type IamOrderProductInfo = {
  owner: string;
  name: string;
  displayName: string;
  image?: string;
  detail?: string;
  price: number;
  currency?: string;
  isRecharge: boolean;
  quantity: number;
  pricingName?: string;
  planName?: string;
};

export type IamOrderItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  user?: string;
  price: number;
  currency?: string;
  /** Order state machine value (lowercase: `created`, `payment_pending`, …). */
  state: string;
  paymentUrl?: string;
  /** Legacy alias for paymentUrl — kept for backwards compatibility with page consumers. */
  payUrl?: string;
  type?: string;
  productName?: string;
  providerName?: string;
  provider?: string;
  tradeNo?: string;
  createdAt: string;
  updatedAt: string;
};

export type IamOrderDetail = IamOrderItem & {
  /** Legacy alias for single-product orders where productId was sufficient. */
  product?: string;
  products?: string[];
  productInfos?: IamOrderProductInfo[];
  payment?: string;
  message?: string;
  couponName?: string;
  couponDiscount: number;
  revision: number;
  /**
   * Settlement instant recorded when the state machine entered `paid`. Absent
   * means the order never reached `paid`, which is what makes a "paid"
   * conclusion replayable after the fact (design report §42.3.3).
   */
  paidAt?: string;
  /** Server decision that the order can no longer change state (§42.3.11). */
  terminal: boolean;
  /** Server decision that the buyer is still expected to pay (§42.3.11). */
  awaitingPayment: boolean;
  /**
   * Server-suggested delay before the next read, in seconds (§42.3.11).
   *
   * Polling pages must use this instead of a locally chosen interval: the
   * transition graph lives in the backend state machine, so the "how often"
   * answer belongs there too.
   */
  pollAfterSeconds: number;
};

export type IamCreateOrderPayload = {
  organizationId?: string;
  name?: string;
  displayName?: string;
  user?: string;
  products?: string[];
  payment?: string;
  price?: number;
  currency?: string;
  couponName?: string;
  couponDiscount?: number;
  notifyUrl?: string;
  returnUrl?: string;
};

export type IamPatchOrderPayload = {
  displayName?: string;
  payment?: string;
  message?: string;
  price?: number;
  currency?: string;
  couponName?: string;
  couponDiscount?: number;
  state?: string;
  revision: number;
};

export const getIamOrders = (params?: IamListParams) => iamList<IamOrderItem>(ORDERS, params);
export const getIamOrdersPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamOrderItem>(ORDERS, params);
export const getIamOrder = (id: string) => iamGet<IamOrderDetail>(ORDER_BY_ID(id));

/**
 * Conditional read of one order for polling (§42.3.11).
 *
 * Re-reading the order resource — rather than a separate payment-status endpoint
 * — is what keeps payment state on a single face: there is one representation of
 * an order and one place its state comes from. Passing the previous `etag` back
 * turns an unchanged poll into an empty 304.
 */
export const getIamOrderIfChanged = (id: string, etag?: string) =>
  iamGetIfChanged<IamOrderDetail>(ORDER_BY_ID(id), etag);

export const createIamOrder = (p: IamCreateOrderPayload) => iamCreate(ORDERS, p);
export const patchIamOrder = (id: string, p: IamPatchOrderPayload) => iamPatch(ORDER_BY_ID(id), p);
export const deleteIamOrder = (id: string) => iamDelete(ORDER_BY_ID(id));

// ─────────────────────────────────────────────
// Order Pay (user-facing payment flow)
// ─────────────────────────────────────────────
export const getIamOrderPayUrl = (id: string) =>
  iamGet<{ payUrl: string; state: string }>(ORDER_PAY_URL(id));

export const notifyIamOrderPaid = (id: string, payload: { paymentId: string }) =>
  iamCreate(ORDER_NOTIFY_PAID(id), payload);

// ─────────────────────────────────────────────
// User-facing Order Actions
// ─────────────────────────────────────────────

/**
 * Place an order.
 *
 * The endpoint returns the created order resource itself, so the caller gets the
 * payment URL and the polling guidance from the same representation it will poll
 * — no second shape to keep in sync.
 */
export type PlaceOrderResult = IamOrderDetail;

/**
 * Place-order payload as the caller expresses it (catalog ids), mapped onto the
 * backend `POST /orders/place` contract (`organizationId` + `products`).
 * 后端使用 DisallowUnknownFields，productId/pricingId 这类目录 id 不能直接透传。
 */
export type PlaceOrderPayload = {
  organizationId?: string;
  productId?: string;
  pricingId?: string;
  products?: string[];
  /** Payment provider business name (`Provider.Name`), stored on the order. */
  payment?: string;
  /** Optional claimed total; the server validates it against the catalog. */
  price?: number;
  currency?: string;
};

export const placeOrder = (payload: PlaceOrderPayload) => {
  const products =
    payload.products?.filter(Boolean) ??
    [payload.productId, payload.pricingId].filter((id): id is string => Boolean(id));
  return iamCreate(ORDER_PLACE, {
    organizationId: payload.organizationId,
    products,
    payment: payload.payment,
    price: payload.price,
    currency: payload.currency,
  }) as Promise<IamOrderDetail>;
};

// ─────────────────────────────────────────────
// Order Management Actions
// ─────────────────────────────────────────────
export const cancelOrder = (id: string, revision: number) =>
  iamPost(`${ORDER_CANCEL(id)}?revision=${revision}`, undefined);

export const payOrder = (id: string, paymentRef: string) => iamPost(ORDER_PAY(id), { paymentRef });

export type UserOrdersResult = {
  items: IamOrderItem[];
  page: { page: number; pageSize: number; total: number };
};

export const getUserOrders = (params: {
  organizationId: string;
  user: string;
  page?: number;
  pageSize?: number;
}) => {
  const query = new URLSearchParams({
    organizationId: params.organizationId,
    user: params.user,
    page: String(params.page ?? 1),
    pageSize: String(params.pageSize ?? 50),
  });
  return iamGet<UserOrdersResult>(`${ORDERS_USER_ORDERS}?${query.toString()}`);
};

/** List orders for a specific user by user ID (user-scoped endpoint). */
export const getUserOrdersByUserId = (
  userId: string,
  params?: { organizationId?: string; page?: number; pageSize?: number },
) => {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 20),
  });
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  return iamGet<UserOrdersResult>(`${USER_ORDERS(userId)}?${query.toString()}`);
};

// ─────────────────────────────────────────────
// Order options (lightweight list for Select components)
// ─────────────────────────────────────────────

/** Lightweight order option — id + displayName for OrderSelect. */
export type OrderOption = { id: string; displayName: string; name?: string };

/**
 * Fetch order options for OrderSelect. Returns up to 50 orders, suitable
 * for a relational-picker dropdown (交互简化 §6).
 */
export async function getOrderOptions(organizationId?: string): Promise<OrderOption[]> {
  const items = await getIamOrders({ page: 1, pageSize: 50, organizationId });
  return items.map((o) => ({
    id: o.id,
    displayName: o.displayName || o.name || o.id,
    name: o.name,
  }));
}
