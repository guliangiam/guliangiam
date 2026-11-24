import type { Meta, StoryObj } from "@storybook/react";
import { PermissionGuard } from "./PermissionGuard";
import { Button, Card, Space, Typography } from "antd";

/**
 * PermissionGuard component for conditionally rendering content based on user permissions.
 *
 * Features:
 * - Checks action-based permissions (e.g. "edit", "delete")
 * - Checks resource-based permissions (e.g. "users", "settings")
 * - Shows 403 Result when permission is denied
 * - Supports custom fallback content
 * - Shows loading skeleton while permissions are being resolved
 *
 * Note: In Storybook, the `usePermissions` hook returns default values.
 * The stories demonstrate the component structure and fallback behavior.
 */
const meta: Meta<typeof PermissionGuard> = {
  title: "Shared/UI/PermissionGuard",
  component: PermissionGuard,
  tags: ["autodocs"],
  argTypes: {
    action: {
      description: "Required action permission (e.g. 'edit', 'delete')",
      control: { type: "text" },
    },
    resource: {
      description: "Required resource permission (e.g. 'users', 'settings')",
      control: { type: "text" },
    },
    fallback: {
      description: "Custom fallback content when permission is denied",
      table: { category: "Content" },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default (with children rendered) ──
export const Default: Story = {
  args: {
    children: (
      <Card>
        <Typography.Text>This content is visible when permission is granted.</Typography.Text>
      </Card>
    ),
  },
};

// ── With Action Check ──
export const WithActionCheck: Story = {
  args: {
    action: "edit",
    children: <Button type="primary">Edit Resource</Button>,
  },
};

// ── With Resource Check ──
export const WithResourceCheck: Story = {
  args: {
    resource: "admin-settings",
    children: (
      <Card>
        <Typography.Title level={4}>Admin Settings</Typography.Title>
        <Typography.Text>
          Only visible to users with admin-settings resource permission.
        </Typography.Text>
      </Card>
    ),
  },
};

// ── With Custom Fallback ──
export const WithCustomFallback: Story = {
  args: {
    action: "delete",
    fallback: (
      <Typography.Text type="secondary" style={{ fontStyle: "italic" }}>
        You do not have permission to delete this resource.
      </Typography.Text>
    ),
    children: <Button danger>Delete Resource</Button>,
  },
};

// ── Complex Content Guard ──
export const ComplexContent: Story = {
  args: {
    action: "manage-users",
    children: (
      <Space orientation="vertical" style={{ width: "100%" }}>
        <Typography.Title level={4}>User Management</Typography.Title>
        <Space>
          <Button type="primary">Create User</Button>
          <Button>Import Users</Button>
          <Button danger>Bulk Delete</Button>
        </Space>
      </Space>
    ),
  },
};

// ── Nested Guards ──
export const NestedGuards: Story = {
  render: () => (
    <Space orientation="vertical" style={{ width: "100%" }}>
      <PermissionGuard action="view-dashboard">
        <Card>
          <Typography.Text>Dashboard content (view-dashboard)</Typography.Text>
        </Card>
      </PermissionGuard>
      <PermissionGuard action="edit-settings">
        <Card>
          <Typography.Text>Settings content (edit-settings)</Typography.Text>
        </Card>
      </PermissionGuard>
    </Space>
  ),
};
