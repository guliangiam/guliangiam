import { App, Button, Form, Input, Modal, Select, Space, Tag } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  listActions,
  listActionVersions,
  publishActionVersion,
  type ActionVersionItem,
} from "@/shared/api/iam/actions";
import type { IamListResponse } from "@/shared/api/iam/client";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  serverErrorMessage,
} from "@/shared/lib/server-error";

export default function ActionVersionsTab() {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [selectedActionId, setSelectedActionId] = useState<string | undefined>(undefined);
  const [publishOpen, setPublishOpen] = useState(false);
  const [form] = Form.useForm<{ code: string }>();

  const actionsQuery = useQuery({
    queryKey: ["actions", "list"],
    queryFn: listActions,
    select: (data) => data?.items ?? [],
  });

  const versionsQuery = useQuery({
    queryKey: ["actions", selectedActionId, "versions"],
    queryFn: (): Promise<IamListResponse<ActionVersionItem> | undefined> =>
      selectedActionId
        ? listActionVersions(selectedActionId)
        : Promise.resolve({ items: [], page: { page: 1, pageSize: 10, total: 0 } }),
    select: (data) => data?.items ?? [],
    enabled: !!selectedActionId,
  });

  const publishMutation = useMutation({
    mutationFn: ({ id, code }: { id: string; code: string }) => publishActionVersion(id, code),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["actions"] });
      message.success(tc("created"));
      setPublishOpen(false);
      form.resetFields();
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(form, err, buildServerFieldResolver<{ code: string }>(form), tc);
      }
      message.error(serverErrorMessage(err, tc, "common:operationFailed"));
    },
  });

  const actionItems: { id: string; name: string }[] = actionsQuery.data ?? [];

  return (
    <>
      <Space style={{ marginBottom: 16 }} wrap>
        <span>{tc("actions.selectAction", "Select action")}:</span>
        <Select
          style={{ minWidth: 240 }}
          allowClear
          placeholder={tc("actions.selectActionPlaceholder", "Choose an action…")}
          value={selectedActionId}
          onChange={setSelectedActionId}
          options={actionItems.map((a) => ({ value: a.id, label: a.name }))}
        />
        {selectedActionId && (
          <PermissionGuard action="actions:write" resource="actions" fallback={null}>
            <Button type="primary" onClick={() => setPublishOpen(true)}>
              {tc("identityActions.publish")}
            </Button>
          </PermissionGuard>
        )}
      </Space>
      <ResourceListCard
        title={tc("nav.actions.versions", "Versions")}
        description={tc(
          "actions.versions.description",
          "Published versions of the selected action.",
        )}
        query={versionsQuery}
        rowKey="id"
        hideSearch
        columns={[
          {
            title: tc("identityActions.version"),
            dataIndex: "version",
            key: "version",
            render: (v: number) => `v${v}`,
          },
          {
            title: tc("identityActions.code"),
            dataIndex: "code",
            key: "code",
            render: (v?: string) =>
              v ? <Tag>{v.length > 40 ? `${v.slice(0, 40)}…` : v}</Tag> : "-",
          },
          {
            title: tc("createdAt", "Created"),
            dataIndex: "createdAt",
            key: "createdAt",
            render: (v: string) => new Date(v).toLocaleString(),
          },
        ]}
      />

      <Modal
        open={publishOpen}
        title={tc("identityActions.publishTitle")}
        onCancel={() => setPublishOpen(false)}
        onOk={() => {
          void form.validateFields().then((values) => {
            if (!selectedActionId) return undefined;
            publishMutation.mutate({ id: selectedActionId, code: values.code });
            return undefined;
          });
        }}
        confirmLoading={publishMutation.isPending}
      >
        <Form form={form} layout="vertical">
          <Form.Item label={tc("identityActions.code")} name="code" rules={[{ required: true }]}>
            <Input.TextArea rows={8} placeholder={tc("identityActions.codePlaceholder")} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
