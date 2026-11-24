import { createDetailLoader, createListLoader } from "@/shared/api/loaders/factory";
import { requestDocumentJson } from "@/shared/api/http";

import {
  KEYS,
  KEY_BY_ID,
  PLANS,
  PLAN_BY_ID,
  PRICINGS,
  PRICING_BY_ID,
  PRODUCTS,
  PRODUCT_BY_ID,
} from "@/shared/api/paths";

export const plansListLoader = createListLoader({
  fetch: async (_ctx, p) =>
    requestDocumentJson(
      `${PLANS}?keyword=${p.keyword ?? ""}&page=${p.page ?? 1}&pageSize=${p.pageSize ?? 10}`,
    ),
});
export const planDetailLoader = createDetailLoader({
  fetch: async (_ctx, id) => requestDocumentJson(`${PLAN_BY_ID(id)}`),
});
export const pricingsListLoader = createListLoader({
  fetch: async (_ctx, p) =>
    requestDocumentJson(
      `${PRICINGS}?keyword=${p.keyword ?? ""}&page=${p.page ?? 1}&pageSize=${p.pageSize ?? 10}`,
    ),
});
export const pricingDetailLoader = createDetailLoader({
  fetch: async (_ctx, id) => requestDocumentJson(`${PRICING_BY_ID(id)}`),
});
export const productsListLoader = createListLoader({
  fetch: async (_ctx, p) =>
    requestDocumentJson(
      `${PRODUCTS}?keyword=${p.keyword ?? ""}&page=${p.page ?? 1}&pageSize=${p.pageSize ?? 10}`,
    ),
});
export const productDetailLoader = createDetailLoader({
  fetch: async (_ctx, id) => requestDocumentJson(`${PRODUCT_BY_ID(id)}`),
});
export const keysListLoader = createListLoader({
  fetch: async (_ctx, p) =>
    requestDocumentJson(
      `${KEYS}?keyword=${p.keyword ?? ""}&page=${p.page ?? 1}&pageSize=${p.pageSize ?? 10}`,
    ),
});
export const keyDetailLoader = createDetailLoader({
  fetch: async (_ctx, id) => requestDocumentJson(`${KEY_BY_ID(id)}`),
});
