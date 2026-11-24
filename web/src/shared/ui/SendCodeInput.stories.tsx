import type { Meta, StoryObj } from "@storybook/react";
import { fn } from "storybook/test";
import { SendCodeInput } from "./SendCodeInput";

/**
 * SendCodeInput component for verification code sending scenarios.
 *
 * Features:
 * - Input + send button combined layout
 * - Automatic cooldown countdown after sending (default 60 seconds)
 * - Supports async send callback
 * - Supports disabled state
 */
const meta: Meta<typeof SendCodeInput> = {
  title: "Shared/UI/SendCodeInput",
  component: SendCodeInput,
  tags: ["autodocs"],
  argTypes: {
    value: {
      description: "Input value (controlled mode)",
      control: { type: "text" },
    },
    onChange: {
      description: "Value change callback",
      table: {
        category: "Events",
      },
    },
    onSendCode: {
      description: "Send verification code callback (async)",
      table: {
        category: "Events",
      },
    },
    cooldown: {
      description: "Cooldown countdown seconds",
      control: { type: "number" },
      table: {
        defaultValue: { summary: "60" },
      },
    },
    placeholder: {
      description: "Input placeholder text",
      control: { type: "text" },
      table: {
        defaultValue: { summary: "Enter verification code" },
      },
    },
    sendButtonText: {
      description: "Send button text",
      control: { type: "text" },
      table: {
        defaultValue: { summary: "Send Code" },
      },
    },
    disabled: {
      description: "Whether disabled",
      control: { type: "boolean" },
      table: {
        defaultValue: { summary: "false" },
      },
    },
  },
  args: {
    onSendCode: fn(() => new Promise((resolve) => setTimeout(resolve, 1000))),
    onChange: fn(),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default ──
export const Default: Story = {
  args: {
    placeholder: "Enter verification code",
    sendButtonText: "Send Code",
  },
};

// ── With Value ──
export const WithValue: Story = {
  args: {
    value: "123456",
    placeholder: "Enter verification code",
    sendButtonText: "Send Code",
  },
};

// ── Disabled ──
export const Disabled: Story = {
  args: {
    disabled: true,
    placeholder: "Enter verification code",
    sendButtonText: "Send Code",
  },
};

// ── Short Cooldown (10s, for testing) ──
export const ShortCooldown: Story = {
  args: {
    cooldown: 10,
    placeholder: "Enter verification code",
    sendButtonText: "Send Code",
  },
};

// ── Custom Placeholder ──
export const CustomPlaceholder: Story = {
  args: {
    placeholder: "Enter 6-digit code",
    sendButtonText: "Get SMS Code",
  },
};

// ── English Mode ──
export const EnglishMode: Story = {
  args: {
    placeholder: "Enter 6-digit code",
    sendButtonText: "Send Code",
  },
};
