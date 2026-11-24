import {
  Alert,
  Card,
  Col,
  Form,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Table,
  Typography,
} from "antd";
import CartTable from "@/shared/ui/CartTable";
import { FormSection } from "@/shared/ui/FormSection";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { listUserTransactions } from "@/shared/api/iam/user";
import { serverErrorMessage } from "@/shared/lib/server-error";
import { formatDateTime, getBalanceCurrencyOptions } from "./types";
import type { ProductInfoItem, UserDetail } from "./types";

interface UserFinanceTabProps {
  user: UserDetail | undefined;
  cart: ProductInfoItem[];
  onChangeCart: (value: ProductInfoItem[]) => void;
  /** users:write；无写权限时金额与购物车只读。 */
  canWrite?: boolean;
}

/**
 * 金额格式化：后端 DECIMAL(19,4) 以 JSON number 返回，前端按 locale + 币种展示，
 * 避免直接渲染原始数字造成「分/元」与千分位误读。
 */
function formatMoney(value: unknown, currency: string | undefined, locale: string): string {
  const amount = typeof value === "number" ? value : Number(value ?? 0);
  if (!Number.isFinite(amount)) return String(value ?? "");
  const code = (currency || "USD").toUpperCase();
  try {
    return new Intl.NumberFormat(locale, {
      style: "currency",
      currency: code,
      minimumFractionDigits: 2,
      maximumFractionDigits: 4,
    }).format(amount);
  } catch {
    return `${amount.toFixed(2)} ${code}`;
  }
}

export default function UserFinanceTab({
  user,
  cart,
  onChangeCart,
  canWrite = true,
}: UserFinanceTabProps) {
  const { t, i18n } = useTranslation();
  const balanceCurrencyOptions = getBalanceCurrencyOptions(t);
  // 页码与用户绑定：切换用户时派生值回到第一页，避免沿用上一个用户的页码
  // 导致新用户看起来「没有流水」（无需副作用重置 state）。
  const [pageState, setPageState] = useState<{ userId?: string; page: number }>({ page: 1 });
  const page = pageState.userId === user?.id ? pageState.page : 1;
  const setPage = (next: number) => setPageState({ userId: user?.id, page: next });
  const pageSize = 20;

  const {
    data: transactionData,
    isLoading: transactionsLoading,
    isError: transactionsError,
    error: transactionsErrorValue,
  } = useQuery({
    queryKey: ["user", user?.id, "transactions", page],
    // 详情级端点要求可信租户：跨组织查看用户时必须显式携带所属组织。
    queryFn: () => listUserTransactions(user!.id, page, pageSize, user!.organizationId),
    enabled: !!user?.id,
  });

  return (
    <Space orientation="vertical" style={{ width: "100%" }} size="large">
      {/* 账户概览：原先在「账号与权限」中，现已合并进资产分区统一编辑/保存。 */}
      <FormSection title={t("resource:user.overviewSection", "Overview")}>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={t("resource:user.balance", "Balance")} name="balance">
              <InputNumber style={{ width: "100%" }} min={0} step={0.01} disabled={!canWrite} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={t("resource:user.balanceCredit", "Credit")} name="balanceCredit">
              <InputNumber style={{ width: "100%" }} min={0} step={0.01} disabled={!canWrite} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item
              label={t("resource:user.balanceCurrency", "Currency")}
              name="balanceCurrency"
            >
              <Select
                options={balanceCurrencyOptions}
                showSearch
                optionFilterProp="label"
                disabled={!canWrite}
              />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={8} md={8}>
            <Form.Item label={t("resource:user.score", "Score")} name="score">
              <InputNumber style={{ width: "100%" }} min={0} disabled={!canWrite} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={8} md={8}>
            <Form.Item label={t("resource:user.karma", "Karma")} name="karma">
              <InputNumber style={{ width: "100%" }} min={0} disabled={!canWrite} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={8} md={8}>
            <Form.Item label={t("resource:user.ranking", "Ranking")} name="ranking">
              <InputNumber style={{ width: "100%" }} min={0} disabled={!canWrite} />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={t("registerType")}>
              <Input value={user?.registerType || "-"} disabled />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={t("registerSource")}>
              <Input value={user?.registerSource || "-"} disabled />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={t("resource:user.ipWhitelist", "IP whitelist")} name="ipWhitelist">
              <Input
                placeholder={t("resource:user.placeholder.ipWhitelist")}
                disabled={!canWrite}
              />
            </Form.Item>
          </Col>
        </Row>
        <Typography.Text type="secondary">
          {formatMoney(user?.balance, user?.balanceCurrency, i18n.language)} ·{" "}
          {user?.balanceCurrency ?? "USD"}
        </Typography.Text>
      </FormSection>

      <Card title={t("resource:user.finance.cart", "Cart")} size="small">
        <CartTable value={cart} onChange={onChangeCart} readOnly={!canWrite} />
      </Card>

      <Card title={t("transactions", "Transactions")} size="small">
        {transactionsError ? (
          <Alert
            type="error"
            showIcon
            title={serverErrorMessage(transactionsErrorValue, t, "common:loadFailed")}
          />
        ) : (
          <Table
            dataSource={transactionData?.items ?? []}
            loading={transactionsLoading}
            rowKey="id"
            size="small"
            locale={{ emptyText: t("noTransactions", "No transactions") }}
            pagination={{
              current: page,
              pageSize,
              total: transactionData?.total ?? 0,
              showSizeChanger: false,
              onChange: setPage,
            }}
            columns={[
              { title: "ID", dataIndex: "id", key: "id", ellipsis: true },
              {
                title: t("transactionType", "Type"),
                dataIndex: "transactionType",
                key: "transactionType",
              },
              {
                title: t("transactionAmount"),
                dataIndex: "amount",
                key: "amount",
                render: (amount: number, record: { currency: string }) =>
                  formatMoney(amount, record.currency, i18n.language),
              },
              { title: t("transactionCurrency"), dataIndex: "currency", key: "currency" },
              { title: t("resource:user.status", "Status"), dataIndex: "status", key: "status" },
              {
                title: t("createdAt"),
                dataIndex: "createdAt",
                key: "createdAt",
                render: (v: string) => formatDateTime(v),
              },
            ]}
          />
        )}
      </Card>
    </Space>
  );
}
