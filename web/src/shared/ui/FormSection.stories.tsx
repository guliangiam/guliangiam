import type { Meta, StoryObj } from "@storybook/react";
import { FormSection } from "./FormSection";
import { Input, Form, Switch, Select } from "antd";

/**
 * FormSection component for displaying form content in logical sections.
 *
 * Features:
 * - Supports collapsible mode, click title area to expand/collapse
 * - Supports required mark and badge
 * - Supports extra action area
 * - Full WCAG accessibility support (keyboard, aria attributes)
 */
const meta: Meta<typeof FormSection> = {
  title: "Shared/UI/FormSection",
  component: FormSection,
  tags: ["autodocs"],
  argTypes: {
    title: {
      description: "Section title",
      control: { type: "text" },
    },
    description: {
      description: "Section description text",
      control: { type: "text" },
    },
    collapsible: {
      description: "Whether collapsible",
      control: { type: "boolean" },
      table: {
        defaultValue: { summary: "false" },
      },
    },
    defaultCollapsed: {
      description: "Whether collapsed by default (only effective when collapsible)",
      control: { type: "boolean" },
      table: {
        defaultValue: { summary: "false" },
      },
    },
    required: {
      description: "Whether to show required mark",
      control: { type: "boolean" },
    },
    badge: {
      description: "Badge number/text",
      control: { type: "text" },
    },
  },
  args: {
    children: (
      <Form layout="vertical">
        <Form.Item label="Name">
          <Input placeholder="Enter name" />
        </Form.Item>
        <Form.Item label="Description">
          <Input.TextArea placeholder="Enter description" rows={2} />
        </Form.Item>
      </Form>
    ),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default ──
export const Default: Story = {
  args: {
    title: "Basic Info",
    description: "Fill in the basic user information",
  },
};

// ── Required Section ──
export const Required: Story = {
  args: {
    title: "Security Settings",
    description: "Configure security-related parameters",
    required: true,
  },
};

// ── Collapsible Section ──
export const Collapsible: Story = {
  args: {
    title: "Advanced Options",
    description: "Click the title area to expand or collapse",
    collapsible: true,
    children: (
      <Form layout="vertical">
        <Form.Item label="Timeout (seconds)">
          <Input type="number" placeholder="300" />
        </Form.Item>
        <Form.Item label="Max Retry Count">
          <Input type="number" placeholder="3" />
        </Form.Item>
      </Form>
    ),
  },
};

// ── Default Collapsed ──
export const DefaultCollapsed: Story = {
  args: {
    title: "Debug Options",
    description: "Collapsed by default, click to expand",
    collapsible: true,
    defaultCollapsed: true,
    children: (
      <Form layout="vertical">
        <Form.Item label="Log Level">
          <Select
            options={[
              { label: "DEBUG", value: "debug" },
              { label: "INFO", value: "info" },
              { label: "WARN", value: "warn" },
            ]}
          />
        </Form.Item>
        <Form.Item label="Enable Verbose Logging">
          <Switch />
        </Form.Item>
      </Form>
    ),
  },
};

// ── With Badge ──
export const WithBadge: Story = {
  args: {
    title: "Permission Configuration",
    description: "Number of currently configured permissions",
    badge: 5,
    children: (
      <Form layout="vertical">
        <Form.Item label="Permission Scope">
          <Select
            mode="multiple"
            placeholder="Select permissions"
            options={[
              { label: "Read", value: "read" },
              { label: "Write", value: "write" },
              { label: "Delete", value: "delete" },
              { label: "Admin", value: "admin" },
              { label: "Audit", value: "audit" },
            ]}
          />
        </Form.Item>
      </Form>
    ),
  },
};

// ── With Extra Action Area ──
export const WithExtra: Story = {
  args: {
    title: "Notification Settings",
    description: "Configure notification receiving methods",
    extra: <Switch checkedChildren="On" unCheckedChildren="Off" defaultChecked />,
    children: (
      <Form layout="vertical">
        <Form.Item label="Notification Email">
          <Input placeholder="admin@example.com" />
        </Form.Item>
        <Form.Item label="Notification Frequency">
          <Select
            options={[
              { label: "Real-time", value: "realtime" },
              { label: "Daily Digest", value: "daily" },
              { label: "Weekly Digest", value: "weekly" },
            ]}
          />
        </Form.Item>
      </Form>
    ),
  },
};

// ── Title Only (no description) ──
export const TitleOnly: Story = {
  args: {
    title: "Simple Section",
  },
};
