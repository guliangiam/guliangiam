import { useCallback, useEffect, useState } from "react";
import { App, Form } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  deleteIamRule,
  getIamRule,
  patchIamRule,
  validateRuleExpressions,
  type IamRuleCondition,
} from "@/shared/api/iam";
import type { RuleValidationResult } from "@/shared/api/iam/rule";
import { queryKeys } from "@/shared/api/query-keys";
import {
  deriveIPRange,
  deriveWAFPattern,
  editableConditions,
  mergeRuleConditions,
} from "./ruleConditions";
import { localizedError } from "@/shared/lib/server-error";

export const SCOPE_OPTIONS = [
  { label: "Organization", value: "org" },
  { label: "Application", value: "application" },
];

export const RULE_TYPE_OPTIONS = [
  { label: "IP", value: "IP" },
  { label: "UA", value: "UA" },
  { label: "Rate", value: "Rate" },
  { label: "WAF", value: "WAF" },
];

export const ACTION_OPTIONS = [
  { label: "Allow", value: "Allow" },
  { label: "Block", value: "Block" },
  { label: "Captcha", value: "Captcha" },
];

export const SEVERITY_OPTIONS = [
  { label: "Low", value: "Low" },
  { label: "Medium", value: "Medium" },
  { label: "High", value: "High" },
  { label: "Critical", value: "Critical" },
];

export const ruleTypeColorMap: Record<string, string> = {
  IP: "volcano",
  UA: "orange",
  Rate: "gold",
  WAF: "red",
};

export const actionColorMap: Record<string, string> = {
  Allow: "green",
  Block: "red",
  Captcha: "blue",
};

export type RuleFormValues = {
  name: string;
  scope: string;
  ruleType: string;
  action: string;
  priority: number;
  ipRange: string;
  wafPattern: string;
  conditions: IamRuleCondition[];
  isEnabled: boolean;
};

export function useRuleDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<RuleFormValues>();
  const [validationResult, setValidationResult] = useState<RuleValidationResult | null>(null);
  const [validating, setValidating] = useState(false);

  const detailQuery = useQuery({
    queryKey: queryKeys.rules.detail(id!),
    queryFn: () => getIamRule(id!),
    enabled: !!id,
  });

  const ruleType = Form.useWatch("ruleType", form) ?? "IP";

  useEffect(() => {
    if (detailQuery.data) {
      const conditions = detailQuery.data.conditions ?? [];
      form.setFieldsValue({
        name: detailQuery.data.name,
        scope: detailQuery.data.scope,
        ruleType: detailQuery.data.ruleType,
        action: detailQuery.data.action,
        priority: detailQuery.data.priority,
        ipRange: deriveIPRange(conditions),
        wafPattern: deriveWAFPattern(conditions),
        conditions: editableConditions(conditions),
        isEnabled: detailQuery.data.isEnabled,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: RuleFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      // Read the typed inputs from the store, not `values`: when the rule type
      // hides one of them the Form.Item is unmounted, but its value still has
      // to survive the save (it is the storage form of those conditions).
      const ipRange = form.getFieldValue("ipRange") as string | undefined;
      const wafPattern = form.getFieldValue("wafPattern") as string | undefined;
      return patchIamRule(detailQuery.data.id, {
        name: values.name,
        scope: values.scope,
        ruleType: values.ruleType,
        action: values.action,
        priority: values.priority,
        conditions: mergeRuleConditions(
          values.conditions,
          ipRange,
          wafPattern,
          detailQuery.data.conditions ?? [],
        ),
        isEnabled: values.isEnabled,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["rules"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamRule(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["rules"] });
      navigate("/rules");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const handleValidate = useCallback(async () => {
    setValidating(true);
    setValidationResult(null);
    try {
      const currentRuleType = form.getFieldValue("ruleType") ?? "IP";
      const conditions: IamRuleCondition[] = form.getFieldValue("conditions") ?? [];
      const expressions = conditions
        .map((c) => c.value)
        .filter((value): value is string => Boolean(value));
      const typeExprFieldName =
        currentRuleType === "IP" ? "ipRange" : currentRuleType === "WAF" ? "wafPattern" : undefined;
      const typeExpr = typeExprFieldName
        ? (form.getFieldValue(typeExprFieldName) as string | undefined)
        : undefined;
      if (typeExpr) expressions.unshift(typeExpr);
      const result = await validateRuleExpressions(currentRuleType, expressions);
      setValidationResult(result);
    } catch {
      setValidationResult({ valid: false, errors: ["Validation request failed"] });
    } finally {
      setValidating(false);
    }
  }, [form]);

  return {
    id,
    tc,
    tr,
    navigate,
    form,
    ruleType,
    validationResult,
    validating,
    detailQuery,
    updateMutation,
    deleteMutation,
    handleValidate,
  };
}
