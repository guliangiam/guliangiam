import {
  Alert,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  Popconfirm,
  Result,
  Skeleton,
  Tag,
} from "antd";
import { Link } from "react-router-dom";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import type { EntryFormValues } from "./useEntryDetailPageController";
import { useEntryDetailPageController } from "./useEntryDetailPageController";

type Props = { ctrl: ReturnType<typeof useEntryDetailPageController> };

function getActionColor(action: string): string {
  if (action.includes("login") || action.includes("create")) return "green";
  if (action.includes("delete") || action.includes("revoke")) return "red";
  if (action.includes("update") || action.includes("patch")) return "blue";
  return "default";
}

export function EntryDetailPageView({ ctrl }: Props) {
  const { id, tc, tr, navigate, form, orgNameMap, detailQuery, updateMutation, deleteMutation } =
    ctrl;

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
          description={tc("notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/entries")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  const entry = detailQuery.data;

  return (
    <ResourceFormPage
      title={tr("entry.detailTitle", { name: entry.name })}
      description={tr("entry.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/entries")}
      onFinish={async (values: EntryFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <>
          <Button onClick={() => navigate(`/entries/${entry.id}/transcript`)}>
            {tr("entry.transcriptOpen")}
          </Button>
          <Popconfirm
            title={tr("entry.deleteConfirmTitle")}
            description={tr("entry.deleteConfirmDesc")}
            onConfirm={() => deleteMutation.mutate()}
            okText={tc("delete")}
            cancelText={tc("cancel")}
            okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
          >
            <Button danger>{tc("delete")}</Button>
          </Popconfirm>
        </>
      }
    >
      <FormSection title={tc("overview", "Overview")}>
        <Descriptions bordered column={{ xs: 1, sm: 2 }} size="small">
          <Descriptions.Item label={tc("organizationId")}>
            {orgNameMap.get(entry.organizationId) || entry.organizationId}
          </Descriptions.Item>
          <Descriptions.Item label={tc("name")}>{entry.name}</Descriptions.Item>
          <Descriptions.Item label={tc("type")}>
            <Tag color={getActionColor(entry.type)}>{entry.type || "-"}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label={tr("entry.provider")}>
            {entry.provider || "-"}
          </Descriptions.Item>
          <Descriptions.Item label={tr("entry.application")}>
            {entry.application || "-"}
          </Descriptions.Item>
          <Descriptions.Item label={tr("entry.clientIp")}>
            {entry.clientIp || "-"}
          </Descriptions.Item>
          <Descriptions.Item label={tr("entry.userAgent")} span={{ xs: 1, sm: 2 }}>
            <span style={{ wordBreak: "break-all" }}>{entry.userAgent || "-"}</span>
          </Descriptions.Item>
          <Descriptions.Item label={tr("entry.createdAt")} span={{ xs: 1, sm: 2 }}>
            {entry.createdAt ? new Date(entry.createdAt).toLocaleString() : "-"}
          </Descriptions.Item>
          {/* 42.3.13 条 6：payload 只读 transcript 通道 */}
          <Descriptions.Item label={tr("entry.message")} span={{ xs: 1, sm: 2 }}>
            <Link to={`/entries/${entry.id}/transcript`}>{tr("entry.transcriptOpen")}</Link>
          </Descriptions.Item>
        </Descriptions>
      </FormSection>

      <FormSection title={tr("entry.editSection", "Edit")}>
        <Form.Item
          label={tc("displayName")}
          name="displayName"
          rules={[{ required: true, message: tr("entry.displayNameRequired") }]}
        >
          <Input />
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}
