import { Card, Empty, Skeleton, Typography } from "antd";
import {
  ResponsiveContainer,
  PieChart as RechartsPieChart,
  Pie,
  Cell,
  Tooltip,
  Legend,
} from "recharts";

type PieChartProps = {
  title: string;
  data: Array<{ name: string; value: number }>;
  colors?: string[];
  loading?: boolean;
};

/** 系列配色全部取语义令牌（§28.1），随主题切换，不在组件内硬编码色值。 */
const DEFAULT_COLORS = [
  "var(--gl-color-primary)",
  "var(--gl-color-success)",
  "var(--gl-color-warning)",
  "var(--gl-color-error)",
  "var(--gl-color-text-tertiary)",
  "color-mix(in srgb, var(--gl-color-primary) 55%, var(--gl-color-bg-elevated))",
  "color-mix(in srgb, var(--gl-color-success) 55%, var(--gl-color-bg-elevated))",
  "color-mix(in srgb, var(--gl-color-warning) 55%, var(--gl-color-bg-elevated))",
];

export function PieChart({ title, data, colors = DEFAULT_COLORS, loading }: PieChartProps) {
  return (
    <Card
      className="gl-panel"
      variant="borderless"
      title={<Typography.Text strong>{title}</Typography.Text>}
    >
      <div className="gl-chart">
        {loading ? (
          <Skeleton active paragraph={{ rows: 6 }} title={false} />
        ) : !data || data.length === 0 ? (
          <Empty description="No data" />
        ) : (
          <ResponsiveContainer width="100%" height={300}>
            <RechartsPieChart>
              <Pie
                data={data}
                cx="50%"
                cy="50%"
                innerRadius={60}
                outerRadius={100}
                dataKey="value"
                nameKey="name"
                label={({ name, percent }: { name?: string; percent?: number }) =>
                  `${name ?? ""} ${((percent ?? 0) * 100).toFixed(0)}%`
                }
                labelLine={false}
              >
                {data.map((_, index) => (
                  <Cell key={`cell-${index}`} fill={colors[index % colors.length]} />
                ))}
              </Pie>
              <Tooltip />
              <Legend />
            </RechartsPieChart>
          </ResponsiveContainer>
        )}
      </div>
    </Card>
  );
}
