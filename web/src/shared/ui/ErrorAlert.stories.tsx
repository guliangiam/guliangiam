import type { Meta, StoryObj } from "@storybook/react";
import { fn } from "storybook/test";
import { ErrorAlert } from "./ErrorAlert";

/**
 * ErrorAlert component for displaying error/warning messages.
 *
 * Built on antd Alert, supports multiple error variants (auth, network, mfa, account, generic),
 * automatically selecting the appropriate Alert type (error / warning / info) based on variant.
 */
const meta: Meta<typeof ErrorAlert> = {
  title: "Shared/UI/ErrorAlert",
  component: ErrorAlert,
  tags: ["autodocs"],
  argTypes: {
    variant: {
      description: "Error category, determines Alert visual type",
      control: { type: "select" },
      options: ["auth", "network", "mfa", "account", "generic"],
      table: {
        defaultValue: { summary: "generic" },
      },
    },
    title: {
      description: "Error title",
      control: { type: "text" },
    },
    description: {
      description: "Error detailed description",
      control: { type: "text" },
    },
    closable: {
      description: "Whether the alert is closable",
      control: { type: "boolean" },
      table: {
        defaultValue: { summary: "false" },
      },
    },
    onClose: {
      description: "Close callback",
      table: {
        category: "Events",
      },
    },
  },
  args: {
    onClose: fn(),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default ──
export const Default: Story = {
  args: {
    title: "Operation Failed",
    description: "Please try again later or contact the administrator.",
    variant: "generic",
  },
};

// ── Auth Error ──
export const AuthError: Story = {
  args: {
    title: "Authentication Failed",
    description: "Invalid username or password. Please try again.",
    variant: "auth",
  },
};

// ── Network Warning ──
export const NetworkWarning: Story = {
  args: {
    title: "Network Connection Error",
    description:
      "Unable to connect to the server. Please check your network settings and try again.",
    variant: "network",
  },
};

// ── MFA Info ──
export const MfaInfo: Story = {
  args: {
    title: "Multi-Factor Authentication",
    description:
      "Please enter the verification code from your authenticator app to complete sign-in.",
    variant: "mfa",
  },
};

// ── Account Error ──
export const AccountError: Story = {
  args: {
    title: "Account Locked",
    description:
      "Due to multiple failed login attempts, your account has been temporarily locked. Please try again in 30 minutes or contact the administrator.",
    variant: "account",
  },
};

// ── Closable ──
export const Closable: Story = {
  args: {
    title: "Closable Alert",
    description: "Click the close button on the right to dismiss this alert.",
    variant: "generic",
    closable: true,
  },
};

// ── Title Only ──
export const TitleOnly: Story = {
  args: {
    title: "Simple Error Alert",
    variant: "auth",
  },
};

// ── Description Only ──
export const DescriptionOnly: Story = {
  args: {
    description: "This is an error message with only a description, no title.",
    variant: "network",
  },
};
