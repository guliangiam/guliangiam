import { App, Form, Input, Modal } from "antd";
import { useTranslation } from "react-i18next";
import type { IdentityPoolItem } from "@/shared/api/iam/identity-pools";
import { useCreateIdentityPool } from "@/shared/hooks/use-identity-pools";
import { handleMutationError } from "@/shared/hooks/useMutationError";
import { isHttpError } from "@/shared/api/http";
import { applyServerFieldErrors, buildServerFieldResolver } from "@/shared/lib/server-error";

type PoolFormModalProps = {
  open: boolean;
  onClose: () => void;
  onCreated: (pool: IdentityPoolItem) => void;
};

type PoolFormValues = {
  name: string;
  displayName?: string;
  description?: string;
};

export function PoolFormModal({ open, onClose, onCreated }: PoolFormModalProps) {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const [form] = Form.useForm<PoolFormValues>();
  const createMutation = useCreateIdentityPool();

  const close = () => {
    form.resetFields();
    createMutation.reset();
    onClose();
  };

  const submit = async () => {
    const values = await form.validateFields().catch(() => undefined);
    if (!values) return;

    try {
      const pool = await createMutation.mutateAsync(values);
      message.success(tc("workload.poolCreated", { defaultValue: "Identity pool created" }));
      onCreated(pool);
      close();
    } catch (error) {
      if (isHttpError(error)) {
        applyServerFieldErrors(form, error, buildServerFieldResolver<PoolFormValues>(form), tc);
      }
      handleMutationError(
        error,
        tc("workload.poolCreateFailed", { defaultValue: "Failed to create identity pool" }),
      );
    }
  };

  return (
    <Modal
      open={open}
      title={tc("workload.createPool", { defaultValue: "Create Identity Pool" })}
      okText={tc("create", { defaultValue: "Create" })}
      cancelText={tc("cancel", { defaultValue: "Cancel" })}
      confirmLoading={createMutation.isPending}
      onOk={() => void submit()}
      onCancel={close}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" requiredMark="optional">
        <Form.Item
          label={tc("name", { defaultValue: "Name" })}
          name="name"
          rules={[{ required: true, whitespace: true, message: tc("required") }]}
        >
          <Input placeholder="production-workloads" />
        </Form.Item>
        <Form.Item label={tc("displayName", { defaultValue: "Display Name" })} name="displayName">
          <Input placeholder="Production Workloads" />
        </Form.Item>
        <Form.Item label={tc("description", { defaultValue: "Description" })} name="description">
          <Input.TextArea rows={3} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
