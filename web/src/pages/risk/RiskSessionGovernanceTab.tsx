import { Tag } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { listSessionGovernance } from "@/shared/api/iam/risk";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";
import { formatDateTime } from "@/shared/lib/date-time";

// Keys mirror the server's closed set (domain/risk.GovernanceActions). Unknown
// values still render through the `default` fallback (手册 4.14.3: response-side
// values are open to extension).
const actionColor: Record<string, string> = {
  monitor: "orange",
  step_up: "orange",
  revoke: "red",
};

export default function RiskSessionGovernanceTab() {
  const { t: tc } = useTranslation("common");

  const query = useQuery({
    queryKey: ["risk", "session-governance"],
    queryFn: listSessionGovernance,
    select: (data) => data?.items ?? [],
  });

  return (
    <ResourceListCard
      title={tc("risk.sessionGovernance", "Session Governance")}
      description={tc(
        "risk.governanceDescription",
        "Automatic governance actions applied to sessions due to risk signals.",
      )}
      query={query}
      rowKey="id"
      columns={[
        {
          title: tc("risk.sessionId", "Session ID"),
          dataIndex: "sessionId",
          key: "sessionId",
          render: (v?: string) => v || "-",
        },
        {
          title: tc("risk.action", "Action"),
          dataIndex: "action",
          key: "action",
          render: (v: string) => (
            <Tag color={actionColor[v] ?? "default"}>{v?.replace("_", " ").toUpperCase()}</Tag>
          ),
        },
        { title: tc("risk.score", "Score"), dataIndex: "riskScore", key: "riskScore" },
        {
          title: tc("risk.reason", "Reason"),
          dataIndex: "reason",
          key: "reason",
          render: (v?: string) => v || "-",
        },
        {
          title: tc("createdAt"),
          dataIndex: "createdAt",
          key: "createdAt",
          render: (v: string) => formatDateTime(v),
        },
      ]}
    />
  );
}
