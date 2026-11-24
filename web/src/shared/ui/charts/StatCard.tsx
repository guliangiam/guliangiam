import { Card, Skeleton } from "antd";
import type { ReactNode } from "react";

type StatCardProps = {
  title: string;
  value: number | string;
  delta?: string;
  icon?: ReactNode;
  loading?: boolean;
  onClick?: () => void;
};

export function StatCard({ title, value, delta, icon, loading, onClick }: StatCardProps) {
  const interactive = Boolean(onClick);
  return (
    <Card
      className={`gl-panel gl-metric-card${interactive ? " gl-metric-card--clickable" : ""}`}
      variant="borderless"
      onClick={onClick}
    >
      <div className="gl-metric-card__header">
        {icon && <span className="gl-metric-card__icon">{icon}</span>}
        <div className="gl-metric-card__label">{title}</div>
      </div>
      <div className="gl-metric-card__body">
        {loading ? (
          <Skeleton active paragraph={{ rows: 1 }} title={false} />
        ) : (
          <div className="gl-metric-card__value">{value}</div>
        )}
      </div>
      {delta && <div className="gl-metric-card__delta">{delta}</div>}
    </Card>
  );
}
