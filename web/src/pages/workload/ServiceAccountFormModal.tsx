import { App, Form, Input, InputNumber, Modal, Select } from "antd";
import { useTranslation } from "react-i18next";
import type {
  CreateServiceAccountRequest,
  ServiceAccountCredentials,
} from "@/shared/api/iam/identity-pools";
import { useCreateServiceAccount } from "@/shared/hooks/use-identity-pools";
import { handleMutationError } from "@/shared/hooks/useMutationError";
import { isHttpError } from "@/shared/api/http";
import { applyServerFieldErrors, buildServerFieldResolver } from "@/shared/lib/server-error";
import { getServiceAccountCredentials } from "./workload-utils";

type ServiceAccountFormModalProps = {
  open: boolean;
  poolId: string;
  onClose: () => void;
  onCreated: (credentials: ServiceAccountCredentials) => void;
};

export function ServiceAccountFormModal({
  open,
  poolId,
  onClose,
  onCreated,
}: ServiceAccountFormModalProps) {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const [form] = Form.useForm<CreateServiceAccountRequest>();
  const createMutation = useCreateServiceAccount(poolId);

  const close = () => {
    form.resetFields();
    createMutation.reset();
    onClose();
  };

  const submit = async () => {
    const values = await form.validateFields().catch(() => undefined);
    if (!values) return;

    try {
      const account = await createMutation.mutateAsync(values);
      const credentials = getServiceAccountCredentials(account);
      createMutation.reset();
      message.success(tc("workload.accountCreated", { defaultValue: "Service account created" }));
      onCreated(credentials);
      close();
    } catch (error) {
      if (isHttpError(error)) {
        applyServerFieldErrors(
          form,
          error,
          buildServerFieldResolver<CreateServiceAccountRequest>(form),
          tc,
        );
      }
      handleMutationError(
        error,
        tc("workload.accountCreateFailed", { defaultValue: "Failed to create service account" }),
      );
    }
  };

  return (
    <Modal
      open={open}
      title={tc("workload.createAccount", { defaultValue: "Create Service Account" })}
      okText={tc("create", { defaultValue: "Create" })}
      cancelText={tc("cancel", { defaultValue: "Cancel" })}
      confirmLoading={createMutation.isPending}
      onOk={() => void submit()}
      onCancel={close}
      destroyOnHidden
    >
      <Form
        form={form}
        layout="vertical"
        initialValues={{ scopes: [], accessTokenTtlSeconds: 900 }}
      >
        <Form.Item
          label={tc("name", { defaultValue: "Name" })}
          name="name"
          rules={[{ required: true, whitespace: true }]}
        >
          <Input placeholder="deploy-runner" />
        </Form.Item>
        <Form.Item label={tc("description", { defaultValue: "Description" })} name="description">
          <Input.TextArea rows={3} />
        </Form.Item>
        <Form.Item label={tc("workload.scopes", { defaultValue: "Scopes" })} name="scopes">
          <Select mode="tags" tokenSeparators={[","]} placeholder="workload:deploy" />
        </Form.Item>
        <Form.Item
          label={tc("workload.accessTokenTtlSeconds", {
            defaultValue: "Access token TTL (seconds)",
          })}
          name="accessTokenTtlSeconds"
          rules={[
            { required: true },
            {
              type: "number",
              min: 60,
              max: 3600,
              message: tc("workload.ttlRange", {
                defaultValue: "TTL must be between 60 and 3600 seconds",
              }),
            },
          ]}
        >
          <InputNumber min={60} max={3600} step={60} style={{ width: "100%" }} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
