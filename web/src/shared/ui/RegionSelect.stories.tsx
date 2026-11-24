import type { Meta, StoryObj } from "@storybook/react";
import { fn } from "storybook/test";
import { RegionSelect } from "./RegionSelect";

/**
 * RegionSelect component for selecting countries/regions with flags.
 *
 * Features:
 * - Searchable dropdown with country flags
 * - Shows country name and code
 * - i18n-aware country names
 * - Configurable size
 */
const meta: Meta<typeof RegionSelect> = {
  title: "Shared/UI/RegionSelect",
  component: RegionSelect,
  tags: ["autodocs"],
  argTypes: {
    value: {
      description: "Selected country code (controlled mode)",
      control: { type: "text" },
    },
    onChange: {
      description: "Callback when selection changes",
      table: { category: "Events" },
    },
    defaultValue: {
      description: "Default selected country code",
      control: { type: "text" },
    },
    size: {
      description: "Select size",
      control: { type: "select" },
      options: ["small", "middle", "large"],
    },
    countryCodes: {
      description: "Restrict to specific country codes",
      control: { type: "object" },
    },
  },
  args: {
    onChange: fn(),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default ──
export const Default: Story = {
  args: {
    style: { width: 300 },
  },
};

// ── Pre-selected (CN) ──
export const PreSelected: Story = {
  args: {
    value: "CN",
    style: { width: 300 },
  },
};

// ── Small Size ──
export const SmallSize: Story = {
  args: {
    size: "small",
    style: { width: 240 },
  },
};

// ── Restricted Countries ──
export const RestrictedCountries: Story = {
  args: {
    countryCodes: ["US", "CN", "JP", "KR", "GB", "DE"],
    style: { width: 300 },
  },
};

// ── With Default Value ──
export const WithDefaultValue: Story = {
  args: {
    defaultValue: "US",
    style: { width: 300 },
  },
};
