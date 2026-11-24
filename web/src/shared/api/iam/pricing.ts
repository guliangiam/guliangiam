import { requestDocumentJson } from "@/shared/api/http";
import { PRICINGS, PRICING_BY_ID } from "@/shared/api/paths";
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
// Pricings
// ─────────────────────────────────────────────
// Mirrors app/pricings.PricingListItem: isEnabled/trialDuration are list fields;
// `application` is detail-only.
export type IamPricingItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  isEnabled: boolean;
  trialDuration: number;
  createdAt: string;
  updatedAt: string;
};
export type IamPricingDetail = IamPricingItem & {
  description: string;
  plans: string[];
  application: string;
  revision: number;
};
export type IamCreatePricingPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  application: string;
  description?: string;
  plans?: string[];
  trialDuration?: number;
};
export type IamPatchPricingPayload = {
  displayName?: string;
  description?: string;
  plans?: string[];
  trialDuration?: number;
  application?: string;
  revision: number;
};

export const getIamPricings = (params?: IamListParams) => iamList<IamPricingItem>(PRICINGS, params);
export const getIamPricingsPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamPricingItem>(PRICINGS, params);
export const getIamPricing = (id: string) => iamGet<IamPricingDetail>(`${PRICING_BY_ID(id)}`);
export const createIamPricing = (p: IamCreatePricingPayload) => iamCreate(PRICINGS, p);
export const patchIamPricing = (id: string, p: IamPatchPricingPayload) =>
  iamPatch(`${PRICING_BY_ID(id)}`, p);
export const deleteIamPricing = (id: string) => iamDelete(`${PRICING_BY_ID(id)}`);

// ─────────────────────────────────────────────
// Options loader for PricingSelect
// ─────────────────────────────────────────────
const pricingOptionSchema = z.object({
  id: z.string(),
  name: z.string(),
  displayName: z.string(),
});

const pricingListOptionsResponseSchema = z.object({
  items: z.array(pricingOptionSchema),
  page: pageMetaSchema,
});

export type PricingOption = { id: string; name: string; displayName: string };

export async function getPricingOptions(organizationId?: string): Promise<PricingOption[]> {
  const query = new URLSearchParams({ page: "1", pageSize: "100" });
  if (organizationId) query.set("organizationId", organizationId);
  const response = await requestDocumentJson(`${PRICINGS}?${query.toString()}`);
  return pricingListOptionsResponseSchema.parse(response).items.map((i) => ({
    id: i.id,
    name: i.name,
    displayName: i.displayName,
  }));
}
