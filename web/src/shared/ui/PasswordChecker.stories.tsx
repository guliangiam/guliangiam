import type { Meta, StoryObj } from "@storybook/react";
import { PasswordChecker } from "./PasswordChecker";
import type { PasswordRule } from "./PasswordChecker";

/**
 * PasswordChecker component for real-time password strength and rule compliance checking.
 *
 * Features:
 * - Displays password strength progress bar (Weak / Fair / Strong)
 * - Checks password rules one by one (length, case, digits, special chars, common passwords)
 * - Supports custom rules
 */
const meta: Meta<typeof PasswordChecker> = {
  title: "Shared/UI/PasswordChecker",
  component: PasswordChecker,
  tags: ["autodocs"],
  argTypes: {
    password: {
      description: "Password string to check",
      control: { type: "text" },
    },
    visible: {
      description: "Whether to show the checker",
      control: { type: "boolean" },
      table: {
        defaultValue: { summary: "true" },
      },
    },
    showStrength: {
      description: "Whether to show the strength progress bar",
      control: { type: "boolean" },
      table: {
        defaultValue: { summary: "true" },
      },
    },
    rules: {
      description: "Custom password rules array",
      control: { type: "object" },
      table: {
        category: "Advanced",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default (empty password) ──
export const Default: Story = {
  args: {
    password: "",
  },
};

// ── Weak Password ──
export const WeakPassword: Story = {
  args: {
    password: "abc",
  },
};

// ── Fair Password ──
export const FairPassword: Story = {
  args: {
    password: "Password1",
  },
};

// ── Strong Password ──
export const StrongPassword: Story = {
  args: {
    password: "MyStr0ng#P@ss2024",
  },
};

// ── Common Weak Password (e.g. 123456) ──
export const CommonPassword: Story = {
  args: {
    password: "123456",
  },
};

// ── Without Strength Bar ──
export const WithoutStrength: Story = {
  args: {
    password: "Password1",
    showStrength: false,
  },
};

// ── Custom Rules ──
const customRules: PasswordRule[] = [
  { key: "minLength6", label: "At least 6 characters", validate: (p) => p.length >= 6 },
  { key: "uppercase", label: "At least one uppercase letter", validate: (p) => /[A-Z]/.test(p) },
  { key: "digit", label: "At least one digit", validate: (p) => /[0-9]/.test(p) },
];

export const CustomRules: Story = {
  args: {
    password: "Abc12",
    rules: customRules,
  },
};

// ── Hidden ──
export const Hidden: Story = {
  args: {
    password: "test",
    visible: false,
  },
};
