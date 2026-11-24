import {
  App,
  Button,
  DatePicker,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Space,
  Tag,
  Typography,
} from "antd";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { type Dayjs } from "dayjs";
import { useTranslation } from "react-i18next";
import {
  createAccessCampaign,
  deleteAccessCampaign,
  listAccessCampaigns,
  setAccessCampaignStatus,
  type AccessCampaignItem,
  type AccessCampaignStatus,
} from "@/shared/api/iam/iga";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  serverErrorMessage,
} from "@/shared/lib/server-error";

type CampaignFormValues = { name: string; window?: [Dayjs, Dayjs] };

const statusColor: Record<string, string> = {
  draft: "default",
  active: "green",
  completed: "blue",
  cancelled: "red",
};

export default function IgaAccessCampaignsTab() {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm<CampaignFormValues>();

  const query = useQuery({
    queryKey: ["iga", "campaigns"],
    queryFn: listAccessCampaigns,
    select: (data) => data?.items ?? [],
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["iga", "campaigns"] });

  const createMutation = useMutation({
    mutationFn: createAccessCampaign,
    onSuccess: () => {
      void invalidate();
      message.success(tc("created"));
      setOpen(false);
      form.resetFields();
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(form, err, buildServerFieldResolver<CampaignFormValues>(form), tc);
      }
      message.error(serverErrorMessage(err, tc, "common:operationFailed"));
    },
  });

  const statusMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: AccessCampaignStatus }) =>
      setAccessCampaignStatus(id, status),
    onSuccess: () => {
      void invalidate();
      message.success(tc("updateSuccess"));
    },
    onError: (err) => message.error(serverErrorMessage(err, tc, "common:operationFailed")),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteAccessCampaign(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["iga", "campaigns"] });
      message.success(tc("deleteSuccess"));
    },
    onError: (err) => message.error(serverErrorMessage(err, tc, "common:operationFailed")),
  });

  const submit = () => {
    void form.validateFields().then((values) => {
      const [start, end] = values.window ?? [];
      createMutation.mutate({
        name: values.name,
        startDate: start?.toISOString(),
        endDate: end?.toISOString(),
      });
      return undefined;
    });
  };

  return (
    <>
      <ResourceListCard<AccessCampaignItem>
        title={tc("nav.iga.campaigns")}
        description={tc("iga.campaigns.description")}
        query={query}
        rowKey="id"
        emptyContent={
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <Space orientation="vertical" size={4}>
                <Typography.Text strong>{tc("noData")}</Typography.Text>
                <Typography.Text type="secondary">
                  {tc("iga.campaigns.description")}
                </Typography.Text>
              </Space>
            }
          >
            <PermissionGuard action="iga:write" resource="iga" fallback={null}>
              <Button type="primary" onClick={() => setOpen(true)}>
                {tc("create")}
              </Button>
            </PermissionGuard>
          </Empty>
        }
        toolbar={
          <PermissionGuard action="iga:write" resource="iga" fallback={null}>
            <Button type="primary" onClick={() => setOpen(true)}>
              {tc("create")}
            </Button>
          </PermissionGuard>
        }
        columns={[
          { title: tc("name"), dataIndex: "name", key: "name" },
          {
            title: tc("status"),
            dataIndex: "status",
            key: "status",
            render: (v: string) => <Tag color={statusColor[v] ?? "default"}>{v}</Tag>,
          },
          {
            title: tc("iga.startAt"),
            dataIndex: "startDate",
            key: "startDate",
            render: (v?: string) => (v ? new Date(v).toLocaleDateString() : "-"),
          },
          {
            title: tc("iga.endAt"),
            dataIndex: "endDate",
            key: "endDate",
            render: (v?: string) => (v ? new Date(v).toLocaleDateString() : "-"),
          },
          {
            title: tc("createdAt"),
            dataIndex: "createdAt",
            key: "createdAt",
            render: (v: string) => new Date(v).toLocaleDateString(),
          },
          {
            title: tc("actions"),
            key: "actions",
            width: 220,
            render: (_, record) => (
              <PermissionGuard action="iga:write" resource="iga" fallback={null}>
                <Space size={0} wrap>
                  {record.status === "draft" && (
                    <Button
                      type="link"
                      size="small"
                      loading={
                        statusMutation.isPending && statusMutation.variables?.id === record.id
                      }
                      onClick={() => statusMutation.mutate({ id: record.id, status: "active" })}
                    >
                      {tc("iga.campaigns.start")}
                    </Button>
                  )}
                  {record.status === "active" && (
                    <Button
                      type="link"
                      size="small"
                      loading={
                        statusMutation.isPending && statusMutation.variables?.id === record.id
                      }
                      onClick={() => statusMutation.mutate({ id: record.id, status: "completed" })}
                    >
                      {tc("iga.campaigns.complete")}
                    </Button>
                  )}
                  {(record.status === "draft" || record.status === "active") && (
                    <Popconfirm
                      title={tc("iga.campaigns.cancelConfirm")}
                      okText={tc("iga.campaigns.cancel")}
                      cancelText={tc("cancel")}
                      okButtonProps={{ danger: true }}
                      onConfirm={() =>
                        statusMutation.mutate({ id: record.id, status: "cancelled" })
                      }
                    >
                      <Button type="link" size="small" danger>
                        {tc("iga.campaigns.cancel")}
                      </Button>
                    </Popconfirm>
                  )}
                  <Popconfirm
                    title={tc("confirmDelete")}
                    okText={tc("delete")}
                    cancelText={tc("cancel")}
                    okButtonProps={{ danger: true }}
                    onConfirm={() => deleteMutation.mutate(record.id)}
                  >
                    <Button
                      type="link"
                      size="small"
                      danger
                      loading={deleteMutation.isPending && deleteMutation.variables === record.id}
                    >
                      {tc("delete")}
                    </Button>
                  </Popconfirm>
                </Space>
              </PermissionGuard>
            ),
          },
        ]}
      />

      <Modal
        open={open}
        title={tc("iga.campaigns.createTitle")}
        onCancel={() => setOpen(false)}
        onOk={submit}
        confirmLoading={createMutation.isPending}
      >
        <Form form={form} layout="vertical">
          <Form.Item label={tc("name")} name="name" rules={[{ required: true }]}>
            <Input placeholder={tc("iga.campaigns.namePlaceholder")} />
          </Form.Item>
          <Form.Item label={tc("iga.campaigns.window")} name="window">
            <DatePicker.RangePicker showTime style={{ width: "100%" }} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
