import type { Meta, StoryObj } from "@storybook/react";
import { ProviderCategoryBadge } from "./ProviderCategoryBadge";

/**
 * ProviderCategoryBadge component displays the category label of an identity Provider.
 *
 * Uses antd Tag component, automatically mapping colors based on category name.
 * Color mapping is managed by `getCategoryColor` in `useProviderCategories`.
 */
const meta: Meta<typeof ProviderCategoryBadge> = {
  title: "Shared/UI/ProviderCategoryBadge",
  component: ProviderCategoryBadge,
  tags: ["autodocs"],
  argTypes: {
    category: {
      description: "Provider category name (e.g. OAuth, SAML, OIDC, etc.)",
      control: { type: "text" },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default ──
export const Default: Story = {
  args: {
    category: "OAuth",
  },
};

// ── OAuth2.0 ──
export const OAuth: Story = {
  args: {
    category: "OAuth2",
  },
};

// ── SAML ──
export const SAML: Story = {
  args: {
    category: "SAML",
  },
};

// ── OIDC ──
export const OIDC: Story = {
  args: {
    category: "OIDC",
  },
};

// ── CAS ──
export const CAS: Story = {
  args: {
    category: "CAS",
  },
};

// ── LDAP ──
export const LDAP: Story = {
  args: {
    category: "LDAP",
  },
};

// ── Empty (shows placeholder) ──
export const Empty: Story = {
  args: {
    category: "",
  },
};

// ── All Categories ──
export const AllCategories: Story = {
  render: () => (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
      {["OAuth2", "OIDC", "SAML", "CAS", "LDAP", "AD", "Password", "SMS", "Email"].map((cat) => (
        <ProviderCategoryBadge key={cat} category={cat} />
      ))}
    </div>
  ),
};
