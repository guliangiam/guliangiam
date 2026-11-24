import {
  Alert,
  Button,
  Card,
  Checkbox,
  Empty,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Result,
  Select,
  Skeleton,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
} from "antd";
import { RedoOutlined, DeleteOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import { SectionNavShell } from "@/features/section-nav/SectionNavShell";
import type { SectionNavItem } from "@/features/section-nav/types";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { FormSection } from "@/shared/ui/FormSection";
import { JsonEditor } from "@/shared/ui/JsonEditor";
import { jsonFieldRule } from "@/shared/lib/json-field-rules";
import WebhookHeaderTable from "@/shared/ui/WebhookHeaderTable";
import type { HeaderItem } from "@/shared/ui/WebhookHeaderTable";
import { GlFormRow } from "@/shared/ui/GlForm";
import {
  useWebhookDetailPageController,
  WEBHOOK_EVENTS,
  WEBHOOK_OBJECT_FIELDS,
  type WebhookFormValues,
  type IamWebhookEventItem,
} from "./useWebhookDetailPageController";

/**
 * Bridge between the stored headers JSON and WebhookHeaderTable rows.
 * Backend contract (BE-B-17): a JSON object with string values — the delivery
 * worker decodes map[string]string. Legacy rows may hold an array of
 * {name,value} items; both shapes are normalized into rows, and edits always
 * serialize back to the object shape so a save repairs legacy data.
 */
function WebhookHeaderTableInput({
  value,
  onChange,
}: {
  value?: string;
  onChange?: (v: string) => void;
}) {
  const items: HeaderItem[] = (() => {
    if (!value) return [];
    try {
      const parsed: unknown = JSON.parse(value);
      if (Array.isArray(parsed)) {
        return parsed
          .filter(
            (it): it is HeaderItem =>
              !!it && typeof it === "object" && typeof (it as HeaderItem).name === "string",
          )
          .map((it) => ({ name: it.name, value: String(it.value ?? "") }));
      }
      if (parsed && typeof parsed === "object") {
        return Object.entries(parsed as Record<string, unknown>).map(([name, v]) => ({
          name,
          value: String(v ?? ""),
        }));
      }
      return [];
    } catch {
      return [];
    }
  })();
  return (
    <WebhookHeaderTable
      value={items}
      onChange={(v) => {
        const headers: Record<string, string> = {};
        for (const item of v) {
          if (item.name.trim() !== "") headers[item.name] = item.value;
        }
        onChange?.(JSON.stringify(headers));
      }}
    />
  );
}

export default function WebhookDetailPage() {
  const navigate = useNavigate();
  const {
    id,
    tc,
    tr,
    form,
    METHOD_OPTIONS,
    CONTENT_TYPE_OPTIONS,
    detailQuery,
    orgOptionsQuery,
    certOptionsQuery,
    eventsQuery,
    updateMutation,
    deleteMutation,
    replayEventMutation,
    deleteEventMutation,
  } = useWebhookDetailPageController();

  if (detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }

  if (detailQuery.isError) {
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
          title={tc("notFound")}
          description={tr("webhook.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/webhooks")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  const sectionItems: SectionNavItem[] = [
    {
      key: "config",
      anchor: "webhook-section-config",
      label: tr("webhook.configTab", "Configuration"),
    },
    { key: "events", anchor: "webhook-section-events", label: tr("webhook.eventsTab", "Events") },
  ];

  return (
    <ResourceFormPage
      title={detailQuery.data.displayName || detailQuery.data.name}
      description={tr("webhook.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/webhooks")}
      onFinish={async (values: WebhookFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      submitPermission={{ permission: "webhook:write", resource: "webhooks" }}
      extra={
        <PermissionGuard action="webhook:write" resource="webhooks" fallback={null}>
          <Popconfirm
            title={tc("deleteConfirm")}
            onConfirm={() => deleteMutation.mutate()}
            okText={tc("delete")}
            cancelText={tc("cancel")}
            okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
          >
            <Button danger>{tc("delete")}</Button>
          </Popconfirm>
        </PermissionGuard>
      }
    >
      <SectionNavShell items={sectionItems}>
        <div className="gl-section-anchors">
          <div id="webhook-section-config" className="gl-section-anchor">
            <>
              <FormSection title={tc("overview", "Overview")}>
                <Form.Item label={tc("name")}>
                  <span>{detailQuery.data.name}</span>
                </Form.Item>
                <Form.Item label={tc("statusLabel")}>
                  {detailQuery.data.isEnabled ? (
                    <Tag color="green">{tc("enabled")}</Tag>
                  ) : (
                    <Tag color="red">{tc("disabled")}</Tag>
                  )}
                </Form.Item>
              </FormSection>

              <FormSection title={tr("webhook.configuration")}>
                <Form.Item name="displayName" rules={[{ required: true }]}>
                  <GlFormRow label={tc("displayName")} required>
                    <Input />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="organization">
                  <GlFormRow label={tr("webhook.organization")}>
                    <Select
                      allowClear
                      showSearch
                      placeholder={tr("webhook.selectOrganization")}
                      options={orgOptionsQuery.data ?? []}
                      loading={orgOptionsQuery.isLoading}
                    />
                  </GlFormRow>
                </Form.Item>
                <Form.Item
                  name="url"
                  rules={[
                    { required: true, message: tr("webhook.urlRequired") },
                    { type: "url", message: tc("form.invalidUrl") },
                  ]}
                >
                  <GlFormRow label={tr("webhook.url")} required>
                    <Input placeholder="https://example.com/webhook" />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="method">
                  <GlFormRow label={tr("webhook.method")}>
                    <Select options={METHOD_OPTIONS} />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="contentType">
                  <GlFormRow label={tr("webhook.contentType")}>
                    <Select options={CONTENT_TYPE_OPTIONS} />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="certId">
                  <GlFormRow label={tr("webhook.certificate")}>
                    <Select
                      allowClear
                      showSearch
                      placeholder={tr("webhook.selectCertificate")}
                      options={certOptionsQuery.data ?? []}
                      loading={certOptionsQuery.isLoading}
                    />
                  </GlFormRow>
                </Form.Item>
                {/* Read-only masked projection (BE-B-7): the plaintext is never
                    returned by reads and must not be prefilled into the form. */}
                <Form.Item label={tr("webhook.secretCurrent")}>
                  {detailQuery.data.secretSet ? (
                    <Typography.Text code>{detailQuery.data.secretMasked}</Typography.Text>
                  ) : (
                    <Typography.Text type="secondary">{tr("webhook.secretNotSet")}</Typography.Text>
                  )}
                </Form.Item>
                <Form.Item name="secret" extra={tr("webhook.secretKeepHint")}>
                  <GlFormRow label={tr("webhook.secret")}>
                    <Input.Password placeholder={tr("webhook.secretPlaceholder")} />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="isEnabled" valuePropName="checked">
                  <GlFormRow label={tc("enabled")} inline>
                    <Switch />
                  </GlFormRow>
                </Form.Item>
              </FormSection>

              <FormSection title={tr("webhook.events")}>
                <Form.Item name="events">
                  <GlFormRow label={tr("webhook.events")}>
                    <Checkbox.Group options={WEBHOOK_EVENTS.map((e) => ({ label: e, value: e }))} />
                  </GlFormRow>
                </Form.Item>
              </FormSection>

              <FormSection title={tr("webhook.requestCustomization")}>
                <Form.Item name="headers">
                  <GlFormRow label={tr("webhook.headers")}>
                    <WebhookHeaderTableInput />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="customBody">
                  <GlFormRow label={tr("webhook.customBody")}>
                    <Input.TextArea
                      rows={4}
                      style={{ fontFamily: "monospace", fontSize: 12 }}
                      placeholder='{"event": "{{.eventType}}", "data": {{.object}}}'
                    />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="customHeaders" rules={[jsonFieldRule(tc("invalidJson"))]}>
                  <GlFormRow label={tr("webhook.customHeaders")}>
                    <JsonEditor rows={3} placeholder='{"X-Custom-Header": "value"}' />
                  </GlFormRow>
                </Form.Item>
              </FormSection>

              <FormSection title={tr("webhook.userExtension")}>
                <Form.Item name="isUserExtended" valuePropName="checked">
                  <GlFormRow label={tr("webhook.isUserExtended")} inline>
                    <Switch />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="tokenFields">
                  <GlFormRow label={tr("webhook.tokenFields")}>
                    <Select
                      mode="multiple"
                      allowClear
                      placeholder={tr("webhook.selectUserFields")}
                      options={WEBHOOK_OBJECT_FIELDS.map((f) => ({ label: f, value: f }))}
                    />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="objectFields">
                  <GlFormRow label={tr("webhook.objectFields")}>
                    <Select
                      mode="multiple"
                      allowClear
                      placeholder={tr("webhook.selectObjectFields")}
                      options={WEBHOOK_OBJECT_FIELDS.map((f) => ({ label: f, value: f }))}
                    />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="singleOrgOnly" valuePropName="checked">
                  <GlFormRow label={tr("webhook.singleOrgOnly")} inline>
                    <Switch />
                  </GlFormRow>
                </Form.Item>
              </FormSection>

              <FormSection title={tr("webhook.retryConfiguration")}>
                <Form.Item name="maxRetries">
                  <GlFormRow label={tr("webhook.maxRetries")}>
                    <InputNumber min={0} max={10} style={{ width: "100%" }} />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="retryInterval">
                  <GlFormRow label={tr("webhook.retryIntervalSeconds")}>
                    <InputNumber min={0} style={{ width: "100%" }} />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="useExponentialBackoff" valuePropName="checked">
                  <GlFormRow label={tr("webhook.useExponentialBackoff")} inline>
                    <Switch />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="timeoutMs">
                  <GlFormRow label={tr("webhook.timeoutMs")}>
                    <InputNumber min={100} style={{ width: "100%" }} />
                  </GlFormRow>
                </Form.Item>
              </FormSection>

              {detailQuery.data.isError && (
                <FormSection title={tr("webhook.errorInformation")}>
                  <Form.Item label={tr("webhook.hasError")}>
                    <Tag color="red">{tc("yes")}</Tag>
                  </Form.Item>
                  {detailQuery.data.errorText && (
                    <Form.Item label={tr("webhook.errorDetails")}>
                      <Typography.Text type="danger" style={{ whiteSpace: "pre-wrap" }}>
                        {detailQuery.data.errorText}
                      </Typography.Text>
                    </Form.Item>
                  )}
                </FormSection>
              )}
            </>
          </div>
          <div id="webhook-section-events" className="gl-section-anchor">
            <FormSection title={tr("webhook.eventsTab", "Events")}>
              {eventsQuery.isLoading ? (
                <Skeleton active paragraph={{ rows: 6 }} />
              ) : (eventsQuery.data ?? []).length === 0 ? (
                <Empty description={tr("webhook.noEvents", "No webhook events yet")} />
              ) : (
                <Table
                  size="small"
                  dataSource={eventsQuery.data ?? []}
                  rowKey="id"
                  pagination={{ pageSize: 10 }}
                  columns={[
                    {
                      title: tr("webhook.eventType", "Event"),
                      dataIndex: "eventType",
                      width: 180,
                      render: (v: string) => <Tag>{v}</Tag>,
                    },
                    {
                      title: tr("webhook.eventStatus", "Status"),
                      dataIndex: "status",
                      width: 100,
                      render: (v: string) => {
                        const color = v === "delivered" ? "green" : v === "failed" ? "red" : "blue";
                        return <Tag color={color}>{v}</Tag>;
                      },
                    },
                    { title: tc("createdAt"), dataIndex: "createdAt", width: 170 },
                    {
                      title: tc("actions"),
                      width: 140,
                      render: (_: unknown, record: IamWebhookEventItem) => (
                        <PermissionGuard action="webhook:write" resource="webhooks" fallback={null}>
                          <Space size="small">
                            <Tooltip title={tr("webhook.replayEvent", "Replay event")}>
                              <Popconfirm
                                title={tr("webhookEvent.replayConfirm", "Replay this event?")}
                                onConfirm={() => replayEventMutation.mutate(record.id)}
                                okText={tr("webhookEvent.replay", "Replay")}
                                cancelText={tc("cancel")}
                              >
                                <Button
                                  size="small"
                                  type="text"
                                  icon={<RedoOutlined />}
                                  loading={
                                    replayEventMutation.isPending &&
                                    replayEventMutation.variables === record.id
                                  }
                                />
                              </Popconfirm>
                            </Tooltip>
                            <Popconfirm
                              title={tr("webhook.deleteEventConfirm", "Delete this event?")}
                              onConfirm={() => deleteEventMutation.mutate(record.id)}
                              okText={tc("delete")}
                              cancelText={tc("cancel")}
                              okButtonProps={{ danger: true }}
                            >
                              <Button size="small" type="text" danger icon={<DeleteOutlined />} />
                            </Popconfirm>
                          </Space>
                        </PermissionGuard>
                      ),
                    },
                  ]}
                />
              )}
            </FormSection>
          </div>
        </div>
      </SectionNavShell>
    </ResourceFormPage>
  );
}
