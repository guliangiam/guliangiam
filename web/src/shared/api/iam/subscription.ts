import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { SUBSCRIPTIONS, SUBSCRIPTION_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Subscriptions
// ─────────────────────────────────────────────
// Mirrors app/subscriptions.SubscriptionListItem: user/plan/startAt/endAt are
// list fields; pricing/expireAt are detail-only.
export type IamSubscriptionItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  user?: string;
  plan?: string;
  state: string;
  startAt?: string | null;
  endAt?: string | null;
  createdAt: string;
  updatedAt: string;
};
export type IamSubscriptionDetail = IamSubscriptionItem & {
  user: string;
  plan: string;
  pricing: string;
  application: string;
  startAt: string | null;
  endAt: string | null;
  expireAt: string | null;
  period: string;
  description: string;
  revision: number;
};
export type IamCreateSubscriptionPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  user?: string;
  plan?: string;
  pricing?: string;
  state?: string;
};
export type IamPatchSubscriptionPayload = {
  displayName?: string;
  state?: string;
  startAt?: string | null;
  endAt?: string | null;
  expireAt?: string | null;
  revision: number;
};

export const getIamSubscriptions = (params?: IamListParams) =>
  iamList<IamSubscriptionItem>(SUBSCRIPTIONS, params);
export const getIamSubscriptionsPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamSubscriptionItem>(SUBSCRIPTIONS, params);
export const getIamSubscription = (id: string) =>
  iamGet<IamSubscriptionDetail>(SUBSCRIPTION_BY_ID(id));
export const createIamSubscription = (p: IamCreateSubscriptionPayload) =>
  iamCreate(SUBSCRIPTIONS, p);
export const patchIamSubscription = (id: string, p: IamPatchSubscriptionPayload) =>
  iamPatch(SUBSCRIPTION_BY_ID(id), p);
export const deleteIamSubscription = (id: string) => iamDelete(SUBSCRIPTION_BY_ID(id));
