import {
  App,
  Badge,
  Button,
  Checkbox,
  Drawer,
  Empty,
  Input,
  List,
  Pagination,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { useUnreadNotificationCount } from "@/features/notifications/useUnreadNotificationCount";
import {
  batchDeleteMyNotifications,
  deleteMyNotification,
  deleteMyNotificationChannelBinding,
  listMyNotificationChannelBindings,
  listMyNotificationPreferences,
  listMyNotificationReplies,
  listMyNotifications,
  markAllMyNotificationsRead,
  markMyNotificationRead,
  markMyNotificationUnread,
  replyToMyNotification,
  updateMyNotificationPreferences,
  upsertMyNotificationChannelBinding,
  type UserNotificationChannelBinding,
  type UserNotificationItem,
  type UserNotificationPreference,
} from "@/shared/api/iam/user-notification";
import { localizedError } from "@/shared/lib/server-error";

const CATEGORY_VALUES = ["system", "security", "business", "feedback", "announcement"] as const;

const CATEGORY_COLORS: Record<string, string> = {
  system: "default",
  security: "red",
  business: "blue",
  feedback: "purple",
  announcement: "gold",
};

const CHANNEL_VALUES = ["inapp", "email", "sms", "wecom_app", "wechat_official"] as const;

type Translate = TFunction<"common">;

function categoryLabel(t: Translate, value: string) {
  return t(`notificationCategory.${value}`, value);
}

function channelLabel(t: Translate, value: string) {
  return t(`notificationChannel.${value}`, value);
}

function categoryOptions(t: Translate) {
  return CATEGORY_VALUES.map((value) => ({ value, label: categoryLabel(t, value) }));
}

function channelOptions(t: Translate) {
  return CHANNEL_VALUES.map((value) => ({ value, label: channelLabel(t, value) }));
}

function formatTime(value?: string | null) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
}

const INBOX_KEY = ["my-notifications"] as const;

