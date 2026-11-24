import type { Meta, StoryObj } from "@storybook/react";
import { ResourceListCard } from "./ResourceListCard";
import { Button, Tag } from "antd";
import type { ColumnsType } from "antd/es/table";

/**
 * ResourceListCard component for displaying resource lists with search, filter, and pagination.
 *
 * Features:
 * - Card layout with title, description, and toolbar
 * - Client-side search with deferred value
 * - Column-specific search dropdowns
 * - Filter dropdowns for structured filtering
 * - Row selection for bulk operations
 * - Server-side pagination support
 * - Virtual table for large datasets (>200 rows)
 * - Loading skeleton and empty state
 */
type UserItem = {
  id: string;
  name: string;
  email: string;
  status: string;
  role: string;
};

const columns: ColumnsType<UserItem> = [
  { title: "Name", dataIndex: "name", key: "name", sorter: true },
  { title: "Email", dataIndex: "email", key: "email" },
  {
    title: "Status",
    dataIndex: "status",
    key: "status",
    render: (status: string) => {
      const color = status === "Active" ? "green" : status === "Inactive" ? "default" : "red";
      return <Tag color={color}>{status}</Tag>;
    },
  },
  { title: "Role", dataIndex: "role", key: "role" },
];

const sampleData: UserItem[] = [
  { id: "1", name: "Alice Johnson", email: "alice@example.com", status: "Active", role: "Admin" },
  { id: "2", name: "Bob Smith", email: "bob@example.com", status: "Active", role: "User" },
  {
    id: "3",
    name: "Charlie Brown",
    email: "charlie@example.com",
    status: "Inactive",
    role: "User",
  },
  { id: "4", name: "Diana Prince", email: "diana@example.com", status: "Active", role: "Editor" },
  {
    id: "5",
    name: "Edward Norton",
    email: "edward@example.com",
    status: "Suspended",
    role: "User",
  },
];

const meta: Meta<typeof ResourceListCard> = {
  title: "Shared/UI/ResourceListCard",
  component: ResourceListCard,
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
    searchPlaceholder: {
      description: "Search input placeholder",
      control: { type: "text" },
    },
    showRefresh: {
      description: "Show refresh button in toolbar",
      control: { type: "boolean" },
      table: { defaultValue: { summary: "true" } },
    },
    badge: {
      description: "Badge count shown next to title",
      control: { type: "number" },
    },
    hideSearch: {
      description: "Hide search input",
      control: { type: "boolean" },
      table: { defaultValue: { summary: "false" } },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default ──
export const Default: Story = {
  args: {
    title: "Users",
    description: "Manage user accounts and permissions",
    query: {
      data: sampleData,
      isLoading: false,
      isFetching: false,
      refetch: () => Promise.resolve({} as never),
    } as never,
    columns,
    rowKey: "id",
  },
};

// ── Loading State ──
export const Loading: Story = {
  args: {
    title: "Users",
    description: "Loading user data...",
    query: {
      data: [],
      isLoading: true,
      isFetching: false,
      refetch: () => Promise.resolve({} as never),
    } as never,
    columns,
    rowKey: "id",
  },
};

// ── Empty State ──
export const Empty: Story = {
  args: {
    title: "Users",
    description: "No users found",
    query: {
      data: [],
      isLoading: false,
      isFetching: false,
      refetch: () => Promise.resolve({} as never),
    } as never,
    columns,
    rowKey: "id",
  },
};

// ── With Badge ──
export const WithBadge: Story = {
  args: {
    title: "Applications",
    description: "Registered applications",
    badge: 12,
    query: {
      data: sampleData.slice(0, 3),
      isLoading: false,
      isFetching: false,
      refetch: () => Promise.resolve({} as never),
    } as never,
    columns,
    rowKey: "id",
  },
};

// ── With Toolbar ──
export const WithToolbar: Story = {
  args: {
    title: "Providers",
    description: "Identity providers configuration",
    toolbar: <Button type="primary">Add Provider</Button>,
    query: {
      data: sampleData.slice(0, 3),
      isLoading: false,
      isFetching: false,
      refetch: () => Promise.resolve({} as never),
    } as never,
    columns,
    rowKey: "id",
  },
};

// ── Without Search ──
export const WithoutSearch: Story = {
  args: {
    title: "Sessions",
    description: "Active sessions",
    hideSearch: true,
    query: {
      data: sampleData.slice(0, 4),
      isLoading: false,
      isFetching: false,
      refetch: () => Promise.resolve({} as never),
    } as never,
    columns,
    rowKey: "id",
  },
};
