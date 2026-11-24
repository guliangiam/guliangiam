import { useEffect } from "react";
import { App, Card, Form, InputNumber, Select } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamTransaction } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { UserSelect } from "@/shared/ui/UserSelect";
import { OrderSelect } from "@/shared/ui/OrderSelect";
import { TRANSACTION_TYPES, stateOptions } from "@/shared/constants/enums";
import { localizedError } from "@/shared/lib/server-error";

type CreateTransactionFormValues = {
  organizationId: string;
  transactionType?: string;
  amount?: number;
  currency?: string;
  orderId?: string;
  userId?: string;
};

export default function TransactionCreatePage() {
  const { t: tr } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<CreateTransactionFormValues>();

  const createMutation = useMutation({
    mutationFn: async (values: CreateTransactionFormValues) =>
      createIamTransaction({
        organizationId: values.organizationId,
        // transactions 表没有 name/displayName/description 列，后端请求体也不接受。
        transactionType: values.transactionType ?? "payment",
        amount: Number(values.amount ?? 0),
        currency: values.currency,
        orderId: values.orderId || undefined,
        userId: values.userId || undefined,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.transactions.all });
      message.success(tr("transaction.created", "Transaction created"));
      navigate("/transactions");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "transaction.createFailed"));
    },
  });

  useEffect(() => {
    form.setFieldsValue({ currency: "CNY" });
  }, [form]);

  return (
    <Card className="gl-panel gl-resource-card" variant="borderless">
      <ResourceFormPage
        title={tr("transaction.createTitle", "Create Transaction")}
        description={tr("transaction.createDesc", "Create a new transaction record.")}
        loading={false}
        saving={createMutation.isPending}
        form={form}
        onCancel={() => navigate("/transactions")}
        onFinish={async (values: CreateTransactionFormValues) => {
          await createMutation.mutateAsync(values);
        }}
      >
        <Form.Item
          name="organizationId"
          rules={[
            {
              required: true,
              message: tr("transaction.orgIdRequired", "Please select Organization"),
            },
          ]}
        >
          <GlFormRow label={tr("transaction.orgId", "Organization ID")} required>
            <OrganizationSelect />
          </GlFormRow>
        </Form.Item>
        <Form.Item
          name="transactionType"
          rules={[
            {
              required: true,
              message: tc("required"),
            },
          ]}
        >
          <GlFormRow label={tr("transaction.type", "Transaction Type")} required>
            <Select allowClear options={stateOptions(TRANSACTION_TYPES, tc)} />
          </GlFormRow>
        </Form.Item>
        <Form.Item
          name="amount"
          rules={[
            { required: true, message: tr("transaction.amountRequired", "Please enter amount") },
            {
              validator: (_, value) =>
                value === undefined || value === null || value > 0
                  ? Promise.resolve()
                  : Promise.reject(
                      new Error(tr("transaction.amountPositive", "Amount must be greater than 0")),
                    ),
            },
          ]}
        >
          <GlFormRow label={tr("transaction.amount", "Amount")} required>
            <InputNumber min={0} style={{ width: "100%" }} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="currency">
          <GlFormRow label={tr("transaction.currency", "Currency")}>
            <Select>
              <Select.Option value="CNY">CNY</Select.Option>
              <Select.Option value="USD">USD</Select.Option>
              <Select.Option value="EUR">EUR</Select.Option>
            </Select>
          </GlFormRow>
        </Form.Item>
        <Form.Item name="orderId">
          <GlFormRow label={tr("transaction.orderId", "Order")}>
            <OrderSelect allowClear />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="userId">
          <GlFormRow label={tr("transaction.userId", "User")}>
            <UserSelect allowClear />
          </GlFormRow>
        </Form.Item>
      </ResourceFormPage>
    </Card>
  );
}
