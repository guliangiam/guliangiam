import {
  Alert,
  Button,
  Card,
  Descriptions,
  Form,
  Image,
  Input,
  List,
  Popconfirm,
  Result,
  Select,
  Skeleton,
  Space,
  Tag,
} from "antd";
import type { ReactNode } from "react";
import { isHttpError } from "@/shared/api/http";
import type { IamTicketMessage } from "@/shared/api/iam";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { NotFoundResult } from "@/shared/ui/QueryErrorResult";
import { FormSection } from "@/shared/ui/FormSection";
import { StateTag } from "@/shared/ui/StateTag";
import { ticketPriorityLabel, ticketTypeOptions } from "@/shared/constants/enums";
import { formatDateTime } from "@/shared/lib/date-time";
import type { TicketFormValues } from "./useTicketDetailPageController";
import { useTicketDetailPageController } from "./useTicketDetailPageController";

type Props = { ctrl: ReturnType<typeof useTicketDetailPageController> };

const IMAGE_URL_PATTERN = /\.(jpe?g|png|gif|webp|svg|bmp)(\?[^#]*)?$/i;

/**
 * 附件字段是逗号分隔的 URL 列表（创建页截图上传写入的形态）。
 * 图片渲染缩略图并支持预览；非图片 URL 退化为链接，历史自由文本原样展示。
 */
function renderAttachmentList(raw: string | undefined, emptyText: string): ReactNode {
  const urls = (raw ?? "")
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
  if (urls.length === 0) return emptyText;
  return (
    <Space wrap size={8}>
      {urls.map((url) =>
        IMAGE_URL_PATTERN.test(url) ? (
          <Image
            key={url}
            src={url}
            width={72}
            height={72}
            style={{ objectFit: "cover", borderRadius: 4 }}
          />
        ) : (
          <a key={url} href={url} target="_blank" rel="noreferrer">
            {url}
          </a>
        ),
      )}
    </Space>
  );
}

export function TicketDetailPageView({ ctrl }: Props) {
  const {
    id,
    tc,
    tr,
    navigate,
    form,
    orgNameMap,
    replyText,
    setReplyText,
    detailQuery,
    messagesQuery,
    updateMutation,
    deleteMutation,
    replyMutation,
    TICKET_STATUSES,
    TICKET_TRANSITIONS,
    allowedTargets,
    PRIORITY_COLORS,
  } = ctrl;

  if (detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }

  if (detailQuery.isError) {
    if (isHttpError(detailQuery.error) && detailQuery.error.status === 404) {
      return <NotFoundResult title={tc("notFound")} description={tc("notFoundDesc")} />;
    }
    return (
      <Result
        status="error"
        title={tc("loadFailed")}
        subTitle={String(detailQuery.error)}
        extra={
          <Button type="primary" onClick={() => detailQuery.refetch()}>
            {tc("retry")}
          </Button>
        }
      />
    );
  }

  if (!detailQuery.data) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Alert
          type="error"
          showIcon
          title={tr("ticket.ticketNotFound")}
          description={tr("ticket.ticketNotFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/tickets")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  const data = detailQuery.data;

  return (
    <ResourceFormPage
      title={tr("ticket.ticketDetailTitle", { name: data.displayName || data.name })}
      description={tr("ticket.editTicketDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/tickets")}
      onFinish={async (values: TicketFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <Popconfirm
          title={tc("deleteConfirm")}
          description={tr("ticket.ticketDeleteDesc")}
          onConfirm={() => deleteMutation.mutate()}
          okText={tc("delete")}
          cancelText={tc("cancel")}
          okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
        >
          <Button danger>{tc("delete")}</Button>
        </Popconfirm>
      }
    >
      <FormSection title={tc("overview", "Overview")}>
        <Descriptions bordered column={{ xs: 1, sm: 2 }} size="small">
          <Descriptions.Item label={tc("name")}>{data.name}</Descriptions.Item>
          <Descriptions.Item label={tc("organization")}>
            {orgNameMap.get(data.organizationId) || data.organizationId}
          </Descriptions.Item>
          <Descriptions.Item label={tr("ticket.currentStatus")}>
            <StateTag value={data.status} />
          </Descriptions.Item>
          <Descriptions.Item label={tr("ticket.priority")}>
            <Tag color={PRIORITY_COLORS[data.priority] || "default"}>
              {ticketPriorityLabel(data.priority, tr)}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label={tr("ticket.creator")} span={{ xs: 1, sm: 2 }}>
            {data.creator || tr("ticket.none")}
          </Descriptions.Item>
          <Descriptions.Item label={tr("ticket.attachment")} span={{ xs: 1, sm: 2 }}>
            {renderAttachmentList(data.attachment, tr("ticket.none"))}
          </Descriptions.Item>
          <Descriptions.Item label={tr("ticket.resolvedAt")} span={{ xs: 1, sm: 2 }}>
            {data.resolvedAt || tr("ticket.unresolved")}
          </Descriptions.Item>
        </Descriptions>
      </FormSection>

      <FormSection title={tc("editSection", "Edit")}>
        <Form.Item
          label={tc("displayName")}
          name="displayName"
          rules={[{ required: true, message: tr("ticket.validation.displayNameRequired") }]}
        >
          <Input />
        </Form.Item>
        <Form.Item label={tc("type")} name="type">
          <Select options={ticketTypeOptions(tr)} placeholder={tr("ticket.typePlaceholder")} />
        </Form.Item>
        <Form.Item label={tc("statusLabel")} name="status">
          <Select options={allowedTargets(TICKET_STATUSES, data.status, TICKET_TRANSITIONS, tc)} />
        </Form.Item>
        <Form.Item label={tr("ticket.priority")} name="priority">
          <Select>
            <Select.Option value="Low">{tr("ticket.priorityLow")}</Select.Option>
            <Select.Option value="Medium">{tr("ticket.priorityMedium")}</Select.Option>
            <Select.Option value="High">{tr("ticket.priorityHigh")}</Select.Option>
            <Select.Option value="Urgent">{tr("ticket.priorityUrgent")}</Select.Option>
          </Select>
        </Form.Item>
        <Form.Item label={tr("ticket.assignee")} name="assignee">
          <Input />
        </Form.Item>
        <Form.Item label={tc("description")} name="description">
          <Input.TextArea rows={3} />
        </Form.Item>
      </FormSection>

      <FormSection title={tr("ticket.messageHistory")}>
        {messagesQuery.data && messagesQuery.data.length > 0 ? (
          <List
            dataSource={messagesQuery.data}
            renderItem={(msg: IamTicketMessage) => (
              <List.Item>
                <div style={{ width: "100%" }}>
                  <div style={{ display: "flex", justifyContent: "space-between" }}>
                    <strong>{msg.author || "System"}</strong>
                    <span style={{ color: "var(--gl-color-text-tertiary)", fontSize: 12 }}>
                      {formatDateTime(msg.createdAt)}
                    </span>
                  </div>
                  <p style={{ marginTop: 4 }}>{msg.content}</p>
                  {msg.attachment ? (
                    <div style={{ marginTop: 4 }}>{renderAttachmentList(msg.attachment, "")}</div>
                  ) : null}
                </div>
              </List.Item>
            )}
          />
        ) : (
          <p style={{ color: "var(--gl-color-text-tertiary)" }}>{tr("ticket.noMessages")}</p>
        )}

        {data.status !== "closed" && (
          <div style={{ marginTop: 16 }}>
            <Input.TextArea
              rows={3}
              value={replyText}
              onChange={(e) => setReplyText(e.target.value)}
              placeholder={tr("ticket.placeholder.replyContent")}
            />
            <Button
              type="primary"
              style={{ marginTop: 8 }}
              loading={replyMutation.isPending}
              disabled={!replyText.trim()}
              onClick={() => replyMutation.mutate()}
            >
              {tr("ticket.sendReply")}
            </Button>
          </div>
        )}
      </FormSection>
    </ResourceFormPage>
  );
}
