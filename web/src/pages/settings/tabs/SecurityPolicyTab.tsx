import { useEffect } from "react";
import { Button, Divider, Form, InputNumber, Select, Switch, message } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { queryKeys } from "@/shared/api/query-keys";
import { updateOrganizationSecurityPolicy } from "@/shared/api/iam";
import { isHttpError } from "@/shared/api/http";
import { handleMutationError } from "@/shared/hooks/useMutationError";
import { applyServerFieldErrors, buildServerFieldResolver } from "@/shared/lib/server-error";
import { PASSWORD_DERIVE_ALGORITHMS, type PasswordDeriveAlgorithm } from "@/shared/constants/enums";
import { FormSection } from "@/shared/ui/FormSection";

type PasswordPolicy = {
  minLength?: number;
  requireUppercase?: boolean;
  requireLowercase?: boolean;
  requireDigit?: boolean;
  requireSpecial?: boolean;
};

type SessionPolicy = {
  idleTimeoutMinutes?: number;
};

type SecurityPolicy = {
  passwordPolicy?: PasswordPolicy;
  passwordDeriveAlgorithm?: string;
  sessionPolicy?: SessionPolicy;
};

const PASSWORD_DERIVE_LABELS: Record<PasswordDeriveAlgorithm, string> = {
  hmac_sha256: "HMAC-SHA256",
  hmac_sm3: "HMAC-SM3",
};

export function SecurityPolicyTab({
  orgId,
  settings,
  onSuccess,
}: {
  orgId: string;
  settings: Record<string, unknown>;
  revision: number;
  onSuccess: () => void;
}) {
  const { t } = useTranslation("resource");
  const [form] = Form.useForm();
  const queryClient = useQueryClient();

  const securityPolicy = (settings.securityPolicySummary ?? {}) as SecurityPolicy;
  const passwordPolicy = securityPolicy.passwordPolicy ?? {};
  const sessionPolicy = securityPolicy.sessionPolicy ?? {};

  useEffect(() => {
    form.setFieldsValue({
      passwordMinLength: passwordPolicy.minLength ?? 8,
      passwordRequireUppercase: passwordPolicy.requireUppercase ?? true,
      passwordRequireLowercase: passwordPolicy.requireLowercase ?? true,
      passwordRequireDigit: passwordPolicy.requireDigit ?? true,
      passwordRequireSpecial: passwordPolicy.requireSpecial ?? false,
      passwordDeriveAlgorithm: securityPolicy.passwordDeriveAlgorithm ?? "hmac_sha256",
      sessionIdleTimeoutMinutes: sessionPolicy.idleTimeoutMinutes ?? 480,
    });
  }, [
    form,
    passwordPolicy.minLength,
    passwordPolicy.requireUppercase,
    passwordPolicy.requireLowercase,
    passwordPolicy.requireDigit,
    passwordPolicy.requireSpecial,
    securityPolicy.passwordDeriveAlgorithm,
    sessionPolicy.idleTimeoutMinutes,
  ]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, unknown>) => {
      const {
        passwordMinLength,
        passwordRequireUppercase,
        passwordRequireLowercase,
        passwordRequireDigit,
        passwordRequireSpecial,
        passwordDeriveAlgorithm,
        sessionIdleTimeoutMinutes,
      } = values;
      return updateOrganizationSecurityPolicy(orgId, {
        passwordPolicy: {
          minLength: passwordMinLength,
          requireUppercase: passwordRequireUppercase,
          requireLowercase: passwordRequireLowercase,
          requireDigit: passwordRequireDigit,
          requireSpecial: passwordRequireSpecial,
        },
        passwordDeriveAlgorithm: passwordDeriveAlgorithm as string | undefined,
        sessionPolicy: {
          idleTimeoutMinutes: sessionIdleTimeoutMinutes,
        },
      });
    },
    onSuccess: () => {
      message.success(t("settings.saveSuccess"));
      onSuccess();
      void queryClient.invalidateQueries({ queryKey: queryKeys.organizations.all });
    },
    onError: (error: Error) => {
      if (isHttpError(error)) {
        applyServerFieldErrors(form, error, buildServerFieldResolver(form), t);
      }
      // 422 的 `error.meta.allowed` 由共享 hook 渲染成字段级可操作提示。
      handleMutationError(error, t("settings.saveFailed"));
    },
  });

  return (
    <Form
      form={form}
      layout="vertical"
      scrollToFirstError
      onFinish={async (values) => {
        // Await the mutation so the submit button stays locked until the request
        // settles; the mutation's onError already surfaces the failure message.
        try {
          await mutation.mutateAsync(values);
        } catch {
          /* handled by mutation onError */
        }
      }}
    >
      <FormSection
        title={t("organization.securitySection")}
        description={t("organization.securitySectionDesc")}
      >
        <Form.Item label={t("settings.passwordMinLength")} name="passwordMinLength">
          {/* 后端 numericValue 拒收字符串且范围 6-128（organizations/validation.go）。 */}
          <InputNumber min={6} max={128} precision={0} style={{ width: "100%" }} />
        </Form.Item>
        <Form.Item
          label={t("settings.passwordRequireUppercase")}
          name="passwordRequireUppercase"
          valuePropName="checked"
        >
          <Switch />
        </Form.Item>
        <Form.Item
          label={t("common:settings.passwordRequireLowercase")}
          name="passwordRequireLowercase"
          valuePropName="checked"
        >
          <Switch />
        </Form.Item>
        <Form.Item
          label={t("common:settings.passwordRequireDigit")}
          name="passwordRequireDigit"
          valuePropName="checked"
        >
          <Switch />
        </Form.Item>
        <Form.Item
          label={t("common:settings.passwordRequireSpecial")}
          name="passwordRequireSpecial"
          valuePropName="checked"
        >
          <Switch />
        </Form.Item>
        <Form.Item
          label={t("settings.passwordDeriveAlgorithm")}
          name="passwordDeriveAlgorithm"
          extra={t("settings.passwordDeriveAlgorithmHint")}
        >
          <Select
            options={PASSWORD_DERIVE_ALGORITHMS.map((value) => ({
              value,
              label: PASSWORD_DERIVE_LABELS[value],
            }))}
          />
        </Form.Item>
        <Divider />
        <Form.Item label={t("settings.sessionIdleTimeout")} name="sessionIdleTimeoutMinutes">
          <InputNumber min={1} max={43200} precision={0} style={{ width: "100%" }} />
        </Form.Item>
      </FormSection>
      <Form.Item>
        <Button type="primary" htmlType="submit" loading={mutation.isPending}>
          {t("settings.save")}
        </Button>
      </Form.Item>
    </Form>
  );
}