export default function NotificationInboxPage() {
  const { t } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();

  const [activeTab, setActiveTab] = useState("inbox");
  const [category, setCategory] = useState<string | undefined>();
  const [unreadOnly, setUnreadOnly] = useState(false);
  const [keyword, setKeyword] = useState("");
  const [searchTerm, setSearchTerm] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [detail, setDetail] = useState<UserNotificationItem | null>(null);
  const [replyDraft, setReplyDraft] = useState("");

  const listQuery = useQuery({
    queryKey: [...INBOX_KEY, "list", { category, unreadOnly, searchTerm, page, pageSize }],
    queryFn: () =>
      listMyNotifications({
        category,
        isRead: unreadOnly ? false : undefined,
        keyword: searchTerm || undefined,
        page,
        pageSize,
      }),
  });

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: INBOX_KEY });
  };

  const readMutation = useMutation({
    mutationFn: ({ id, read }: { id: string; read: boolean }) =>
      read ? markMyNotificationRead(id) : markMyNotificationUnread(id),
    onSuccess: invalidate,
    onError: (error) => message.error(localizedError(error, t, "操作失败")),
  });

  const deleteMutation = useMutation({
    mutationFn: deleteMyNotification,
    onSuccess: () => {
      invalidate();
      message.success(t("deleted", "已删除"));
    },
    onError: (error) => message.error(localizedError(error, t, "删除失败")),
  });

  const batchDeleteMutation = useMutation({
    mutationFn: batchDeleteMyNotifications,
    onSuccess: (count) => {
      setSelectedIds([]);
      invalidate();
      message.success(`${t("deleted", "已删除")} ${count}`);
    },
    onError: (error) => message.error(localizedError(error, t, "删除失败")),
  });

  const markAllMutation = useMutation({
    mutationFn: () => markAllMyNotificationsRead(category),
    onSuccess: (count) => {
      invalidate();
      message.success(`${t("markedRead", "已标记已读")} ${count}`);
    },
    onError: (error) => message.error(localizedError(error, t, "操作失败")),
  });

  const repliesQuery = useQuery({
    queryKey: [...INBOX_KEY, "replies", detail?.id],
    queryFn: () => listMyNotificationReplies(detail!.id),
    enabled: Boolean(detail?.id),
  });

  const replyMutation = useMutation({
    mutationFn: ({ id, content }: { id: string; content: string }) =>
      replyToMyNotification(id, content),
    onSuccess: () => {
      setReplyDraft("");
      void queryClient.invalidateQueries({ queryKey: [...INBOX_KEY, "replies", detail?.id] });
    },
    onError: (error) => message.error(localizedError(error, t, "回复失败")),
  });

  const items = listQuery.data?.items ?? [];
  const total = listQuery.data?.total ?? 0;
  const allSelected = items.length > 0 && items.every((item) => selectedIds.includes(item.id));

  const openDetail = (item: UserNotificationItem) => {
    setDetail(item);
    if (!item.isRead) {
      readMutation.mutate({ id: item.id, read: true });
    }
  };

  const inboxTab = (
    <div className="gl-stack">
      <Space wrap>
        <Select
          allowClear
          placeholder={t("allCategories", "全部分类")}
          style={{ minWidth: 160 }}
          value={category}
          onChange={(value) => {
            setCategory(value);
            setPage(1);
            setSelectedIds([]);
          }}
          options={categoryOptions(t)}
        />
        <Space size={6}>
          <Switch
            checked={unreadOnly}
            onChange={(checked) => {
              setUnreadOnly(checked);
              setPage(1);
            }}
          />
          <span>{t("unreadOnly", "仅未读")}</span>
        </Space>
        <Input.Search
          allowClear
          placeholder={t("placeholder.searchByName", "搜索标题或内容")}
          style={{ width: 260 }}
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
          onSearch={(value) => {
            setSearchTerm(value.trim());
            setPage(1);
          }}
        />
        <Button onClick={() => markAllMutation.mutate()} loading={markAllMutation.isPending}>
          {t("markAllRead", "全部已读")}
        </Button>
        {selectedIds.length > 0 && (
          <Popconfirm
            title={`${t("batchDeleteConfirm", "确定删除选中的通知？")} (${selectedIds.length})`}
            onConfirm={() => batchDeleteMutation.mutate(selectedIds)}
          >
            <Button danger loading={batchDeleteMutation.isPending}>
              {t("batchDelete", "批量删除")} ({selectedIds.length})
            </Button>
          </Popconfirm>
        )}
      </Space>

      {items.length > 0 && (
        <Checkbox
          checked={allSelected}
          indeterminate={!allSelected && selectedIds.length > 0}
          onChange={(event) =>
            setSelectedIds(event.target.checked ? items.map((item) => item.id) : [])
          }
        >
          {t("selectAll", "全选本页")}
        </Checkbox>
      )}

      <List
        loading={listQuery.isLoading}
        dataSource={items}
        locale={{ emptyText: <Empty description={t("empty", "暂无通知")} /> }}
        renderItem={(item) => (
          <List.Item
            key={item.id}
            actions={[
              <Button
                key="read"
                type="link"
                size="small"
                onClick={() => readMutation.mutate({ id: item.id, read: !item.isRead })}
              >
                {item.isRead ? t("markUnread", "标记未读") : t("markRead", "标记已读")}
              </Button>,
              <Button key="view" type="link" size="small" onClick={() => openDetail(item)}>
                {t("detail", "详情")}
              </Button>,
              <Popconfirm
                key="delete"
                title={t("confirmDelete", "确定删除该通知？")}
                onConfirm={() => deleteMutation.mutate(item.id)}
              >
                <Button type="link" size="small" danger>
                  {t("delete", "删除")}
                </Button>
              </Popconfirm>,
            ]}
          >
            <List.Item.Meta
              avatar={
                <Checkbox
                  checked={selectedIds.includes(item.id)}
                  onChange={(event) =>
                    setSelectedIds((prev) =>
                      event.target.checked
                        ? [...prev, item.id]
                        : prev.filter((id) => id !== item.id),
                    )
                  }
                />
              }
              title={
                <Space size={8} wrap>
                  <Badge status={item.isRead ? "default" : "processing"} />
                  <Typography.Text strong={!item.isRead}>{item.title}</Typography.Text>
                  <Tag color={CATEGORY_COLORS[item.category] ?? "default"}>
                    {categoryLabel(t, item.category)}
                  </Tag>
                </Space>
              }
              description={
                <div>
                  <Typography.Paragraph
                    type={item.isRead ? "secondary" : undefined}
                    ellipsis={{ rows: 2 }}
                    style={{ marginBottom: 4 }}
                  >
                    {item.content}
                  </Typography.Paragraph>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {formatTime(item.createdAt)}
                  </Typography.Text>
                </div>
              }
            />
          </List.Item>
        )}
      />

      <Pagination
        align="end"
        current={page}
        pageSize={pageSize}
        total={total}
        showSizeChanger
        pageSizeOptions={[10, 20, 50]}
        showTotal={(count) => t("total", { count })}
        onChange={(nextPage, nextSize) => {
          setPage(nextPage);
          setPageSize(nextSize);
          setSelectedIds([]);
        }}
      />
    </div>
  );

  return (
    <div className="gl-content__inner--reading">
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        {t("notificationCenter", "通知中心")}
      </Typography.Title>
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          {
            key: "inbox",
            label: <UnreadTabLabel>{t("inbox", "收件箱")}</UnreadTabLabel>,
            children: inboxTab,
          },
          {
            key: "preferences",
            label: t("preferences", "通知偏好"),
            children: <NotificationPreferencesPanel />,
          },
          {
            key: "bindings",
            label: t("bindings", "渠道绑定"),
            children: <ChannelBindingsPanel />,
          },
        ]}
      />

      <Drawer
        open={Boolean(detail)}
        width={560}
        title={detail?.title}
        onClose={() => {
          setDetail(null);
          setReplyDraft("");
        }}
      >
        {detail && (
          <div className="gl-stack">
            <Space wrap>
              <Tag color={CATEGORY_COLORS[detail.category] ?? "default"}>
                {categoryLabel(t, detail.category)}
              </Tag>
              <Typography.Text type="secondary">{formatTime(detail.createdAt)}</Typography.Text>
            </Space>
            <Typography.Paragraph style={{ whiteSpace: "pre-wrap" }}>
              {detail.content}
            </Typography.Paragraph>
            {detail.link && (
              <Typography.Link href={detail.link} target="_blank" rel="noreferrer">
                {t("viewSource", "查看相关页面")}
              </Typography.Link>
            )}
            {detail.allowReply && (
              <>
                <Typography.Title level={5} style={{ marginBottom: 0 }}>
                  {t("replies", "回复记录")}
                </Typography.Title>
                <List
                  size="small"
                  loading={repliesQuery.isLoading}
                  dataSource={repliesQuery.data ?? []}
                  locale={{ emptyText: t("noReplies", "暂无回复") }}
                  renderItem={(reply) => (
                    <List.Item>
                      <List.Item.Meta
                        description={formatTime(reply.createdAt)}
                        title={reply.content}
                      />
                    </List.Item>
                  )}
                />
                <Input.TextArea
                  rows={3}
                  maxLength={2000}
                  showCount
                  value={replyDraft}
                  placeholder={t("replyPlaceholder", "输入回复内容")}
                  onChange={(event) => setReplyDraft(event.target.value)}
                />
                <Button
                  type="primary"
                  disabled={!replyDraft.trim()}
                  loading={replyMutation.isPending}
                  onClick={() =>
                    replyMutation.mutate({ id: detail.id, content: replyDraft.trim() })
                  }
                >
                  {t("sendReply", "发送回复")}
                </Button>
              </>
            )}
          </div>
        )}
      </Drawer>
    </div>
  );
}

