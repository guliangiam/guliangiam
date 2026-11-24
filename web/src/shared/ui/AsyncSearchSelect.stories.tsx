import type { Meta, StoryObj } from "@storybook/react";
import { fn } from "storybook/test";
import { AsyncSearchSelect } from "./AsyncSearchSelect";

/**
 * AsyncSearchSelect component for searchable dropdowns with async data loading.
 *
 * Features:
 * - Debounced async search (250ms)
 * - Merges static options with search results
 * - Supports multiple selection mode
 * - Shows loading skeleton while fetching
 */
const meta: Meta<typeof AsyncSearchSelect> = {
  title: "Shared/UI/AsyncSearchSelect",
  component: AsyncSearchSelect,
  tags: ["autodocs"],
  argTypes: {
    value: {
      description: "Selected value (controlled mode)",
      control: { type: "text" },
    },
    onChange: {
      description: "Callback when selection changes",
      table: { category: "Events" },
    },
    placeholder: {
      description: "Placeholder text",
      control: { type: "text" },
    },
    mode: {
      description: "Selection mode",
      control: { type: "select" },
      options: [undefined, "multiple"],
    },
    allowClear: {
      description: "Whether to allow clearing the selection",
      control: { type: "boolean" },
    },
    loader: {
      description: "Async function to load options based on search text",
      table: { category: "Data" },
    },
    staticOptions: {
      description: "Static options always available",
      table: { category: "Data" },
    },
  },
  args: {
    onChange: fn(),
    loader: fn(async (search: string) => {
      // Simulated async loader
      const allOptions = [
        { label: "Alice Johnson", value: "alice" },
        { label: "Bob Smith", value: "bob" },
        { label: "Charlie Brown", value: "charlie" },
        { label: "Diana Prince", value: "diana" },
        { label: "Edward Norton", value: "edward" },
      ];
      await new Promise((r) => setTimeout(r, 500));
      if (!search) return allOptions;
      return allOptions.filter((o) => o.label.toLowerCase().includes(search.toLowerCase()));
    }),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default ──
export const Default: Story = {
  args: {
    placeholder: "Search users...",
    style: { width: 300 },
  },
};

// ── With Static Options ──
export const WithStaticOptions: Story = {
  args: {
    placeholder: "Search or select...",
    staticOptions: [
      { label: "Current User", value: "current" },
      { label: "System Admin", value: "admin" },
    ],
    style: { width: 300 },
  },
};

// ── Multiple Selection ──
export const Multiple: Story = {
  args: {
    placeholder: "Select multiple users...",
    mode: "multiple",
    style: { width: 400 },
  },
};

// ── With Pre-selected Value ──
export const PreSelected: Story = {
  args: {
    value: "alice",
    placeholder: "Search users...",
    allowClear: true,
    style: { width: 300 },
  },
};

// ── Clearable ──
export const Clearable: Story = {
  args: {
    placeholder: "Search and clear...",
    allowClear: true,
    style: { width: 300 },
  },
};
