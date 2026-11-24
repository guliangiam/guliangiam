import { useCallback, useState } from "react";
import { Alert, App, Button, Form, Input, InputNumber, Select, Space, Switch } from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  MinusCircleOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamRule, validateRuleExpressions } from "@/shared/api/iam";
import type { IamRuleCondition, RuleValidationResult } from "@/shared/api/iam";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { localizedError } from "@/shared/lib/server-error";

type RuleFormValues = {
  organizationId: string;
  name: string;
  scope: string;
  ruleType: string;
  action: string;
  priority?: number;
  conditions?: IamRuleCondition[];
  isEnabled?: boolean;
};

export default function RuleCreatePage() {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<RuleFormValues>();
  const [validationResult, setValidationResult] = useState<RuleValidationResult | null>(null);
  const [validating, setValidating] = useState(false);

  // Mirror the detail page's expression validation: check the condition values
  // against the rule type via the same backend endpoint before submitting.
  const handleValidate = useCallback(async () => {
    setValidating(true);
    setValidationResult(null);
    try {
      const currentRuleType = form.getFieldValue("ruleType") ?? "IP";
      const conditions: IamRuleCondition[] = form.getFieldValue("conditions") ?? [];
      const expressions = conditions
        .map((c) => c.value)
        .filter((value): value is string => Boolean(value));
      const result = await validateRuleExpressions(currentRuleType, expressions);
      setValidationResult(result);
    } catch {
      setValidationResult({ valid: false, errors: ["Validation request failed"] });
    } finally {
      setValidating(false);
    }
  }, [form]);

  const mutation = useMutation({
    mutationFn: async (values: RuleFormValues) => {
      return createIamRule({
        organizationId: values.organizationId,
        name: values.name,
        scope: values.scope,
        ruleType: values.ruleType,
        action: values.action,
        priority: values.priority ?? 0,
        conditions: values.conditions ?? [],
        isEnabled: values.isEnabled ?? true,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["rules"] });
      message.success(tr("rule.created", "Rule created"));
      navigate("/rules");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "rule.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("rule.createTitle", "New Rule")}
      description={tr(
        "rule.createDesc",
        "Create a new access rule, define scope, type, action and matching conditions.",
      )}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/rules")}
      onFinish={async (values: RuleFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tc("validation.organizationIdRequired") }]}
      >
        <GlFormRow label={tr("rule.orgId", "Organization ID")} required>
          <OrganizationSelect ariaLabel={tc("organizationId")} style={{ width: "100%" }} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="name"
        rules={[{ required: true, message: tr("rule.nameRequired", "Please enter rule name") }]}
      >
        <GlFormRow label={tr("rule.name", "Name")} required>
          <Input placeholder={tr("rule.namePlaceholder", "e.g. block-malicious-ip")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="scope"
        rules={[{ required: true, message: tr("rule.selectScope", "Select scope") }]}
      >
        <GlFormRow label={tr("rule.scope", "Scope")} required>
          <Select
            placeholder={tr("rule.selectScope", "Select scope")}
            options={[
              { value: "org", label: tr("rule.scopeOrg", "Organization") },
              { value: "application", label: tr("rule.scopeApp", "Application") },
            ]}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="ruleType"
        rules={[{ required: true, message: tr("rule.selectRuleType", "Select rule type") }]}
      >
        <GlFormRow label={tr("rule.ruleType", "Rule Type")} required>
          <Select
            placeholder={tr("rule.selectRuleType", "Select rule type")}
            options={[
              { value: "IP", label: "IP" },
              { value: "UA", label: "UA" },
              { value: "Rate", label: tr("rule.rateType", "Rate Limit") },
              { value: "WAF", label: "WAF" },
            ]}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="action"
        rules={[{ required: true, message: tr("rule.selectAction", "Select action") }]}
      >
        <GlFormRow label={tr("rule.action", "Action")} required>
          <Select
            placeholder={tr("rule.selectAction", "Select action")}
            options={[
              { value: "Allow", label: tr("rule.actionAllow", "Allow") },
              { value: "Block", label: tr("rule.actionBlock", "Block") },
              { value: "Captcha", label: tr("rule.actionCaptcha", "Captcha") },
            ]}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="priority" initialValue={0}>
        <GlFormRow label={tr("rule.priority", "Priority")}>
          <InputNumber
            min={0}
            style={{ width: "100%" }}
            placeholder={tr("rule.priorityPlaceholder", "Lower number = higher priority")}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item>
        <GlFormRow label={tr("rule.matchConditions", "Match Conditions")}>
          <Form.List name="conditions">
            {(fields, { add, remove }) => (
              <>
                {fields.map(({ key, name, ...restField }) => (
                  <Space key={key} style={{ display: "flex", marginBottom: 8 }} align="baseline">
                    <Form.Item
                      {...restField}
                      name={[name, "field"]}
                      rules={[{ required: true, message: tr("rule.validation.fieldRequired") }]}
                    >
                      <Input placeholder={tr("rule.field", "Field")} />
                    </Form.Item>
                    <Form.Item
                      {...restField}
                      name={[name, "operator"]}
                      rules={[
                        {
                          required: true,
                          message: tr("rule.validation.operatorRequired"),
                        },
                      ]}
                    >
                      <Input placeholder={tr("rule.operator", "Operator")} style={{ width: 100 }} />
                    </Form.Item>
                    <Form.Item
                      {...restField}
                      name={[name, "value"]}
                      rules={[{ required: true, message: tr("rule.validation.valueRequired") }]}
                    >
                      <Input placeholder={tr("rule.value", "Value")} />
                    </Form.Item>
                    <MinusCircleOutlined onClick={() => remove(name)} />
                  </Space>
                ))}
                <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                  {tr("rule.addCondition", "Add Condition")}
                </Button>
              </>
            )}
          </Form.List>
        </GlFormRow>
      </Form.Item>
      <Form.Item>
        <GlFormRow label={tc("validateExpressions")}>
          <Space orientation="vertical" style={{ width: "100%" }}>
            <Button
              icon={
                validationResult?.valid ? (
                  <CheckCircleOutlined />
                ) : validationResult ? (
                  <CloseCircleOutlined />
                ) : undefined
              }
              loading={validating}
              onClick={handleValidate}
            >
              {tc("validateNow")}
            </Button>
            {validationResult && (
              <Alert
                type={validationResult.valid ? "success" : "error"}
                title={validationResult.valid ? tc("validExpressions") : tc("invalidExpressions")}
                description={
                  validationResult.errors?.length ? (
                    <ul style={{ margin: 0, paddingLeft: 16 }}>
                      {validationResult.errors.map((e, i) => (
                        <li key={i}>{e}</li>
                      ))}
                    </ul>
                  ) : undefined
                }
                showIcon
              />
            )}
          </Space>
        </GlFormRow>
      </Form.Item>
      <Form.Item name="isEnabled" valuePropName="checked" initialValue={true}>
        <GlFormRow label={tr("rule.enabled", "Enabled")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
