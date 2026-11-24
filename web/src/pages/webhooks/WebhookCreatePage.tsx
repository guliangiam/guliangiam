import { useState } from "react";
import {
  App,
  Button,
  Checkbox,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Switch,
  Typography,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamWebhook } from "@/shared/api/iam";
import { getCertOptions } from "@/shared/api/iam/cert";
import { getOrganizationOptions } from "@/shared/api/iam/organization";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { JsonEditor } from "@/shared/ui/JsonEditor";
import { jsonFieldRule } from "@/shared/lib/json-field-rules";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { localizedError } from "@/shared/lib/server-error";

const METHOD_VALUES = ["POST", "GET", "PUT", "DELETE"] as const;

const CONTENT_TYPE_VALUES = ["application/json", "application/x-www-form-urlencoded"] as const;

const WEBHOOK_EVENTS = [
  "user.created",
  "user.updated",
  "user.deleted",
  "organization.created",
  "organization.updated",
  "organization.deleted",
  "application.created",
  "application.updated",
  "application.deleted",
  "provider.created",
  "provider.updated",
  "provider.deleted",
  "role.created",
  "role.updated",
  "role.deleted",
  "permission.created",
  "permission.updated",
  "permission.deleted",
  "group.created",
  "group.updated",
  "group.deleted",
  "session.created",
  "session.revoked",
  "token.issued",
  "token.revoked",
  "login.success",
  "login.failure",
  "password.changed",
  "password.reset",
  "mfa.enabled",
  "mfa.disabled",
];

const WEBHOOK_OBJECT_FIELDS = [
  "All",
  "owner",
  "name",
  "createdTime",
  "displayName",
  "type",
  "isEnabled",
  "description",
  "organization",
  "email",
  "phone",
];

type WebhookFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  organization: string;
  url: string;
  method: string;
  contentType: string;
  headers: string;
  events: string[];
  customBody: string;
  customHeaders: string;
  certId: string;
  isUserExtended: boolean;
  tokenFields: string[];
  singleOrgOnly: boolean;
  objectFields: string[];
  maxRetries: number;
  retryInterval: number;
  useExponentialBackoff: boolean;
  timeoutMs: number;
  isEnabled: boolean;
};