function UnreadTabLabel({ children }: { children: React.ReactNode }) {
  const { data } = useUnreadNotificationCount();
  return (
    <Space size={6}>
      {children}
      {data ? <Badge count={data} size="small" /> : null}
    </Space>
  );
}

function NotificationPreferencesPanel() {
  const { t } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<Record<string, boolean>>({});

  const query = useQuery({
    queryKey: [...INBOX_KEY, "preferences"],
    queryFn: listMyNotificationPreferences,
  });

  const rows = useMemo(() => {
    const map = new Map<string, Record<string, boolean>>();
    for (const item of query.data ?? []) {
      const row = map.get(item.category) ?? {};
      row[item.channel] = item.enabled;
      map.set(item.category, row);
    }
    return map;
  }, [query.data]);

  const isEnabled = (category: string, channel: string) =>
    draft[`${category}:${channel}`] ?? rows.get(category)?.[channel] ?? channel === "inapp";

  const toggle = (category: string, channel: string, enabled: boolean) => {
    setDraft((prev) => ({ ...prev, [`${category}:${channel}`]: enabled }));
  };

  const saveMutation = useMutation({
    mutationFn: () => {
      const preferences: UserNotificationPreference[] = [];
      for (const categoryOption of CATEGORY_VALUES) {
        for (const channel of CHANNEL_VALUES) {
          // 站内信始终可用，不提供关闭入口，避免用户把通知彻底静音。
          if (channel === "inapp") continue;
          preferences.push({
            category: categoryOption,
            channel,
            enabled: isEnabled(categoryOption, channel),
          });
        }
      }
      return updateMyNotificationPreferences(preferences);
    },
    onSuccess: () => {
      message.success(t("saved", "已保存"));
      void queryClient.invalidateQueries({ queryKey: [...INBOX_KEY, "preferences"] });
    },
    onError: (error) => message.error(localizedError(error, t, "保存失败")),
  });

  const channels = channelOptions(t).filter((channel) => channel.value !== "inapp");
  return (
    <div className="gl-stack">
      <Typography.Paragraph type="secondary">
        {t(
          "preferencesHint",
          "站内信始终开启；外部渠道仅在对应提供商已配置且账号已绑定后才会实际触达。",
        )}
      </Typography.Paragraph>
      <Table
        rowKey="category"
        loading={query.isLoading}
        pagination={false}
        dataSource={categoryOptions(t)}
        columns={[
          {
            title: t("category", "分类"),
            dataIndex: "label",
            width: 160,
          },
          ...channels.map((channel) => ({
            title: channel.label,
            key: channel.value,
            render: (_: unknown, row: { value: string }) => (
              <Switch
                checked={isEnabled(row.value, channel.value)}
                onChange={(checked) => toggle(row.value, channel.value, checked)}
              />
            ),
          })),
        ]}
      />
      <Button type="primary" loading={saveMutation.isPending} onClick={() => saveMutation.mutate()}>
        {t("save", "保存")}
      </Button>
    </div>
  );
}

