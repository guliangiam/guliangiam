import { App, Button, Form, Input, Modal, Select, Space, Tag, Switch } from "antd";
import { useMutation } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import {
  markNotificationRead,
  markAllNotificationsRead,
  type NotificationItem,
} from "@/shared/api/iam/notification";
import { broadcastNotification } from "@/shared/api/iam/user-notification";
import { getUserOptions } from "@/shared/api/iam/user";
import { iamListWithPagination, type IamListParams } from "@/shared/api/iam/client";
import { NOTIFICATIONS } from "@/shared/api/paths";
import { useCrudList } from "@/shared/hooks/useCrudList";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { useProviderNames } from "@/shared/hooks/useResourceNames";
import { localizedError } from "@/shared/lib/server-error";

const STATUS_COLORS: Record<string, string> = {
  pending: "processing",
  sent: "success",
  failed: "error",
};

function formatDateTime(value?: string | null) {
  if (!value) return "-";
  return new Date(value).toLocaleString();
}

export default function NotificationListPage() {
  const { t } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);
  const organization = useSessionStore((state) => state.organization);

  const { nameMap: providerNameMap } = useProviderNames();

  // ── 分页模式（原版 getNotifications 支持 page/pageSize params） ──
  const list = useCrudList<
    NotificationItem,
    { page: number; pageSize: number; search?: string },
    string
  >({
    buildQueryKey: (p) =>
      ["notifications", p.page, p.pageSize, p.search, organization?.id] as const,
    buildParams: ({ page, pageSize, search }) => ({
      page,
      pageSize,
      search: search || undefined,
      // organizationId 在 paginatedFn 内从 session 注入
    }),
    paginatedFn: (p) => {
      const params: IamListParams = {
        page: p.page,
        pageSize: p.pageSize,
      };
      if (organization?.id) params.organizationId = organization.id;
      if (p.search) params.search = p.search;
      // 服务端分页：total 取响应 page.total，而不是当前页数组长度（否则永远只有一页）。
      return iamListWithPagination<NotificationItem>(NOTIFICATIONS, params);
    },
    deleteFn: markNotificationRead,
    invalidateKeysOnDelete: [["notifications"]],
    onDeleteSuccess: () => message.success(t("success")),
    enabled: account !== null,
  });

  // ── pendingCount：status === "pending" 的数量 ──
  const pendingCount = useMemo(
    () => (list.query.data ?? []).filter((n) => n.status === "pending").length,
    [list.query.data],
  );

  const markAllReadMutation = useMutation({
    mutationFn: () => {
      if (!organization?.id) return Promise.reject(new Error(t("operationFailed")));
      return markAllNotificationsRead(organization.id);
    },
    onSuccess: () => {
      message.success(t("success"));
      list.refetch();
    },
    onError: (error) => {
      message.error(localizedError(error, t, "operationFailed"));
    },
  });

  // ── 管理员群发：写入每个用户的收件箱，并按偏好扇出外部渠道 ──
  const [broadcastOpen, setBroadcastOpen] = useState(false);
  const [broadcastForm] = Form.useForm();
  const [userOptions, setUserOptions] = useState<{ label: string; value: string }[]>([]);
  useEffect(() => {
    if (!broadcastOpen) return;
    void getUserOptions(organization?.id ?? undefined).then((users) =>
      setUserOptions(
        users.map((u) => ({
          label:
            u.displayName && u.displayName !== u.username
              ? `${u.displayName} (${u.username})`
              : u.username || u.displayName || u.id,
          value: u.id,
        })),
      ),
    );
  }, [broadcastOpen, organization?.id]);

  const broadcastMutation = useMutation({
    mutationFn: (values: {
      userIds: string[];
      category: string;
      title: string;
      content: string;
      link?: string;
      allowReply?: boolean;
    }) => broadcastNotification(values),
    onSuccess: (result) => {
      message.success(
        `已发送 ${result.delivered} 人${result.failed ? `，失败 ${result.failed} 人` : ""}`,
      );
      setBroadcastOpen(false);
      broadcastForm.resetFields();
      list.refetch();
    },
    onError: (error) => message.error(localizedError(error, t, "operationFailed")),
  });

  return (
    <>
      {renderCrudListView(
        {
          title: (
            <Space>
              {t("nav.notificationDeliveries", "Notifications")}
              {pendingCount > 0 && (
                <Tag color="processing">{t("pendingCount", { count: pendingCount })}</Tag>
              )}
            </Space>
          ),
          searchPlaceholder: t("placeholder.searchByName", "Search..."),
          rowKey: "id",
          // 行删除实为"标记已读"，批量动作按真实语义命名
          enableBulkDelete: true,
          bulkDeleteLabel: t("markRead", "Mark read"),
          bulkDeleteConfirmTitle: (count: number) => `${t("markRead", "Mark read")} (${count})`,
          toolbar: (
            <Space size={8}>
              <Button
                size="small"
                disabled={pendingCount === 0}
                onClick={() => markAllReadMutation.mutate()}
                loading={markAllReadMutation.isPending}
              >
                {t("markAllRead", "Mark all as read")}
              </Button>
              <Button size="small" type="primary" onClick={() => setBroadcastOpen(true)}>
                发送通知
              </Button>
            </Space>
          ),
          columns: [
            {
              title: t("notification.subject", "Subject"),
              dataIndex: "subject",
              key: "subject",
              width: 200,
              fixed: "left",
              ellipsis: true,
              render: (v: string) => v || "-",
            },
            {
              title: t("notification.provider", "Provider"),
              dataIndex: "providerId",
              key: "providerId",
              render: (id: string) => providerNameMap.get(id) ?? id,
            },
            {
              title: t("notification.recipient", "Recipient"),
              dataIndex: "recipient",
              key: "recipient",
            },
            {
              title: t("statusLabel"),
              dataIndex: "status",
              key: "status",
              render: (v: string) => <Tag color={STATUS_COLORS[v] ?? "default"}>{v}</Tag>,
            },
            {
              title: t("sentAt", "Sent At"),
              dataIndex: "sentAt",
              key: "sentAt",
              render: (v: string | null) => formatDateTime(v),
            },
            {
              title: t("createdAt"),
              dataIndex: "createdAt",
              key: "createdAt",
              render: (v: string) => formatDateTime(v),
            },
            {
              title: t("actions"),
              width: 160,
              key: "actions",
              fixed: "right",
              render: (_, record) => (
                <Space size={4}>
                  <Button
                    type="link"
                    size="small"
                    disabled={record.status !== "pending"}
                    onClick={() => list.deleteMutation.mutate(record.id)}
                  >
                    {t("markRead", "Mark read")}
                  </Button>
                </Space>
              ),
            },
          ],
        },
        list,
      )}
      <Modal
        open={broadcastOpen}
        title="发送通知"
        okText="发送"
        cancelText="取消"
        confirmLoading={broadcastMutation.isPending}
        onCancel={() => setBroadcastOpen(false)}
        onOk={() => broadcastForm.submit()}
      >
        <Form
          form={broadcastForm}
          layout="vertical"
          initialValues={{ category: "announcement", allowReply: false }}
          onFinish={(values) => broadcastMutation.mutate(values)}
        >
          <Form.Item
            name="userIds"
            label="接收人"
            rules={[{ required: true, message: "请选择至少一个接收人" }]}
          >
            <Select
              mode="multiple"
              showSearch
              optionFilterProp="label"
              options={userOptions}
              placeholder="选择用户"
            />
          </Form.Item>
          <Form.Item name="category" label="分类" rules={[{ required: true }]}>
            <Select
              options={[
                { value: "announcement", label: "公告" },
                { value: "system", label: "系统" },
                { value: "business", label: "业务" },
                { value: "security", label: "安全" },
              ]}
            />
          </Form.Item>
          <Form.Item name="title" label="标题" rules={[{ required: true }]}>
            <Input maxLength={120} />
          </Form.Item>
          <Form.Item name="content" label="内容" rules={[{ required: true }]}>
            <Input.TextArea rows={4} maxLength={2000} showCount />
          </Form.Item>
          <Form.Item name="link" label="跳转链接">
            <Input placeholder="/tickets 或 https://..." />
          </Form.Item>
          <Form.Item name="allowReply" label="允许回复" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
