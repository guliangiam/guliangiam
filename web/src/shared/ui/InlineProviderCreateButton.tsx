import { useState } from "react";
import { Button, Form, Input, Modal, Select, App } from "antd";
import type { ButtonProps } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { createProvider, type CreateProviderPayload } from "@/shared/api/iam/provider-extended";
import { useSessionStore } from "@/features/session/session-store";
import { useProviderCategories } from "@/shared/hooks/useProviderCategories";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { localizedError } from "@/shared/lib/server-error";

type InlineProviderCreateButtonProps = {
  /** Called after a successful creation with the new provider id. */
  onCreated?: (id: string) => void;
  /** Text label for the trigger button. Defaults to "+ New Provider". */
  buttonText?: string;
  /** Button variant. Defaults to "link". */
  variant?: "link" | "button";
  /** Optional custom button props (e.g. disabled, style). */
  buttonProps?: ButtonProps;
};

type InlineProviderFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  category: string;
  providerType: string;
};

/**
 * A small button/link that opens a Modal to create a new Provider inline.
 * On success, closes the modal and invokes `onCreated` with the new id,
 * plus invalidates the "providers" query cache so parent consumers refresh.
 */
export function InlineProviderCreateButton({
  onCreated,
  buttonText,
  variant = "link",
  buttonProps,
}: InlineProviderCreateButtonProps) {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const sessionOrg = useSessionStore((state) => state.organization);

  const [open, setOpen] = useState(false);
  const [form] = Form.useForm<InlineProviderFormValues>();

  const watchedCategory = Form.useWatch("category", form) ?? "OAuth";
  const { categoryOptions, providerTypeOptions } = useProviderCategories({
    category: watchedCategory,
  });

  const mutation = useMutation({
    mutationFn: async (values: InlineProviderFormValues) => {
      const payload: CreateProviderPayload = {
        organizationId: (values.organizationId || sessionOrg?.id) ?? "",
        name: values.name,
        displayName: values.displayName,
        category: values.category,
        providerType: values.providerType,
      };
      return createProvider(payload);
    },
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: ["providers"] });
      message.success(tr("provider.providerCreated"));
      setOpen(false);
      form.resetFields();
      onCreated?.(result.id);
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "provider.saveFailed"));
    },
  });

  const openModal = () => {
    form.setFieldsValue({
      organizationId: sessionOrg?.id ?? "",
      name: "",
      displayName: "",
      category: "OAuth",
      providerType: "GitHub",
    });
    setOpen(true);
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      await mutation.mutateAsync(values);
    } catch {
      // validation errors shown inline
    }
  };

  const triggerText = buttonText ?? `+ ${t("create")} ${t("provider")}`;

  const trigger =
    variant === "link" ? (
      <Button type="link" {...buttonProps} onClick={openModal}>
        {triggerText}
      </Button>
    ) : (
      <Button {...buttonProps} onClick={openModal}>
        {triggerText}
      </Button>
    );

  return (
    <>
      {trigger}
      <Modal
        title={tr("provider.newProvider")}
        open={open}
        onOk={handleSubmit}
        onCancel={() => {
          setOpen(false);
          form.resetFields();
        }}
        confirmLoading={mutation.isPending}
        okText={t("create")}
        cancelText={t("cancel")}
        destroyOnHidden
        width={520}
      >
        <Form<InlineProviderFormValues>
          form={form}
          layout="vertical"
          initialValues={{
            category: "OAuth",
            providerType: "GitHub",
          }}
        >
          <Form.Item label={t("organization")} name="organizationId">
            <OrganizationSelect
              ariaLabel={t("organization")}
              style={{ width: "100%" }}
              allowClear
            />
          </Form.Item>
          <Form.Item
            label={t("name")}
            name="name"
            rules={[{ required: true, message: t("required") }]}
          >
            <Input placeholder={t("name")} />
          </Form.Item>
          <Form.Item
            label={t("displayName")}
            name="displayName"
            rules={[{ required: true, message: t("required") }]}
          >
            <Input placeholder={t("displayName")} />
          </Form.Item>
          <Form.Item
            label={t("category")}
            name="category"
            rules={[{ required: true, message: t("required") }]}
          >
            <Select
              options={categoryOptions}
              onChange={() => {
                form.setFieldValue("providerType", undefined);
              }}
            />
          </Form.Item>
          <Form.Item
            label={t("type")}
            name="providerType"
            rules={[{ required: true, message: t("required") }]}
          >
            <Select showSearch optionFilterProp="value" options={providerTypeOptions} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
