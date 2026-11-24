import { Select, Space, Tag } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  listActions,
  listActionExecutionLogs,
  type ActionExecutionLogItem,
} from "@/shared/api/iam/actions";
import type { IamListResponse } from "@/shared/api/iam/client";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";

const logStatusColor: Record<string, string> = {
  running: "blue",
  success: "green",
  failed: "red",
  cancelled: "default",
  timeout: "orange",
};

export default function ActionLogsTab() {
  const { t: tc } = useTranslation("common");
  const [selectedActionId, setSelectedActionId] = useState<string | undefined>(undefined);

  const actionsQuery = useQuery({
    queryKey: ["actions", "list"],
    queryFn: listActions,
    select: (data) => data?.items ?? [],
  });

  const logsQuery = useQuery({
    queryKey: ["actions", selectedActionId, "execution-logs"],
    queryFn: (): Promise<IamListResponse<ActionExecutionLogItem> | undefined> =>
      selectedActionId
        ? listActionExecutionLogs(selectedActionId)
        : Promise.resolve({ items: [], page: { page: 1, pageSize: 10, total: 0 } }),
    select: (data) => data?.items ?? [],
    enabled: !!selectedActionId,
  });

  const actionItems: { id: string; name: string }[] = actionsQuery.data ?? [];

  return (
    <>
      <Space style={{ marginBottom: 16 }}>
        <span>{tc("actions.selectAction", "Select action")}:</span>
        <Select
          style={{ minWidth: 240 }}
          allowClear
          placeholder={tc("actions.selectActionPlaceholder", "Choose an action…")}
          value={selectedActionId}
          onChange={setSelectedActionId}
          options={actionItems.map((a) => ({ value: a.id, label: a.name }))}
        />
      </Space>
      <ResourceListCard
        title={tc("nav.actions.executionLogs", "Execution Logs")}
        description={tc(
          "actions.executionLogs.description",
          "Execution history and results for the selected action.",
        )}
        query={logsQuery}
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
            title: tc("status"),
            dataIndex: "status",
            key: "status",
            render: (v: string) => <Tag color={logStatusColor[v] ?? "default"}>{v}</Tag>,
          },
          {
            title: tc("identityActions.error"),
            dataIndex: "error",
            key: "error",
            render: (v?: string) => v || "-",
          },
          {
            title: tc("createdAt", "Created"),
            dataIndex: "createdAt",
            key: "createdAt",
            render: (v: string) => new Date(v).toLocaleString(),
          },
        ]}
      />
    </>
  );
}