function ChannelBindingsPanel() {
  const { t } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [channel, setChannel] = useState("wecom_app");
  const [externalId, setExternalId] = useState("");

  const query = useQuery({
    queryKey: [...INBOX_KEY, "bindings"],
    queryFn: listMyNotificationChannelBindings,
  });

  const upsertMutation = useMutation({
    mutationFn: () =>
      upsertMyNotificationChannelBinding({ channel, externalId: externalId.trim() }),
    onSuccess: () => {
      setExternalId("");
      message.success(t("saved", "已保存"));
      void queryClient.invalidateQueries({ queryKey: [...INBOX_KEY, "bindings"] });
    },
    onError: (error) => message.error(localizedError(error, t, "保存失败")),
  });

  const deleteMutation = useMutation({
    mutationFn: deleteMyNotificationChannelBinding,
    onSuccess: () => {
      message.success(t("deleted", "已删除"));
      void queryClient.invalidateQueries({ queryKey: [...INBOX_KEY, "bindings"] });
    },
    onError: (error) => message.error(localizedError(error, t, "删除失败")),
  });

  return (
    <div className="gl-stack">
      <Typography.Paragraph type="secondary">
        {t(
          "bindingsHint",
          "企业微信填写成员 UserID，微信公众号填写 OpenID；未绑定的渠道不会向你推送外部消息。",
        )}
      </Typography.Paragraph>
      <Space wrap>
        <Select
          style={{ width: 180 }}
          value={channel}
          onChange={setChannel}
          options={channelOptions(t).filter(
            (item) => item.value !== "inapp" && item.value !== "email" && item.value !== "sms",
          )}
        />
        <Input
          style={{ width: 260 }}
          value={externalId}
          placeholder={t("externalId", "外部账号标识")}
          onChange={(event) => setExternalId(event.target.value)}
        />
        <Button
          type="primary"
          disabled={!externalId.trim()}
          loading={upsertMutation.isPending}
          onClick={() => upsertMutation.mutate()}
        >
          {t("addBinding", "新增/更新绑定")}
        </Button>
      </Space>
      <Table<UserNotificationChannelBinding>
        rowKey="id"
        loading={query.isLoading}
        pagination={false}
        dataSource={query.data ?? []}
        columns={[
          {
            title: t("channel", "渠道"),
            dataIndex: "channel",
            render: (value: string) => channelLabel(t, value),
          },
          { title: t("externalId", "外部账号标识"), dataIndex: "externalId" },
          {
            title: t("verified", "已验证"),
            dataIndex: "verified",
            render: (value: boolean) => (
              <Tag color={value ? "green" : "default"}>{t(value ? "yes" : "no")}</Tag>
            ),
          },
          {
            title: t("createdAt", "创建时间"),
            dataIndex: "createdAt",
            render: (value: string) => formatTime(value),
          },
          {
            title: t("actions", "操作"),
            key: "actions",
            render: (_: unknown, row) => (
              <Popconfirm
                title={t("confirmUnbind", "确定解绑该渠道？")}
                onConfirm={() => deleteMutation.mutate(row.id)}
              >
                <Button type="link" danger size="small">
                  {t("unbind", "解绑")}
                </Button>
              </Popconfirm>
            ),
          },
        ]}
      />
    </div>
  );
}
