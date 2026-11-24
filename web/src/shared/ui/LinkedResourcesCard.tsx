import { Card, Empty, Skeleton, Table, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import { memo } from "react";
import { useTranslation } from "react-i18next";

type LinkedResourcesCardProps<T extends Record<string, unknown>> = {
  title: string;
  description: string;
  loading?: boolean;
  items: T[];
  columns: ColumnsType<T>;
  rowKey: keyof T | ((record: T) => string);
};

export const LinkedResourcesCard = memo(function LinkedResourcesCard<
  T extends Record<string, unknown>,
>({ title, description, loading, items, columns, rowKey }: LinkedResourcesCardProps<T>) {
  const { t } = useTranslation("common");
  return (
    <Card className="gl-panel gl-resource-card" variant="borderless">
      <div className="gl-resource-card__header">
        <div>
          <Typography.Title level={3}>{title}</Typography.Title>
          <Typography.Paragraph>{description}</Typography.Paragraph>
        </div>
      </div>

      {loading ? (
        <Skeleton active paragraph={{ rows: 4 }} />
      ) : items.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("noLinkedData")} />
      ) : (
        <Table<T>
          size="small"
          rowKey={rowKey as never}
          dataSource={items}
          columns={columns}
          pagination={false}
          scroll={{ x: 720 }}
        />
      )}
    </Card>
  );
}) as <T extends Record<string, unknown>>(props: LinkedResourcesCardProps<T>) => React.JSX.Element;