export default function WebhookCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<WebhookFormValues>();
  // BE-B-7: the signing secret plaintext exists only in the create response.
  // Hold it in memory until the operator confirms, then discard it for good.
  const [secretReveal, setSecretReveal] = useState<string | null>(null);

  const orgOptionsQuery = useQuery({
    queryKey: ["org-options"],
    queryFn: () => getOrganizationOptions(),
    staleTime: 60_000,
  });

  const certOptionsQuery = useQuery({
    queryKey: ["cert-options"],
    queryFn: () => getCertOptions(),
    staleTime: 60_000,
  });

  const METHOD_OPTIONS = METHOD_VALUES.map((v) => ({
    label: tr(`webhook.methodOption.${v}`),
    value: v,
  }));
  const CONTENT_TYPE_OPTIONS = CONTENT_TYPE_VALUES.map((v) => ({
    label: tr(`webhook.contentTypeOption.${v}`),
    value: v,
  }));

  const mutation = useMutation({
    mutationFn: async (values: WebhookFormValues) => {
      return createIamWebhook({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        organization: values.organization,
        url: values.url,
        method: values.method,
        contentType: values.contentType,
        headers: values.headers,
        events: values.events,
        customBody: values.customBody,
        customHeaders: values.customHeaders,
        certId: values.certId,
        isUserExtended: values.isUserExtended,
        tokenFields: values.tokenFields,
        singleOrgOnly: values.singleOrgOnly,
        objectFields: values.objectFields,
        maxRetries: values.maxRetries,
        retryInterval: values.retryInterval,
        useExponentialBackoff: values.useExponentialBackoff,
        timeoutMs: values.timeoutMs,
        isEnabled: values.isEnabled,
      });
    },
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.webhooks.all });
      const plain = result?.data?.secretPlain;
      if (plain) {
        // Show the one-time secret before leaving; navigation happens on confirm.
        setSecretReveal(plain);
        return;
      }
      message.success(tc("createSuccess"));
      navigate("/webhooks");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const handleCopySecret = () => {
    if (!secretReveal) return;
    void navigator.clipboard?.writeText(secretReveal).then(
      () => message.success(tc("copiedToClipboard")),
      () => message.error(tc("error")),
    );
  };

  const dismissSecretReveal = () => {
    setSecretReveal(null);
    message.success(tc("createSuccess"));
    navigate("/webhooks");
  };

  return (
    <>
      <ResourceFormPage
        title={tc("webhookCreateTitle")}
        description={tc("webhookCreateDescription")}
        saving={mutation.isPending}
        form={form}
        onCancel={() => navigate("/webhooks")}
        onFinish={async (values: WebhookFormValues) => {
          await mutation.mutateAsync(values);
        }}
      >
        <Form.Item
          name="organizationId"
          rules={[{ required: true, message: tc("validation.organizationIdRequired") }]}
        >
          <GlFormRow label={tc("organizationId")} required>
            <OrganizationSelect ariaLabel={tc("organizationId")} style={{ width: "100%" }} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="name" rules={[{ required: true, message: tc("validation.nameRequired") }]}>
          <GlFormRow label={tc("name")} required>
            <Input placeholder="e.g. webhook-login" />
          </GlFormRow>
        </Form.Item>

        <FormSection title={tc("webhookConfig")}>
          <Form.Item name="displayName" rules={[{ required: true }]}>
            <GlFormRow label={tc("displayName")} required>
              <Input />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="organization">
            <GlFormRow label={tr("webhook.organization")}>
              <Select
                allowClear
                showSearch
                optionFilterProp="label"
                placeholder={tr("webhook.selectOrganization")}
                options={orgOptionsQuery.data ?? []}
                loading={orgOptionsQuery.isLoading}
              />
            </GlFormRow>
          </Form.Item>
          <Form.Item
            name="url"
            rules={[
              { required: true, message: tr("webhook.urlRequired") },
              { type: "url", message: tc("form.invalidUrl") },
            ]}
          >
            <GlFormRow label={tr("webhook.url")} required>
              <Input placeholder="https://example.com/webhook" />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="method" initialValue="POST">
            <GlFormRow label={tr("webhook.method")}>
              <Select options={METHOD_OPTIONS} />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="contentType" initialValue="application/json">
            <GlFormRow label={tr("webhook.contentType")}>
              <Select options={CONTENT_TYPE_OPTIONS} />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="certId">
            <GlFormRow label={tr("webhook.certificate")}>
              <Select
                allowClear
                showSearch
                optionFilterProp="label"
                options={certOptionsQuery.data ?? []}
                loading={certOptionsQuery.isLoading}
                placeholder={tr("webhook.selectCertificate")}
              />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="isEnabled" valuePropName="checked" initialValue={true}>
            <GlFormRow label={tc("enabled")} inline>
              <Switch />
            </GlFormRow>
          </Form.Item>
        </FormSection>

        <FormSection title={tc("webhookEvents")}>
          <Form.Item name="events">
            <GlFormRow label={tr("webhook.events")}>
              <Checkbox.Group options={WEBHOOK_EVENTS.map((e) => ({ label: e, value: e }))} />
            </GlFormRow>
          </Form.Item>
        </FormSection>

        <FormSection title={tc("webhookCustomization")}>
          <Form.Item name="headers" rules={[jsonFieldRule(tc("invalidJson"))]}>
            <GlFormRow label={tr("webhook.headers")}>
              <JsonEditor rows={4} placeholder='{"Authorization": "Bearer xxx"}' />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="customBody">
            <GlFormRow label={tr("webhook.customBody")}>
              <Input.TextArea
                rows={4}
                style={{ fontFamily: "monospace", fontSize: 12 }}
                placeholder='{"event": "{{.eventType}}", "data": {{.object}}}'
              />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="customHeaders" rules={[jsonFieldRule(tc("invalidJson"))]}>
            <GlFormRow label={tr("webhook.customHeaders")}>
              <JsonEditor rows={3} placeholder='{"X-Custom-Header": "value"}' />
            </GlFormRow>
          </Form.Item>
        </FormSection>

        <FormSection title={tc("webhookUserExtension")}>
          <Form.Item name="isUserExtended" valuePropName="checked">
            <GlFormRow label={tr("webhook.isUserExtended")} inline>
              <Switch />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="tokenFields">
            <GlFormRow label={tr("webhook.tokenFields")}>
              <Select
                mode="multiple"
                allowClear
                placeholder={tr("webhook.selectUserFields")}
                options={WEBHOOK_OBJECT_FIELDS.map((f) => ({ label: f, value: f }))}
              />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="objectFields">
            <GlFormRow label={tr("webhook.objectFields")}>
              <Select
                mode="multiple"
                allowClear
                placeholder={tr("webhook.selectObjectFields")}
                options={WEBHOOK_OBJECT_FIELDS.map((f) => ({ label: f, value: f }))}
              />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="singleOrgOnly" valuePropName="checked">
            <GlFormRow label={tr("webhook.singleOrgOnly")} inline>
              <Switch />
            </GlFormRow>
          </Form.Item>
        </FormSection>

        <FormSection title={tc("webhookRetry")}>
          <Form.Item name="maxRetries" initialValue={3}>
            <GlFormRow label={tr("webhook.maxRetries")}>
              <InputNumber min={0} max={10} style={{ width: "100%" }} />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="retryInterval" initialValue={60}>
            <GlFormRow label={tr("webhook.retryIntervalSeconds")}>
              <InputNumber min={0} style={{ width: "100%" }} />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="useExponentialBackoff" valuePropName="checked">
            <GlFormRow label={tr("webhook.useExponentialBackoff")} inline>
              <Switch />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="timeoutMs" initialValue={5000}>
            <GlFormRow label={tr("webhook.timeoutMs")}>
              <InputNumber min={100} style={{ width: "100%" }} />
            </GlFormRow>
          </Form.Item>
        </FormSection>
      </ResourceFormPage>

      {/* One-time signing secret. The plaintext only exists in the create
          response; closing this dialog discards it for good (BE-B-7). */}
      <Modal
        open={secretReveal !== null}
        title={tr("webhook.oneTimeSecretTitle")}
        closable={false}
        mask={{ closable: false }}
        keyboard={false}
        okText={tr("webhook.oneTimeSecretConfirm")}
        cancelButtonProps={{ style: { display: "none" } }}
        onOk={dismissSecretReveal}
      >
        <Space.Compact style={{ width: "100%" }}>
          <Input readOnly value={secretReveal ?? ""} />
          <Button onClick={handleCopySecret}>{tr("webhook.copySecret")}</Button>
        </Space.Compact>
        <Typography.Paragraph type="warning" style={{ marginTop: 12 }}>
          {tr("webhook.oneTimeSecretWarning")}
        </Typography.Paragraph>
      </Modal>
    </>
  );
}
