import {
  iamList,
  iamListWithPagination,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  iamPostRaw,
  type IamListParams,
} from "./client";
import { RULES, RULE_BY_ID, RULE_VALIDATE } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Rules (WAF)
// ─────────────────────────────────────────────
// Conditions are the backend's only storage for type-specific rule data:
// IP rules use `{field:"ip", operator:"eq"|"cidr"|"range", value}` and WAF rules
// use `{pattern, target?}` (see middleware/ip_rules.go and middleware/waf.go).
// The detail page derives its typed inputs from these entries and writes them
// back, so no field is shown that the backend cannot persist.
export type IamRuleCondition = {
  field?: string;
  operator?: string;
  value?: string;
  pattern?: string;
  target?: string;
  [key: string]: unknown;
};
export type IamRuleItem = {
  id: string;
  organizationId: string;
  name: string;
  scope: string;
  ruleType: string;
  action: string;
  priority: number;
  isEnabled: boolean;
  createdAt: string;
  updatedAt: string;
  revision: number;
};
export type IamRuleDetail = IamRuleItem & {
  conditions: IamRuleCondition[];
};
export type IamCreateRulePayload = {
  organizationId: string;
  name: string;
  scope: string;
  ruleType: string;
  action: string;
  priority?: number;
  conditions?: IamRuleCondition[];
  isEnabled?: boolean;
};
export type IamPatchRulePayload = {
  name?: string;
  scope?: string;
  ruleType?: string;
  action?: string;
  priority?: number;
  conditions?: IamRuleCondition[];
  isEnabled?: boolean;
  revision: number;
};

export const getIamRules = (params?: IamListParams) => iamList<IamRuleItem>(RULES, params);
export const getIamRulesPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamRuleItem>(RULES, params);
export const getIamRule = (id: string) => iamGet<IamRuleDetail>(RULE_BY_ID(id));
export const createIamRule = (p: IamCreateRulePayload) => iamCreate(RULES, p);
export const patchIamRule = (id: string, p: IamPatchRulePayload) => iamPatch(RULE_BY_ID(id), p);
export const deleteIamRule = (id: string) => iamDelete(RULE_BY_ID(id));

// ─────────────────────────────────────────────
// Rule Expression Validation
// ─────────────────────────────────────────────
export type RuleValidationResult = {
  valid: boolean;
  errors?: string[];
};

export const validateRuleExpressions = (
  ruleType: string,
  conditions: string[],
): Promise<RuleValidationResult> =>
  iamPostRaw(RULE_VALIDATE, {
    ruleType,
    conditions,
  }) as unknown as Promise<RuleValidationResult>;
