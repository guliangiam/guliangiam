import type { Meta, StoryObj } from "@storybook/react";
import { fn } from "storybook/test";
import { GroupTreeSelect } from "./GroupTreeSelect";

/**
 * GroupTreeSelect component for selecting groups in a tree hierarchy.
 *
 * Features:
 * - Builds tree structure from flat group list with parentId
 * - Single or multiple selection
 * - Searchable tree nodes
 * - Clearable selection
 */
const meta: Meta<typeof GroupTreeSelect> = {
  title: "Shared/UI/GroupTreeSelect",
  component: GroupTreeSelect,
  tags: ["autodocs"],
  argTypes: {
    value: {
      description: "Selected group ID(s)",
      control: { type: "text" },
    },
    onChange: {
      description: "Callback when selection changes",
      table: { category: "Events" },
    },
    groups: {
      description: "Flat list of groups with id, name, displayName, parentId",
      control: { type: "object" },
    },
    multiple: {
      description: "Whether to allow multiple selection",
      control: { type: "boolean" },
      table: { defaultValue: { summary: "false" } },
    },
    placeholder: {
      description: "Placeholder text",
      control: { type: "text" },
    },
  },
  args: {
    onChange: fn(),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const sampleGroups = [
  { id: "1", name: "engineering", displayName: "Engineering", parentId: undefined },
  { id: "2", name: "frontend", displayName: "Frontend Team", parentId: "1" },
  { id: "3", name: "backend", displayName: "Backend Team", parentId: "1" },
  { id: "4", name: "devops", displayName: "DevOps Team", parentId: "1" },
  { id: "5", name: "marketing", displayName: "Marketing", parentId: undefined },
  { id: "6", name: "content", displayName: "Content Team", parentId: "5" },
  { id: "7", name: "seo", displayName: "SEO Team", parentId: "5" },
  { id: "8", name: "admin", displayName: "Administrators", parentId: undefined },
];

// ── Default (single select) ──
export const Default: Story = {
  args: {
    groups: sampleGroups,
    placeholder: "Select group",
    style: { width: 300 },
  },
};

// ── Multiple Selection ──
export const Multiple: Story = {
  args: {
    groups: sampleGroups,
    multiple: true,
    placeholder: "Select groups",
    style: { width: 300 },
  },
};

// ── Pre-selected Value ──
export const PreSelected: Story = {
  args: {
    groups: sampleGroups,
    value: "2",
    placeholder: "Select group",
    style: { width: 300 },
  },
};

// ── Empty Groups ──
export const EmptyGroups: Story = {
  args: {
    groups: [],
    placeholder: "No groups available",
    style: { width: 300 },
  },
};

// ── Flat Structure (no hierarchy) ──
export const FlatStructure: Story = {
  args: {
    groups: [
      { id: "1", name: "users", displayName: "Users" },
      { id: "2", name: "admins", displayName: "Administrators" },
      { id: "3", name: "editors", displayName: "Editors" },
    ],
    placeholder: "Select group",
    style: { width: 300 },
  },
};
