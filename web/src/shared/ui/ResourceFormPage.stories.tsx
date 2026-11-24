import type { Meta, StoryObj } from "@storybook/react";
import { ResourceFormPage } from "./ResourceFormPage";
import { Form, Input, Select, Switch } from "antd";

/**
 * ResourceFormPage component provides a standard layout for resource edit/create pages.
 *
 * Features:
 * - Card layout with title, description, and action buttons
 * - Loading skeleton state
 * - Error alert banner
 * - Sticky form footer with save/reset
 * - Save & Exit button option
 * - Double-submit prevention
 */
const meta: Meta<typeof ResourceFormPage> = {
  title: "Shared/UI/ResourceFormPage",
  component: ResourceFormPage,
  tags: ["autodocs"],
  argTypes: {
    title: {
      description: "Page title",
      control: { type: "text" },
    },
    description: {
      description: "Page description",
      control: { type: "text" },
    },
    loading: {
      description: "Whether the page is in loading state",
      control: { type: "boolean" },
    },
    saving: {
      description: "Whether the form is being saved",
      control: { type: "boolean" },
    },
    error: {
      description: "Error message to display",
      control: { type: "text" },
    },
    showSaveAndExit: {
      description: "Show 'Save & Exit' button",
      control: { type: "boolean" },
    },
  },
  args: {
    onCancel: () => {},
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default ──
export const Default: Story = {
  render: () => {
    const [form] = Form.useForm();
    return (
      <ResourceFormPage
        title="Edit Application"
        description="Configure application settings and permissions"
        form={form}
        onFinish={(values) => console.warn("Submit:", values)}
        onCancel={() => console.warn("Cancel")}
      >
        <Form.Item name="name" label="Application Name" rules={[{ required: true }]}>
          <Input placeholder="Enter application name" />
        </Form.Item>
        <Form.Item name="type" label="Application Type">
          <Select
            options={[
              { label: "Web Application", value: "web" },
              { label: "Native Application", value: "native" },
              { label: "SPA", value: "spa" },
            ]}
          />
        </Form.Item>
        <Form.Item name="enabled" label="Enabled" valuePropName="checked">
          <Switch />
        </Form.Item>
      </ResourceFormPage>
    );
  },
};

// ── Loading State ──
export const Loading: Story = {
  args: {
    title: "Edit Application",
    description: "Loading application data...",
    loading: true,
  },
};

// ── With Error ──
export const WithError: Story = {
  render: () => {
    const [form] = Form.useForm();
    return (
      <ResourceFormPage
        title="Edit Application"
        description="Configure application settings"
        form={form}
        error="Failed to save: Network connection error. Please try again."
        onFinish={(values) => console.warn("Submit:", values)}
        onCancel={() => console.warn("Cancel")}
      >
        <Form.Item name="name" label="Application Name">
          <Input placeholder="Enter application name" />
        </Form.Item>
      </ResourceFormPage>
    );
  },
};

// ── Saving State ──
export const Saving: Story = {
  render: () => {
    const [form] = Form.useForm();
    return (
      <ResourceFormPage
        title="Edit Application"
        description="Configure application settings"
        form={form}
        saving
        onFinish={() => {}}
        onCancel={() => {}}
      >
        <Form.Item name="name" label="Application Name">
          <Input placeholder="Enter application name" />
        </Form.Item>
      </ResourceFormPage>
    );
  },
};

// ── With Save & Exit ──
export const WithSaveAndExit: Story = {
  render: () => {
    const [form] = Form.useForm();
    return (
      <ResourceFormPage
        title="Create Provider"
        description="Set up a new identity provider"
        form={form}
        showSaveAndExit
        onFinish={(values) => console.warn("Submit:", values)}
        onCancel={() => console.warn("Cancel")}
      >
        <Form.Item name="name" label="Provider Name" rules={[{ required: true }]}>
          <Input placeholder="Enter provider name" />
        </Form.Item>
        <Form.Item name="category" label="Category">
          <Select
            options={[
              { label: "OAuth2", value: "OAuth2" },
              { label: "OIDC", value: "OIDC" },
              { label: "SAML", value: "SAML" },
              { label: "LDAP", value: "LDAP" },
            ]}
          />
        </Form.Item>
      </ResourceFormPage>
    );
  },
};
