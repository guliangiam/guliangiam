import { Space, Tag } from "antd";
import type { WorkloadResourceStatus } from "@/shared/api/iam/identity-pools";

export function StringTags({ values }: { values: string[] }) {
  if (values.length === 0) return <>-</>;
  return (
    <Space size={[4, 4]} wrap>
      {values.map((value) => (
        <Tag key={value}>{value}</Tag>
      ))}
    </Space>
  );
}

export function WorkloadStatusTag({ status }: { status: WorkloadResourceStatus }) {
  const color = status === "active" ? "green" : status === "deleted" ? "red" : "default";
  return <Tag color={color}>{status}</Tag>;
}
