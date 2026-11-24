import { Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { MeteringEventItem } from "@/shared/api/iam/metering";

const EVENT_TYPE_COLORS: Record<string, string> = {
  "ai.usage.tokens": "blue",
  "ai.usage.cost": "gold",
};

export const EVENT_TYPE_OPTIONS = [
  { value: "ai.usage.tokens", label: "ai.usage.tokens" },
  { value: "ai.usage.cost", label: "ai.usage.cost" },
];

export type MeteringEventFilters = {
  eventType?: string;
  subject?: string;
  timeRange?: [string, string]; // [from ISO, to ISO]
};

export type MeteringEventColumnLabels = {
  eventType: string;
  subject: string;
  quantity: string;
  occurredAt: string;
  createdAt: string;
  metadata: string;
};

/** Simple ISO → local readable. Kept local — matches pattern in other pages. */
export function formatDateTime(iso?: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

export function buildMeteringEventColumns(
  labels: MeteringEventColumnLabels,
): ColumnsType<MeteringEventItem> {
  return [
    {
      title: labels.eventType,
      dataIndex: "eventType",
      key: "eventType",
      width: 180,
      render: (v: string) => <Tag color={EVENT_TYPE_COLORS[v] ?? "default"}>{v}</Tag>,
    },
    {
      title: labels.subject,
      dataIndex: "subject",
      key: "subject",
      width: 220,
      ellipsis: true,
      render: (v: string) => <code style={{ fontSize: "var(--gl-font-size-xs)" }}>{v || "—"}</code>,
    },
    {
      title: labels.quantity,
      dataIndex: "quantity",
      key: "quantity",
      width: 140,
      align: "right",
      render: (v: number, row) => (
        <span style={{ fontVariantNumeric: "tabular-nums", fontWeight: 500 }}>
          {Number(v).toLocaleString()}
          {row.unit && (
            <span
              style={{
                color: "var(--gl-color-text-tertiary)",
                marginLeft: "var(--gl-space-1)",
                fontSize: "var(--gl-font-size-xs)",
              }}
            >
              {row.unit}
            </span>
          )}
        </span>
      ),
    },
    {
      title: labels.occurredAt,
      dataIndex: "occurredAt",
      key: "occurredAt",
      width: 180,
      render: (v: string) => formatDateTime(v),
    },
    {
      title: labels.createdAt,
      dataIndex: "createdAt",
      key: "createdAt",
      width: 180,
      render: (v: string) => formatDateTime(v),
    },
    {
      title: labels.metadata,
      dataIndex: "metadata",
      key: "metadata",
      ellipsis: true,
      render: (v: Record<string, unknown> | null) => {
        if (!v) return "—";
        const json = JSON.stringify(v);
        return (
          <span
            style={{
              fontSize: "var(--gl-font-size-xs)",
              color: "var(--gl-color-text-secondary)",
            }}
          >
            {json}
          </span>
        );
      },
    },
  ];
}
