import type { Meta, StoryObj } from "@storybook/react";
import { ConditionalField } from "./ConditionalField";
import { Form, Input, Select } from "antd";

/**
 * ConditionalField component for conditionally rendering form fields.
 *
 * Features:
 * - Watches specified form field dependencies
 * - Renders children only when the `when` callback returns true
 * - Works seamlessly with antd Form.Item
 */
const meta: Meta<typeof ConditionalField> = {
  title: "Shared/UI/ConditionalField",
  component: ConditionalField,
  tags: ["autodocs"],
  argTypes: {
    dependencies: {
      description: "Array of form field names to watch",
      control: { type: "object" },
    },
    when: {
      description: "Callback that returns true to show children",
      table: { category: "Logic" },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default: Show field based on select value ──
export const Default: Story = {
  render: () => (
    <Form layout="vertical" initialValues={{ authType: "password" }}>
      <Form.Item name="authType" label="Authentication Type">
        <Select
          options={[
            { label: "Password", value: "password" },
            { label: "OAuth2", value: "oauth2" },
            { label: "SAML", value: "saml" },
          ]}
        />
      </Form.Item>
      <ConditionalField
        dependencies={["authType"]}
        when={(values) => values.authType === "password"}
      >
        <Form.Item name="password" label="Password">
          <Input.Password placeholder="Enter password" />
        </Form.Item>
      </ConditionalField>
      <ConditionalField dependencies={["authType"]} when={(values) => values.authType === "oauth2"}>
        <Form.Item name="clientId" label="Client ID">
          <Input placeholder="Enter OAuth2 Client ID" />
        </Form.Item>
      </ConditionalField>
      <ConditionalField dependencies={["authType"]} when={(values) => values.authType === "saml"}>
        <Form.Item name="metadataUrl" label="SAML Metadata URL">
          <Input placeholder="Enter SAML metadata URL" />
        </Form.Item>
      </ConditionalField>
    </Form>
  ),
};

// ── Multiple Dependencies ──
export const MultipleDependencies: Story = {
  render: () => (
    <Form layout="vertical" initialValues={{ enableMfa: false, mfaMethod: "sms" }}>
      <Form.Item name="enableMfa" label="Enable MFA" valuePropName="checked">
        <Select
          options={[
            { label: "Yes", value: true },
            { label: "No", value: false },
          ]}
        />
      </Form.Item>
      <ConditionalField dependencies={["enableMfa"]} when={(values) => values.enableMfa === true}>
        <Form.Item name="mfaMethod" label="MFA Method">
          <Select
            options={[
              { label: "SMS", value: "sms" },
              { label: "Authenticator App", value: "app" },
              { label: "Email", value: "email" },
            ]}
          />
        </Form.Item>
      </ConditionalField>
    </Form>
  ),
};

// ── Hidden by Default ──
export const HiddenByDefault: Story = {
  render: () => (
    <Form layout="vertical" initialValues={{ showAdvanced: false }}>
      <Form.Item name="showAdvanced" label="Show Advanced Options" valuePropName="checked">
        <Select
          options={[
            { label: "Yes", value: true },
            { label: "No", value: false },
          ]}
        />
      </Form.Item>
      <ConditionalField
        dependencies={["showAdvanced"]}
        when={(values) => values.showAdvanced === true}
      >
        <Form.Item name="timeout" label="Request Timeout (ms)">
          <Input type="number" placeholder="30000" />
        </Form.Item>
        <Form.Item name="retryCount" label="Max Retry Count">
          <Input type="number" placeholder="3" />
        </Form.Item>
      </ConditionalField>
    </Form>
  ),
};
