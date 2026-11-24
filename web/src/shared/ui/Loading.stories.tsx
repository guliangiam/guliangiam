import type { Meta, StoryObj } from "@storybook/react";
import { Loading, AiDots } from "./Loading";

/**
 * Loading component provides a unified loading state indicator.
 *
 * Three modes:
 * - **page** — full-page centered loading, suitable for route transitions
 * - **section** — in-block loading (default), suitable for card/content areas
 * - **small** — inline small loading, suitable for next to buttons, inside list items
 *
 * Uses AiDots animation internally (indigo → violet → cyan bouncing dots).
 */
const meta: Meta<typeof Loading> = {
  title: "Shared/UI/Loading",
  component: Loading,
  tags: ["autodocs"],
  argTypes: {
    spinning: {
      description: "Whether to show loading state",
      control: { type: "boolean" },
      table: {
        defaultValue: { summary: "true" },
      },
    },
    tip: {
      description: "Loading tip text (not displayed in small mode)",
      control: { type: "text" },
    },
    type: {
      description: "Loading type: page=full-page, section=in-block, small=inline",
      control: { type: "select" },
      options: ["page", "section", "small"],
      table: {
        defaultValue: { summary: "section" },
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default (section mode) ──
export const Default: Story = {
  args: {
    spinning: true,
  },
};

// ── Full Page Loading ──
export const PageLoading: Story = {
  args: {
    type: "page",
    tip: "Loading page...",
  },
};

// ── Section Loading ──
export const SectionLoading: Story = {
  args: {
    type: "section",
    tip: "Fetching data...",
  },
};

// ── Inline Small Loading ──
export const SmallLoading: Story = {
  args: {
    type: "small",
  },
};

// ── With Tip Text ──
export const WithTip: Story = {
  args: {
    tip: "Saving configuration, please wait...",
  },
};

// ── Not Spinning ──
export const NotSpinning: Story = {
  args: {
    spinning: false,
    tip: "Loading complete",
  },
};

// ── AiDots Standalone ──
const aiDotsMeta: Meta<typeof AiDots> = {
  title: "Shared/UI/Loading/AiDots",
  component: AiDots,
  tags: ["autodocs"],
  argTypes: {
    size: {
      description: "Animation dot size",
      control: { type: "select" },
      options: ["small", "medium", "large"],
      table: {
        defaultValue: { summary: "medium" },
      },
    },
  },
};

export { aiDotsMeta as AiDotsMeta };

// AiDots stories
export const AiDotsSmall: StoryObj<typeof AiDots> = {
  args: { size: "small" },
};

export const AiDotsMedium: StoryObj<typeof AiDots> = {
  args: { size: "medium" },
};

export const AiDotsLarge: StoryObj<typeof AiDots> = {
  args: { size: "large" },
};
