import { Card, Empty, Skeleton, Typography } from "antd";
import {
  ResponsiveContainer,
  BarChart as RechartsBarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
} from "recharts";

type BarChartProps = {
  title: string;
  data: Array<Record<string, unknown>>;
  xKey: string;
  yKeys: Array<{ key: string; color: string; name?: string }>;
  loading?: boolean;
  stacked?: boolean;
};

export function BarChart({ title, data, xKey, yKeys, loading, stacked }: BarChartProps) {
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
            <RechartsBarChart data={data}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--gl-color-border, #eee)" />
              <XAxis
                dataKey={xKey}
                tick={{ fontSize: 12 }}
                stroke="var(--gl-color-text-tertiary, #999)"
              />
              <YAxis tick={{ fontSize: 12 }} stroke="var(--gl-color-text-tertiary, #999)" />
              <Tooltip />
              {yKeys.map((yk) => (
                <Bar
                  key={yk.key}
                  dataKey={yk.key}
                  fill={yk.color}
                  name={yk.name || yk.key}
                  stackId={stacked ? "stack" : undefined}
                />
              ))}
            </RechartsBarChart>
          </ResponsiveContainer>
        )}
      </div>
    </Card>
  );
}
