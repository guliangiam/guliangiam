import type { Dayjs } from "dayjs";
import { App, DatePicker, Form, Input, Select, Switch } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamKey } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { GlFormRow } from "@/shared/ui/GlForm";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { KEY_ALGORITHM_OPTIONS, KEY_TYPE_OPTIONS } from "./useKeyDetailPageController";
import { localizedError } from "@/shared/lib/server-error";

/** 与后端 CreateKeyRequest 完全一致的字段集合。 */
type CreateKeyFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  keyType: string;
  algorithm?: string;
  publicKey: string;
  privateKey?: string;
  expiresAt?: Dayjs | null;
  isEnabled: boolean;
};

export default function KeyCreatePage() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<CreateKeyFormValues>();

  const createMutation = useMutation({
    mutationFn: async (values: CreateKeyFormValues) =>
      createIamKey({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        keyType: values.keyType,
        algorithm: values.algorithm,
        publicKey: values.publicKey,
        privateKey: values.privateKey,
        ...(values.expiresAt ? { expiresAt: values.expiresAt.toISOString() } : {}),
        isEnabled: values.isEnabled,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.keys.all });
      message.success(tr("key.created", "Key created"));
      navigate("/keys");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "key.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("key.createTitle", "New Key")}
      description={tr("key.createDesc", "Create a new key record.")}
      loading={false}
      saving={createMutation.isPending}
      form={form}
      onCancel={() => navigate("/keys")}
      onFinish={async (values: CreateKeyFormValues) => {
        await createMutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tr("key.orgIdRequired", "Please select organization") }]}
      >
        <GlFormRow label={tr("key.orgId", "Organization ID")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="name"
        rules={[{ required: true, message: tr("key.nameRequired", "Please enter name") }]}
      >
        <GlFormRow label={tr("key.name", "Name")} required>
          <Input />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[
          { required: true, message: tr("key.displayNameRequired", "Please enter display name") },
        ]}
      >
        <GlFormRow label={tr("key.displayName", "Display Name")} required>
          <Input />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="keyType" rules={[{ required: true, message: tr("key.typeRequired") }]}>
        <GlFormRow label={tr("key.type", "Type")} required>
          <Select options={KEY_TYPE_OPTIONS} placeholder={t("selectKeyType")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="algorithm">
        <GlFormRow label={t("algorithm")}>
          <Select options={KEY_ALGORITHM_OPTIONS} allowClear />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="publicKey" rules={[{ required: true, message: t("required") }]}>
        <GlFormRow label={t("publicKey")} required>
          <Input.TextArea rows={4} style={{ fontFamily: "monospace", fontSize: 12 }} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="privateKey">
        <GlFormRow label={t("privateKey")}>
          <Input.TextArea rows={4} style={{ fontFamily: "monospace", fontSize: 12 }} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="expiresAt">
        <GlFormRow label={t("expireTime")}>
          <DatePicker showTime style={{ width: "100%" }} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="isEnabled" valuePropName="checked" initialValue={true}>
        <GlFormRow label={tr("key.enabled", "Enabled")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
