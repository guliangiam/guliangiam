import type { Meta, StoryObj } from "@storybook/react";
import { fn } from "storybook/test";
import { ThemeTokenEditor } from "./ThemeTokenEditor";

/**
 * ThemeTokenEditor component for editing antd theme design tokens.
 *
 * Features:
 * - Visual editor for color tokens (via ColorPicker)
 * - Numeric token editing (border radius, font size)
 * - Switch toggles (wireframe mode)
 * - JSON string input/output for persistence
 * - Parse error handling
 */
const meta: Meta<typeof ThemeTokenEditor> = {
  title: "Shared/UI/ThemeTokenEditor",
  component: ThemeTokenEditor,
  tags: ["autodocs"],
  argTypes: {
    value: {
      description: "JSON string of theme tokens",
      control: { type: "text" },
    },
    onChange: {
      description: "Callback when tokens change (returns JSON string)",
      table: { category: "Events" },
    },
  },
  args: {
    onChange: fn(),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default (empty) ──
export const Default: Story = {
  args: {},
};

// ── With Pre-set Colors ──
export const WithPresetColors: Story = {
  args: {
    value: JSON.stringify(
      {
        colorPrimary: "#1677ff",
        colorSuccess: "#52c41a",
        colorWarning: "#faad14",
        colorError: "#ff4d4f",
      },
      null,
      2,
    ),
  },
};

// ── Dark Theme Preset ──
export const DarkThemePreset: Story = {
  args: {
    value: JSON.stringify(
      {
        colorPrimary: "#1668dc",
        colorBgBase: "#141414",
        colorTextBase: "#ffffff",
        colorBgContainer: "#1f1f1f",
        colorBorder: "#424242",
        borderRadius: 6,
        fontSize: 14,
      },
      null,
      2,
    ),
  },
};

// ── Custom Border Radius ──
export const CustomBorderRadius: Story = {
  args: {
    value: JSON.stringify(
      {
        borderRadius: 12,
        fontSize: 16,
      },
      null,
      2,
    ),
  },
};

// ── Wireframe Mode ──
export const WireframeMode: Story = {
  args: {
    value: JSON.stringify(
      {
        wireframe: true,
        colorPrimary: "#722ed1",
      },
      null,
      2,
    ),
  },
};

// ── Invalid JSON (error state) ──
export const InvalidJson: Story = {
  args: {
    value: "not valid json{",
  },
};
