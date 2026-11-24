import { App, Button, Popconfirm, Tag } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  getIamWebhookEventsPaginated,
  replayIamWebhookEvent,
  deleteIamWebhookEvent,
  type IamWebhookEventItem,
} from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { serverErrorMessage } from "@/shared/lib/server-error";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { useCrudList } from "@/shared/hooks/useCrudList";

const statusColorMap: Record<string, string> = {
  delivered: "green",
  pending: "orange",
  failed: "red",
  retrying: "gold",
};

type WebhookEventParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
};

/** 展开行的载荷/响应体详情 — 文案由调用方传入，避免在页面组件内定义内嵌组件。 */
function renderWebhookEventDetail(
  record: IamWebhookEventItem,
  labels: { payload: string; responseBody: string; noPayload: string },
) {
  return (
    <div style={{ padding: "8px 0" }}>
      {record.payload !== null && record.payload !== undefined && (
        <div style={{ marginBottom: 8 }}>
          <strong>{labels.payload}:</strong>
          <pre
            style={{
              margin: "4px 0",
              padding: 8,
              background: "var(--ant-color-bg-container)",
              borderRadius: 4,
              fontSize: 12,
              maxHeight: 200,
              overflow: "auto",
            }}
          >
            {(() => {
              try {
                return JSON.stringify(JSON.parse(String(record.payload)), null, 2);
              } catch {
                return String(record.payload);
              }
            })()}
          </pre>
        </div>
      )}
      {record.responseBody && (
        <div>
          <strong>{labels.responseBody}:</strong>
          <pre
            style={{
              margin: "4px 0",
              padding: 8,
              background: "var(--ant-color-bg-container)",
              borderRadius: 4,
              fontSize: 12,
              maxHeight: 200,
              overflow: "auto",
            }}
          >
            {(() => {
              try {
                return JSON.stringify(JSON.parse(record.responseBody), null, 2);
              } catch {
                return record.responseBody;
              }
            })()}
          </pre>
        </div>
      )}
      {(record.payload === null || record.payload === undefined) && !record.responseBody && (
        <span style={{ color: "var(--ant-color-text-tertiary)" }}>{labels.noPayload}</span>
      )}
    </div>
  );
}

export default function WebhookEventsPage() {
  const { t: _tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { webhookId } = useParams<{ webhookId: string }>();
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const list = useCrudList<IamWebhookEventItem, WebhookEventParams, string>({
    enabled: !!webhookId,
    buildQueryKey: (p) =>
      ["webhook-events", "iam", webhookId, p.page, p.pageSize, p.search] as const,
    buildParams: ({ page, pageSize, search }) => ({
      page,
      pageSize,
      search: search || undefined,
    }),
    paginatedFn: (params) => getIamWebhookEventsPaginated(webhookId!, params),
    deleteFn: deleteIamWebhookEvent,
    invalidateKeysOnDelete: [queryKeys.webhookEvents.iam()],
    onDeleteSuccess: () => message.success(tr("webhookEvent.deleteSuccess", "Event deleted")),
    onDeleteError: (error) =>
      message.error(serverErrorMessage(error, tr, "webhookEvent.deleteFailed")),
  });

  const replayMutation = useMutation({
    mutationFn: async (eventId: string) => replayIamWebhookEvent(eventId),
    onSuccess: () => {
      message.success(tr("webhookEvent.replayTriggered", "Event replay triggered"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.webhookEvents.iam() });
    },
    onError: (error) => {
      message.error(serverErrorMessage(error, tr, "webhookEvent.replayFailed"));
    },
  });

  return renderCrudListView<IamWebhookEventItem, WebhookEventParams, string>(
    {
      // ── UI ──
      title: tr("webhookEvent.title", "Webhook Events"),
      description: tr("webhookEvent.description", { webhookId }),
      searchPlaceholder: tr("webhookEvent.searchPlaceholder", "Filter by event type"),
      rowKey: "id",
      enableBulkDelete: true,
      searchableColumns: [
        { dataIndex: "eventType", label: tr("webhookEvent.eventType", "Event Type") },
      ],
      toolbar: (
        <Button onClick={() => navigate("/webhooks")}>
          {tr("webhookEvent.backToWebhooks", "Back to Webhook List")}
        </Button>
      ),
      expandable: {
        rowExpandable: () => true,
        expandedRowRender: (record: IamWebhookEventItem) =>
          renderWebhookEventDetail(record, {
            payload: tr("webhookEvent.payload", "Request Payload"),
            responseBody: tr("webhookEvent.responseBody", "Response Body"),
            noPayload: tr("webhookEvent.noPayload", "No payload data available"),
          }),
      },
      columns: [
        {
          title: tr("webhookEvent.eventType", "Event Type"),
          dataIndex: "eventType",
          key: "eventType",
          width: 140,
          fixed: "left",
          render: (v: string) => <Tag>{v}</Tag>,
        },
        {
          title: tr("webhookEvent.status", "Status"),
          dataIndex: "status",
          key: "status",
          render: (v: string) => <Tag color={statusColorMap[v] ?? "default"}>{v}</Tag>,
        },
        {
          title: tr("webhookEvent.createdAt", "Created At"),
          dataIndex: "createdAt",
          key: "createdAt",
          render: (v: string) => (v ? new Date(v).toLocaleString() : "-"),
        },
        {
          title: tr("webhookEvent.actions", "Actions"),
          key: "actions",
          fixed: "right",
          width: 160,
          render: (_: unknown, record: IamWebhookEventItem) => (
            <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
              <Popconfirm
                title={tr("webhookEvent.replayConfirm", "Replay this event?")}
                onConfirm={() => replayMutation.mutate(record.id)}
                okText={tr("webhookEvent.replay", "Replay")}
                cancelText={tr("webhookEvent.cancel", "Cancel")}
              >
                <Button
                  type="link"
                  size="small"
                  loading={replayMutation.isPending && replayMutation.variables === record.id}
                >
                  {tr("webhookEvent.replay", "Replay")}
                </Button>
              </Popconfirm>
              <Popconfirm
                title={tr(
                  "webhookEvent.deleteConfirm",
                  "Are you sure you want to delete this event?",
                )}
                onConfirm={() => list.deleteMutation.mutate(record.id)}
                okText={tr("webhookEvent.delete", "Delete")}
                cancelText={tr("webhookEvent.cancel", "Cancel")}
              >
                <Button
                  type="link"
                  size="small"
                  danger
                  loading={
                    list.deleteMutation.isPending && list.deleteMutation.variables === record.id
                  }
                >
                  {tr("webhookEvent.delete", "Delete")}
                </Button>
              </Popconfirm>
            </div>
          ),
        },
      ],
    },
    list,
  );
}
