/**
 * Organization 详情页 · Tab 5「计费与配额」
 *
 * 内含交易流水子表（`OrgTransactionSubTable`），它自行发起查询，
 * 故本组件只需要 `organizationId` 作为归属标识。
 */
import { Form, InputNumber, Select, Switch } from "antd";
import { useTranslation } from "react-i18next";
import { FormSection } from "@/shared/ui/FormSection";
import { ORG_CURRENCY_CODES, ORG_DEFAULT_CURRENCY_CODES } from "@/shared/lib/currency-utils";
import { OrgTransactionSubTable } from "../OrgSubTables";

type FinanceTabProps = {
  organizationId: string;
};

export function FinanceTab({ organizationId }: FinanceTabProps) {
  const { t: tr } = useTranslation("resource");

  return (
    <>
      <Form.Item
        label={tr("organization.balance")}
        name="balance"
        extra={tr("organization.balanceHint")}
      >
        <InputNumber min={0} precision={2} style={{ width: "100%" }} />
      </Form.Item>
      <Form.Item label={tr("organization.currency")} name="currency">
        <Select
          allowClear
          showSearch
          optionFilterProp="label"
          options={ORG_CURRENCY_CODES.map((c) => ({ label: c, value: c }))}
        />
      </Form.Item>
      <Form.Item
        label={tr("organization.enableDeposit")}
        name="enableDeposit"
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item
        label={tr("organization.balanceCredit")}
        name="balanceCredit"
        extra={tr("organization.balanceCreditHint")}
      >
        <InputNumber min={0} precision={2} style={{ width: "100%" }} />
      </Form.Item>
      <Form.Item label={tr("organization.mu")} name="mu">
        <InputNumber min={0} precision={2} style={{ width: "100%" }} />
      </Form.Item>
      <Form.Item
        label={tr("organization.defaultQuota")}
        name="defaultQuota"
        extra={tr("organization.defaultQuotaHint")}
      >
        <InputNumber min={0} style={{ width: "100%" }} />
      </Form.Item>
      <Form.Item label={tr("organization.defaultCurrency")} name="defaultCurrency">
        <Select
          allowClear
          showSearch
          options={ORG_DEFAULT_CURRENCY_CODES.map((c) => ({ label: c, value: c }))}
        />
      </Form.Item>
      <FormSection
        title={tr("organization.transactionsSection")}
        description={tr("organization.transactionsSectionDesc")}
      >
        <OrgTransactionSubTable organizationId={organizationId} />
      </FormSection>
    </>
  );
}
