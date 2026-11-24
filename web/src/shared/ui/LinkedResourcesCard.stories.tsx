import type { Meta, StoryObj } from "@storybook/react";
import { LinkedResourcesCard } from "./LinkedResourcesCard";
import { Tag } from "antd";
import type { ColumnsType } from "antd/es/table";

/**
 * LinkedResourcesCard component for displaying linked resource tables.
 *
 * Features:
 * - Card layout with title and description
 * - Loading skeleton state
 * - Empty state with simple illustration
 * - Compact table without pagination
 */
type LinkedItem = {
  id: string;
  name: string;
  type: string;
  status: string;
};

const columns: ColumnsType<LinkedItem> = [
  { title: "Name", dataIndex: "name", key: "name" },
  { title: "Type", dataIndex: "type", key: "type" },
  {
    title: "Status",
    dataIndex: "status",
    key: "status",
    render: (status: string) => (
      <Tag color={status === "Active" ? "green" : "default"}>{status}</Tag>
    ),
  },
];

const sampleItems: LinkedItem[] = [
  { id: "1", name: "Main Application", type: "Application", status: "Active" },
  { id: "2", name: "Admin Group", type: "Group", status: "Active" },
  { id: "3", name: "Viewer Role", type: "Role", status: "Inactive" },
];

const meta: Meta<typeof LinkedResourcesCard> = {
  title: "Shared/UI/LinkedResourcesCard",
  component: LinkedResourcesCard,
  tags: ["autodocs"],
  argTypes: {
    title: {
      description: "Card title",
      control: { type: "text" },
    },
    description: {
      description: "Card description",
      control: { type: "text" },
    },
    loading: {
      description: "Whether the data is loading",
      control: { type: "boolean" },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default ──
export const Default: Story = {
  args: {
    title: "Linked Resources",
    description: "Resources associated with this item",
    items: sampleItems,
    columns,
    rowKey: "id",
  },
};

// ── Loading ──
export const Loading: Story = {
  args: {
    title: "Linked Resources",
    description: "Loading linked resources...",
    loading: true,
    items: [],
    columns,
    rowKey: "id",
  },
};

// ── Empty ──
export const Empty: Story = {
  args: {
    title: "Linked Resources",
    description: "No resources are currently linked",
    items: [],
    columns,
    rowKey: "id",
  },
};

// ── Single Item ──
export const SingleItem: Story = {
  args: {
    title: "Assigned Role",
    description: "The role assigned to this user",
    items: [sampleItems[0]],
    columns,
    rowKey: "id",
  },
};
