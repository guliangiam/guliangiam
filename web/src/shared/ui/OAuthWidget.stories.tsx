import type { Meta, StoryObj } from "@storybook/react";
import { fn } from "storybook/test";
import { OAuthWidget } from "./OAuthWidget";

/**
 * OAuthWidget component for displaying and managing OAuth provider connections.
 *
 * Features:
 * - Shows provider icon and display name
 * - Link/Unlink buttons for identity management
 * - Loading state support
 * - Can disable link/unlink actions
 */
const meta: Meta<typeof OAuthWidget> = {
  title: "Shared/UI/OAuthWidget",
  component: OAuthWidget,
  tags: ["autodocs"],
  argTypes: {
    provider: {
      description: "Provider info object (type, name, displayName, category)",
      control: { type: "object" },
    },
    linkedIdentity: {
      description: "Linked identity data (null = not linked)",
      table: { category: "Data" },
    },
    canUnlink: {
      description: "Whether the unlink button is enabled",
      control: { type: "boolean" },
      table: { defaultValue: { summary: "true" } },
    },
    canLink: {
      description: "Whether the link button is enabled",
      control: { type: "boolean" },
      table: { defaultValue: { summary: "true" } },
    },
    loading: {
      description: "Whether the widget is in loading state",
      control: { type: "boolean" },
    },
    onLink: {
      description: "Callback when link button is clicked",
      table: { category: "Events" },
    },
    onUnlink: {
      description: "Callback when unlink button is clicked",
      table: { category: "Events" },
    },
  },
  args: {
    onLink: fn(),
    onUnlink: fn(),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Not Linked ──
export const NotLinked: Story = {
  args: {
    provider: {
      type: "GitHub",
      name: "github",
      displayName: "GitHub",
      category: "OAuth2",
    },
  },
};

// ── Linked ──
export const Linked: Story = {
  args: {
    provider: {
      type: "Google",
      name: "google",
      displayName: "Google",
      category: "OIDC",
    },
    linkedIdentity: {
      providerId: "google",
      identityType: "OAuth2",
      subject: "123456789",
      providerAccount: "user@gmail.com",
      profileSnapshot: {
        displayName: "John Doe",
        username: "johndoe",
      },
    },
  },
};

// ── Loading State ──
export const Loading: Story = {
  args: {
    provider: {
      type: "WeChat",
      name: "wechat",
      displayName: "WeChat",
      category: "OAuth2",
    },
    loading: true,
  },
};

// ── Cannot Unlink ──
export const CannotUnlink: Story = {
  args: {
    provider: {
      type: "GitHub",
      name: "github",
      displayName: "GitHub",
      category: "OAuth2",
    },
    linkedIdentity: {
      providerId: "github",
      identityType: "OAuth2",
      subject: "github-123",
      providerAccount: "octocat",
      profileSnapshot: { displayName: "Octocat" },
    },
    canUnlink: false,
  },
};

// ── Cannot Link ──
export const CannotLink: Story = {
  args: {
    provider: {
      type: "SAML",
      name: "saml-enterprise",
      displayName: "Enterprise SSO",
      category: "SAML",
    },
    canLink: false,
  },
};

// ── Multiple Providers ──
export const MultipleProviders: Story = {
  render: () => (
    <div style={{ display: "flex", flexDirection: "column", gap: 0, maxWidth: 500 }}>
      <OAuthWidget
        provider={{ type: "GitHub", name: "github", displayName: "GitHub", category: "OAuth2" }}
        linkedIdentity={{
          providerId: "github",
          identityType: "OAuth2",
          subject: "gh-1",
          profileSnapshot: { displayName: "devuser" },
        }}
        onLink={fn()}
        onUnlink={fn()}
      />
      <OAuthWidget
        provider={{ type: "Google", name: "google", displayName: "Google", category: "OIDC" }}
        onLink={fn()}
        onUnlink={fn()}
      />
      <OAuthWidget
        provider={{ type: "WeChat", name: "wechat", displayName: "WeChat", category: "OAuth2" }}
        linkedIdentity={{
          providerId: "wechat",
          identityType: "OAuth2",
          subject: "wx-2",
          profileSnapshot: { displayName: "微信用户" },
        }}
        onLink={fn()}
        onUnlink={fn()}
      />
    </div>
  ),
};
