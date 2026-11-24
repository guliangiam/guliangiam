import { requestDocumentJson } from "@/shared/api/http";
import { PLANS, PLAN_BY_ID } from "@/shared/api/paths";
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
// Plans
// ─────────────────────────────────────────────
// Mirrors app/plans.PlanListItem: price/currency are list fields; `isExclusive`
// is detail-only.
export type IamPlanItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  period: string;
  price: number;
  currency?: string;
  state: string;
  createdAt: string;
  updatedAt: string;
};
export type IamPlanDetail = IamPlanItem & {
  description: string;
  isExclusive: boolean;
  paymentProviders: string[];
  /** 计划指向的真实商品 ID（后端 PlanDetail.product）；购买入口用它下单。 */
  product?: string;
  revision: number;
};
export type IamCreatePlanPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  period?: string;
  state?: string;
  isExclusive?: boolean;
  description?: string;
  price?: number;
  currency?: string;
  paymentProviders?: string[];
};
export type IamPatchPlanPayload = {
  displayName?: string;
  description?: string;
  price?: number;
  currency?: string;
  period?: string;
  state?: string;
  isExclusive?: boolean;
  paymentProviders?: string[];
  revision: number;
};

export const getIamPlans = (params?: IamListParams) => iamList<IamPlanItem>(PLANS, params);
export const getIamPlansPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamPlanItem>(PLANS, params);
export const getIamPlan = (id: string) => iamGet<IamPlanDetail>(`${PLAN_BY_ID(id)}`);
export const createIamPlan = (p: IamCreatePlanPayload) => iamCreate(PLANS, p);
export const patchIamPlan = (id: string, p: IamPatchPlanPayload) =>
  iamPatch(`${PLAN_BY_ID(id)}`, p);
export const deleteIamPlan = (id: string) => iamDelete(`${PLAN_BY_ID(id)}`);

// ─────────────────────────────────────────────
// Options loader for PlanSelect
// ─────────────────────────────────────────────
const planOptionSchema = z.object({
  id: z.string(),
  name: z.string(),
  displayName: z.string(),
});

const planListOptionsResponseSchema = z.object({
  items: z.array(planOptionSchema),
  page: pageMetaSchema,
});

export type PlanOption = { id: string; name: string; displayName: string };

export async function getPlanOptions(organizationId?: string): Promise<PlanOption[]> {
  const query = new URLSearchParams({ page: "1", pageSize: "100" });
  if (organizationId) query.set("organizationId", organizationId);
  const response = await requestDocumentJson(`${PLANS}?${query.toString()}`);
  return planListOptionsResponseSchema.parse(response).items.map((i) => ({
    id: i.id,
    name: i.name,
    displayName: i.displayName,
  }));
}
