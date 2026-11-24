import type { Meta, StoryObj } from "@storybook/react";
import { ProviderIcon } from "./ProviderIcon";

/**
 * ProviderIcon component renders a branded avatar icon for OAuth/SAML/Web3 providers.
 *
 * Features:
 * - SVG brand marks for well-known providers (GitHub, Google, etc.)
 * - Colored initials fallback for unknown providers
 * - Configurable size
 * - 60+ built-in provider brand configurations
 */
const meta: Meta<typeof ProviderIcon> = {
  title: "Shared/UI/ProviderIcon",
  component: ProviderIcon,
  tags: ["autodocs"],
  argTypes: {
    type: {
      description: "Provider type name (e.g. GitHub, Google, SAML)",
      control: { type: "text" },
    },
    size: {
      description: "Avatar size in pixels",
      control: { type: "number" },
      table: { defaultValue: { summary: "32" } },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default (GitHub with SVG icon) ──
export const Default: Story = {
  args: {
    type: "GitHub",
    size: 32,
  },
};

// ── Google ──
export const Google: Story = {
  args: {
    type: "Google",
    size: 32,
  },
};

// ── WeChat ──
export const WeChat: Story = {
  args: {
    type: "WeChat",
    size: 32,
  },
};

// ── SAML (initials fallback) ──
export const SAML: Story = {
  args: {
    type: "SAML",
    size: 32,
  },
};

// ── Unknown Provider (auto initials) ──
export const UnknownProvider: Story = {
  args: {
    type: "MyCustomProvider",
    size: 32,
  },
};

// ── Large Size ──
export const LargeSize: Story = {
  args: {
    type: "GitHub",
    size: 64,
  },
};

// ── Small Size ──
export const SmallSize: Story = {
  args: {
    type: "Google",
    size: 20,
  },
};

// ── Provider Grid ──
export const ProviderGrid: Story = {
  render: () => (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 12, alignItems: "center" }}>
      {[
        "GitHub",
        "Google",
        "AzureAD",
        "WeChat",
        "DingTalk",
        "Facebook",
        "Twitter",
        "LinkedIn",
        "GitLab",
        "Slack",
        "Discord",
        "Apple",
        "Okta",
        "SAML",
        "MetaMask",
        "Alipay",
        "QQ",
        "Bilibili",
        "OIDC",
        "GuliangIAM",
      ].map((type) => (
        <div key={type} style={{ textAlign: "center", width: 64 }}>
          <ProviderIcon type={type} size={36} />
          <div
            style={{ fontSize: 10, marginTop: 4, color: "var(--gl-color-text-secondary, #595959)" }}
          >
            {type}
          </div>
        </div>
      ))}
    </div>
  ),
};
