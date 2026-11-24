import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { listRiskSignals } from "@/shared/api/iam/risk";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";
import { formatDateTime } from "@/shared/lib/date-time";

export default function RiskSignalsTab() {
  const { t: tc } = useTranslation("common");

  const query = useQuery({
    queryKey: ["risk", "signals"],
    queryFn: listRiskSignals,
    select: (data) => data?.items ?? [],
  });

  return (
    <ResourceListCard
      title={tc("risk.signals", "Risk Signals")}
      description={tc(
        "risk.signalsDescription",
        "Real-time risk signals from adaptive authentication engines.",
      )}
      query={query}
      rowKey="id"
      columns={[
        { title: tc("risk.signalType", "Signal Type"), dataIndex: "signalType", key: "signalType" },
        {
          title: tc("risk.signalValue", "Signal Value"),
          dataIndex: "signalValue",
          key: "signalValue",
          ellipsis: true,
          render: (v: unknown) => (v === null || v === undefined ? "-" : JSON.stringify(v)),
        },
        {
          title: tc("risk.ip", "IP"),
          dataIndex: "ip",
          key: "ip",
          render: (v?: string) => v || "-",
        },
        {
          title: tc("user"),
          dataIndex: "userId",
          key: "userId",
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
