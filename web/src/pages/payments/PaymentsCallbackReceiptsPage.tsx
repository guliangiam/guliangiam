// PaymentsCallbackReceiptsPage.tsx
// G7 渠道回调审计管理面 —— 列出所有 pay_callback_receipts 记录，用于运营排查
// "渠道是否真的回调了 / 回调 payload 是什么 / 处理是否成功"。
//
// 只读 —— 没有写操作（callback_receipt 的写入由回调 pipeline 负责）。
//
// 后端: GET /api/v1/pay-callback-receipts?organizationId=&page=&pageSize=
// Frontend: listCallbackReceipts() 见 shared/api/payments-admin.ts

import { useQuery } from "@tanstack/react-query";
import { Card, Table, Tag, Empty, Space, Button, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import { listCallbackReceipts, type CallbackReceiptListItem } from "@/shared/api/payments-admin";
import { useSessionStore } from "@/features/session/session-store";
import { ReloadOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import { useState } from "react";
import dayjs from "dayjs";

const { Title, Text, Paragraph } = Typography;

const PROCESS_STATE_MAP: Record<string, { color: string; label: string }> = {
  received: { color: "blue", label: "Received" },
  verified: { color: "cyan", label: "Verified" },
  parsed: { color: "geekblue", label: "Parsed" },
  applied: { color: "green", label: "Applied" },
  ignored: { color: "default", label: "Ignored" },
  error: { color: "red", label: "Error" },
};

function stateTag(state: string) {
  const m = PROCESS_STATE_MAP[state] ?? { color: "default", label: state };
  return <Tag color={m.color}>{m.label}</Tag>;
}

function shortSha(hash: string) {
  if (!hash) return "-";
  return hash.slice(0, 12) + "…";
}

export default function PaymentsCallbackReceiptsPage() {
  const organization = useSessionStore((state) => state.organization);
  const [tc] = useTranslation("common");
  // 服务端分页：页码/页大小进查询键与请求，翻页真正取回对应切片（FE-C-13）。
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const query = useQuery({
    queryKey: ["callbackReceipts", organization?.id, page, pageSize],
    queryFn: () =>
      organization?.id
        ? listCallbackReceipts({ organizationId: organization.id, page, pageSize })
        : Promise.resolve(undefined),
    enabled: !!organization?.id,
    refetchInterval: 30_000,
  });

  const data = query.data;
  const items = data?.items ?? [];
  const total = data?.page?.total ?? 0;

  const columns: ColumnsType<CallbackReceiptListItem> = [
    {
      title: tc("payments.channel", "Channel"),
      dataIndex: "channel",
      width: 120,
      fixed: "left",
    },
    {
      title: tc("payments.processState", "Process"),
      dataIndex: "processState",
      width: 110,
      render: (s: string) => stateTag(s),
    },
    {
      title: tc("payments.payloadSha256", "Payload SHA256"),
      dataIndex: "payloadSha256",
      width: 180,
      render: (h: string) => (
        <Text copyable code ellipsis={{ tooltip: h }} style={{ maxWidth: 180 }}>
          {shortSha(h)}
        </Text>
      ),
    },
    {
      title: tc("payments.eventKey", "Event Key"),
      dataIndex: "eventKey",
      width: 160,
      render: (v?: string | null) => v ?? "-",
    },
    {
      title: tc("payments.outTradeNo", "Out Trade No."),
      dataIndex: "outTradeNo",
      width: 180,
      render: (v?: string | null) => v ?? "-",
    },
    {
      title: tc("payments.clientIp", "Client IP"),
      dataIndex: "clientIp",
      width: 130,
      render: (v?: string | null) => v ?? "-",
    },
    {
      title: tc("payments.receivedAt", "Received At"),
      dataIndex: "receivedAt",
      width: 180,
      render: (v: string) => dayjs(v).format("YYYY-MM-DD HH:mm:ss"),
    },
    {
      title: tc("payments.error", "Error"),
      dataIndex: "errorMessage",
      ellipsis: true,
      render: (v?: string | null) => (v ? <Text type="danger">{v}</Text> : "-"),
    },
  ];

  return (
    <Card
      title={
        <Space>
          <Title level={4} style={{ margin: 0 }}>
            {tc("payments.callbackReceipts.title", "Callback Receipts")}
          </Title>
          <Tag color="blue">{total}</Tag>
        </Space>
      }
      extra={
        <Space>
          <Text type="secondary">
            {tc(
              "payments.callbackReceipts.subtitle",
              "Raw channel callbacks logged verbatim for audit",
            )}
          </Text>
          <Button
            icon={<ReloadOutlined />}
            onClick={() => query.refetch()}
            loading={query.isFetching}
          >
            {tc("common.refresh", "Refresh")}
          </Button>
        </Space>
      }
    >
      {items.length === 0 && !query.isFetching ? (
        <Empty
          description={tc(
            "payments.callbackReceipts.empty",
            "No callback receipts received yet — channels will appear here after they POST to /notify",
          )}
        />
      ) : (
        <>
          <Paragraph type="secondary" style={{ marginBottom: 16 }}>
            {tc(
              "payments.callbackReceipts.hint",
              "Every channel callback (charge result, refund result, ...) is logged verbatim. The " +
                "Process column tells you which pipeline stage it reached (received → verified → parsed → applied).",
            )}
          </Paragraph>
          <Table<CallbackReceiptListItem>
            rowKey="id"
            columns={columns}
            dataSource={items}
            loading={query.isFetching}
            size="small"
            pagination={{
              current: page,
              total,
              pageSize,
              showSizeChanger: true,
              showTotal: (n) => tc("total", { count: n }),
              showLessItems: true,
              onChange: (nextPage, nextPageSize) => {
                setPage(nextPageSize === pageSize ? nextPage : 1);
                setPageSize(nextPageSize);
              },
            }}
            scroll={{ x: 1150 }}
          />
        </>
      )}
    </Card>
  );
}
