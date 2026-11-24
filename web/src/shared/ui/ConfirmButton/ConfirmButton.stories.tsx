import type { Meta, StoryObj } from "@storybook/react";
import { fn } from "storybook/test";
import { ConfirmButton } from "./ConfirmButton";

const meta: Meta<typeof ConfirmButton> = {
  title: "Shared/UI/ConfirmButton",
  component: ConfirmButton,
  tags: ["autodocs"],
  args: {
    onConfirm: fn(),
    children: "Delete user",
  },
  argTypes: {
    danger: { control: "boolean" },
    disabled: { control: "boolean" },
    size: { control: "inline-radio", options: ["small", "middle", "large"] },
    timeout: { control: "number" },
    confirmLabel: { control: "text" },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Warning: Story = {
  args: {
    danger: false,
    children: "Disable account",
    confirmLabel: "Confirm disable",
  },
};

export const Disabled: Story = {
  args: {
    disabled: true,
  },
};
