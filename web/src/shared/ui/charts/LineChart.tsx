import { Card, Empty, Skeleton, Typography } from "antd";
import {
  ResponsiveContainer,
  LineChart as RechartsLineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
} from "recharts";

type LineConfig = {
  key: string;
  color: string;
  name: string;
};

type LineChartProps = {
  title: string;
  data: Array<Record<string, unknown>>;
  xKey: string;
  /** Single line mode: yKey + color */
  yKey?: string;
  color?: string;
  /** Multi-line mode: lines array */
  lines?: LineConfig[];
  loading?: boolean;
};

export function LineChart({
  title,
  data,
  xKey,
  yKey,
  color = "var(--gl-color-primary)",
  lines,
  loading,
}: LineChartProps) {
  const lineConfigs = lines ?? (yKey ? [{ key: yKey, color, name: yKey }] : []);

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
            <RechartsLineChart data={data}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--gl-color-border, #eee)" />
              <XAxis
                dataKey={xKey}
                tick={{ fontSize: 12 }}
                stroke="var(--gl-color-text-tertiary, #999)"
              />
              <YAxis tick={{ fontSize: 12 }} stroke="var(--gl-color-text-tertiary, #999)" />
              <Tooltip />
              {lineConfigs.length > 1 && <Legend />}
              {lineConfigs.map((line) => (
                <Line
                  key={line.key}
                  type="monotone"
                  dataKey={line.key}
                  stroke={line.color}
                  strokeWidth={2}
                  dot={false}
                  name={line.name}
                />
              ))}
            </RechartsLineChart>
          </ResponsiveContainer>
        )}
      </div>
    </Card>
  );
}
