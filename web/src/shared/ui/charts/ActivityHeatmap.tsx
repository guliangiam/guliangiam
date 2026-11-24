import { Card, Empty, Skeleton, Typography } from "antd";

type ActivityHeatmapPoint = {
  date: string;
  count: number;
};

type ActivityHeatmapProps = {
  title: string;
  data: ActivityHeatmapPoint[];
  loading?: boolean;
};

const CELL_SIZE = 12;
const CELL_GAP = 3;
const WEEKS = 53;
const DAY_NAMES = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const MONTH_NAMES = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
];

function getIntensity(count: number, max: number): number {
  if (count === 0) return 0;
  if (max === 0) return 0;
  const ratio = count / max;
  if (ratio <= 0.25) return 1;
  if (ratio <= 0.5) return 2;
  if (ratio <= 0.75) return 3;
  return 4;
}

// 空单元格用中性边框色；1–4 级由品牌主色与卡片底色混合，深浅随主题切换。
const INTENSITY_COLORS = [
  "var(--gl-color-border)",
  "color-mix(in srgb, var(--gl-color-primary) 20%, var(--gl-color-bg-elevated))",
  "color-mix(in srgb, var(--gl-color-primary) 45%, var(--gl-color-bg-elevated))",
  "color-mix(in srgb, var(--gl-color-primary) 70%, var(--gl-color-bg-elevated))",
  "var(--gl-color-primary)",
];

function formatDate(dateStr: string): string {
  const d = new Date(dateStr + "T00:00:00");
  return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

export function ActivityHeatmap({ title, data, loading }: ActivityHeatmapProps) {
  if (loading) {
    return (
      <Card
        className="gl-panel"
        variant="borderless"
        title={<Typography.Text strong>{title}</Typography.Text>}
      >
        <Skeleton active paragraph={{ rows: 4 }} title={false} />
      </Card>
    );
  }

  if (!data || data.length === 0) {
    return (
      <Card
        className="gl-panel"
        variant="borderless"
        title={<Typography.Text strong>{title}</Typography.Text>}
      >
        <Empty description="No data" />
      </Card>
    );
  }

  // Build a map of date -> count
  const countMap = new Map<string, number>();
  let maxCount = 0;
  for (const point of data) {
    countMap.set(point.date, point.count);
    if (point.count > maxCount) maxCount = point.count;
  }

  // Determine start date: the Sunday on or before 364 days ago
  // oxlint-disable-next-line react/purity — 展示型组件以「渲染当刻」为刻度基准。
  const today = new Date();
  const startDate = new Date(today);
  startDate.setDate(startDate.getDate() - 364);
  // Adjust to previous Sunday
  startDate.setDate(startDate.getDate() - startDate.getDay());

  // Build grid: array of weeks, each week is 7 days
  const weeks: Array<Array<{ date: string; count: number; intensity: number }>> = [];
  let currentWeek: Array<{ date: string; count: number; intensity: number }> = [];

  // 以 UTC 日历日数算出「startDate..today」的闭区间天数，避免本地时区 DST 让差值偏移一天。
  const utcDayNumber = (d: Date) => Date.UTC(d.getFullYear(), d.getMonth(), d.getDate());
  const dayCount = Math.round((utcDayNumber(today) - utcDayNumber(startDate)) / 86_400_000) + 1;

  for (let dayOffset = 0; dayOffset < dayCount; dayOffset++) {
    const cursor = new Date(startDate);
    cursor.setDate(startDate.getDate() + dayOffset);
    const dateStr = cursor.toISOString().slice(0, 10);
    const count = countMap.get(dateStr) ?? 0;
    const intensity = getIntensity(count, maxCount);
    currentWeek.push({ date: dateStr, count, intensity });

    if (cursor.getDay() === 6) {
      weeks.push(currentWeek);
      currentWeek = [];
    }
  }
  if (currentWeek.length > 0) {
    // Pad remaining days
    while (currentWeek.length < 7) {
      currentWeek.push({ date: "", count: 0, intensity: 0 });
    }
    weeks.push(currentWeek);
  }

  // Calculate month labels
  const monthLabels: Array<{ label: string; x: number }> = [];
  let lastMonth = -1;
  for (let w = 0; w < weeks.length; w++) {
    // Find the first non-empty day in the week
    const firstDay = weeks[w].find((d) => d.date !== "");
    if (firstDay) {
      const month = new Date(firstDay.date + "T00:00:00").getMonth();
      if (month !== lastMonth) {
        monthLabels.push({ label: MONTH_NAMES[month], x: w * (CELL_SIZE + CELL_GAP) });
        lastMonth = month;
      }
    }
  }

  const svgWidth = WEEKS * (CELL_SIZE + CELL_GAP) + 36;
  const svgHeight = 7 * (CELL_SIZE + CELL_GAP) + 24;

  const totalLogins = data.reduce((sum, d) => sum + d.count, 0);

  return (
    <Card
      className="gl-panel"
      variant="borderless"
      title={<Typography.Text strong>{title}</Typography.Text>}
      extra={
        <Typography.Text type="secondary">
          {totalLogins.toLocaleString()} logins in the last year
        </Typography.Text>
      }
    >
      <div style={{ overflowX: "auto" }}>
        {/* WCAG 1.1.1 Non-text Content — SVG needs role="img" and aria-label (A-01) */}
        <svg width={svgWidth} height={svgHeight} role="img" aria-label="Activity heatmap">
          <title>Activity heatmap for the past year</title>
          {/* Month labels */}
          {monthLabels.map((m, i) => (
            <text key={i} x={m.x + 36} y={10} fontSize={10} fill="var(--gl-color-text-tertiary)">
              {m.label}
            </text>
          ))}

          {/* Day labels */}
          {DAY_NAMES.map((name, i) => (
            <text
              key={name}
              x={0}
              y={20 + i * (CELL_SIZE + CELL_GAP) + CELL_SIZE / 2 + 4}
              fontSize={9}
              fill="var(--gl-color-text-tertiary)"
            >
              {i % 2 === 1 ? name : ""}
            </text>
          ))}

          {/* Heatmap cells */}
          {weeks.map((week, wIdx) =>
            week.map((day, dIdx) => {
              if (day.date === "") return null;
              const x = 36 + wIdx * (CELL_SIZE + CELL_GAP);
              const y = 18 + dIdx * (CELL_SIZE + CELL_GAP);
              return (
                <rect
                  key={day.date}
                  x={x}
                  y={y}
                  width={CELL_SIZE}
                  height={CELL_SIZE}
                  rx={2}
                  ry={2}
                  fill={INTENSITY_COLORS[day.intensity]}
                  style={{ cursor: "pointer" }}
                >
                  <title>{`${formatDate(day.date)}: ${day.count} logins`}</title>
                </rect>
              );
            }),
          )}
        </svg>

        {/* Legend */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "var(--gl-space-1)",
            marginTop: "var(--gl-space-2)",
            justifyContent: "flex-end",
          }}
        >
          <Typography.Text type="secondary" style={{ fontSize: "var(--gl-font-size-xs)" }}>
            Less
          </Typography.Text>
          {INTENSITY_COLORS.map((color, i) => (
            <span key={i} style={{ display: "inline-block" }}>
              <svg width={CELL_SIZE} height={CELL_SIZE} aria-hidden="true">
                <rect width={CELL_SIZE} height={CELL_SIZE} rx={2} fill={color} />
              </svg>
            </span>
          ))}
          <Typography.Text type="secondary" style={{ fontSize: "var(--gl-font-size-xs)" }}>
            More
          </Typography.Text>
        </div>
      </div>
    </Card>
  );
}
