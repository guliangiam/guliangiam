import { requestDocumentJson } from "@/shared/api/http";
import { PRODUCTS, PRODUCT_BY_ID, PRODUCT_BUY } from "@/shared/api/paths";
import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { z } from "zod";
import { pageMetaSchema } from "./schemas";

// ─────────────────────────────────────────────
// Products
// ─────────────────────────────────────────────
// Mirrors app/products.ProductListItem: `isRecharge` is only on the detail
// DTO, so it must not be declared here (a list column reading it rendered
// undefined). `image` is present on both.
export type IamProductItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  image?: string;
  currency: string;
  price: number;
  state: string;
  createdAt: string;
  updatedAt: string;
};
export type IamProductDetail = IamProductItem & {
  image: string;
  detail: string;
  description: string;
  tag: string;
  quantity: number;
  sold: number;
  isRecharge: boolean;
  rechargeOptions: number[];
  disableCustomRecharge: boolean;
  providers: string[];
  successUrl: string;
  revision: number;
};
export type IamCreateProductPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  currency?: string;
  price?: number;
  state?: string;
  image?: string;
  detail?: string;
  description?: string;
  tag?: string;
  quantity?: number;
  isRecharge?: boolean;
  rechargeOptions?: number[];
  disableCustomRecharge?: boolean;
  providers?: string[];
  successUrl?: string;
};
export type IamPatchProductPayload = {
  displayName?: string;
  image?: string;
  detail?: string;
  description?: string;
  tag?: string;
  currency?: string;
  price?: number;
  quantity?: number;
  isRecharge?: boolean;
  rechargeOptions?: number[];
  disableCustomRecharge?: boolean;
  providers?: string[];
  successUrl?: string;
  state?: string;
  revision: number;
};

export const getIamProducts = (params?: IamListParams) => iamList<IamProductItem>(PRODUCTS, params);
export const getIamProductsPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamProductItem>(PRODUCTS, params);
export const getIamProduct = (id: string) => iamGet<IamProductDetail>(`${PRODUCT_BY_ID(id)}`);
export const createIamProduct = (p: IamCreateProductPayload) => iamCreate(PRODUCTS, p);
export const patchIamProduct = (id: string, p: IamPatchProductPayload) =>
  iamPatch(`${PRODUCT_BY_ID(id)}`, p);
export const deleteIamProduct = (id: string) => iamDelete(`${PRODUCT_BY_ID(id)}`);

// ─────────────────────────────────────────────
// Product Store (public-facing)
// ─────────────────────────────────────────────
export type IamProductStoreItem = {
  id: string;
  name: string;
  displayName: string;
  image: string;
  detail: string;
  description: string;
  tag: string;
  currency: string;
  price: number;
  quantity: number;
  sold: number;
  state: string;
  isRecharge: boolean;
};

export const getIamProductStore = (params?: IamListParams) =>
  iamList<IamProductStoreItem>(PRODUCTS, { ...params, status: "active" });

/** Mirrors the Go `BuyProduct` request body (`user/pricingName/planName/quantity`). */
export type IamBuyProductPayload = {
  user?: string;
  pricingName?: string;
  planName?: string;
  quantity?: number;
};

export const buyIamProduct = (productId: string, payload: IamBuyProductPayload) =>
  iamCreate(`${PRODUCT_BUY(productId)}`, payload);

// ─────────────────────────────────────────────
// Options loader for ProductSelect
// ─────────────────────────────────────────────
const productOptionSchema = z.object({
  id: z.string(),
  name: z.string(),
  displayName: z.string(),
});

const productListOptionsResponseSchema = z.object({
  items: z.array(productOptionSchema),
  page: pageMetaSchema,
});

export type ProductOption = { id: string; name: string; displayName: string };

export async function getProductOptions(organizationId?: string): Promise<ProductOption[]> {
  const query = new URLSearchParams({ page: "1", pageSize: "100" });
  if (organizationId) query.set("organizationId", organizationId);
  const response = await requestDocumentJson(`${PRODUCTS}?${query.toString()}`);
  return productListOptionsResponseSchema.parse(response).items.map((i) => ({
    id: i.id,
    name: i.name,
    displayName: i.displayName,
  }));
}
