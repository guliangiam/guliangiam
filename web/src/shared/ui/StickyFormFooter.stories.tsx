import type { Meta, StoryObj } from "@storybook/react";
import { fn } from "storybook/test";
import { StickyFormFooter } from "./StickyFormFooter";
import { Button, Tag, Space } from "antd";

/**
 * StickyFormFooter component provides a sticky action bar at the bottom of forms.
 *
 * Features:
 * - Fixed at the viewport bottom, always visible
 * - Contains save and reset buttons
 * - Supports loading state (saving)
 * - Supports extra action area
 * - Button text supports i18n internationalization
 */
const meta: Meta<typeof StickyFormFooter> = {
  title: "Shared/UI/StickyFormFooter",
  component: StickyFormFooter,
  tags: ["autodocs"],
  argTypes: {
    loading: {
      description: "Whether the save button is in loading state",
      control: { type: "boolean" },
    },
    onSave: {
      description: "Save callback",
      table: {
        category: "Events",
      },
    },
    onReset: {
      description: "Reset callback",
      table: {
        category: "Events",
      },
    },
    saveText: {
      description: "Save button text (overrides i18n default)",
      control: { type: "text" },
    },
    resetText: {
      description: "Reset button text (overrides i18n default)",
      control: { type: "text" },
    },
    extra: {
      description: "Extra action area (rendered to the left of buttons)",
      table: {
        category: "Content",
      },
    },
  },
  args: {
    onSave: fn(),
    onReset: fn(),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default ──
export const Default: Story = {
  args: {
    saveText: "Save",
    resetText: "Reset",
  },
};

// ── Loading (save button in loading state) ──
export const Loading: Story = {
  args: {
    loading: true,
    saveText: "Save",
    resetText: "Reset",
  },
};

// ── Save Only (no reset) ──
export const SaveOnly: Story = {
  args: {
    saveText: "Submit",
    onReset: undefined,
  },
};

// ── With Extra Actions ──
export const WithExtra: Story = {
  args: {
    saveText: "Save",
    resetText: "Reset",
    extra: (
      <Space>
        <Tag color="blue">Draft</Tag>
        <Button size="small">Preview</Button>
      </Space>
    ),
  },
};

// ── Custom Button Text ──
export const CustomText: Story = {
  args: {
    saveText: "Confirm Changes",
    resetText: "Cancel",
  },
};
