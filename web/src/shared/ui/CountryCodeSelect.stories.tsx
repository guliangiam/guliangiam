import type { Meta, StoryObj } from "@storybook/react";
import { fn } from "storybook/test";
import { CountryCodeSelect } from "./CountryCodeSelect";

/**
 * CountryCodeSelect component for selecting country calling codes.
 *
 * Features:
 * - Searchable dropdown with country flags
 * - Shows country name + calling code
 * - Supports multiple selection mode
 * - Supports "All" default option
 * - i18n-aware country names
 */
const meta: Meta<typeof CountryCodeSelect> = {
  title: "Shared/UI/CountryCodeSelect",
  component: CountryCodeSelect,
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
    disabled: {
      description: "Whether the select is disabled",
      control: { type: "boolean" },
      table: { defaultValue: { summary: "false" } },
    },
    mode: {
      description: "Selection mode: single or multiple/tags",
      control: { type: "select" },
      options: [undefined, "multiple", "tags"],
    },
    hasDefault: {
      description: "Whether to show 'All' as the first option",
      control: { type: "boolean" },
      table: { defaultValue: { summary: "false" } },
    },
    countryCodes: {
      description: "Restrict to specific country codes",
      control: { type: "object" },
    },
    initValue: {
      description: "Initial selected value (uncontrolled mode)",
      control: { type: "text" },
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
    style: { width: 240 },
  },
};

// ── Pre-selected (US) ──
export const PreSelected: Story = {
  args: {
    value: "US",
    style: { width: 240 },
  },
};

// ── With "All" Option ──
export const WithAllOption: Story = {
  args: {
    hasDefault: true,
    style: { width: 240 },
  },
};

// ── Multiple Selection ──
export const Multiple: Story = {
  args: {
    mode: "multiple",
    style: { width: 300 },
  },
};

// ── Restricted Countries ──
export const RestrictedCountries: Story = {
  args: {
    countryCodes: ["US", "CN", "JP", "GB", "DE", "FR"],
    style: { width: 240 },
  },
};

// ── Disabled ──
export const Disabled: Story = {
  args: {
    disabled: true,
    value: "CN",
    style: { width: 240 },
  },
};
