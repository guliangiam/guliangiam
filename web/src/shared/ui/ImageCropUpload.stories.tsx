import type { Meta, StoryObj } from "@storybook/react";
import { fn } from "storybook/test";
import { ImageCropUpload } from "./ImageCropUpload";

/**
 * ImageCropUpload component for avatar/image upload with crop functionality.
 *
 * Features:
 * - Free-transform image crop before upload (cropperjs: resizable crop box, wheel zoom, button rotate)
 * - Circle or square avatar shape
 * - Preview modal on click
 * - File size validation
 * - Disabled state support
 */
const meta: Meta<typeof ImageCropUpload> = {
  title: "Shared/UI/ImageCropUpload",
  component: ImageCropUpload,
  tags: ["autodocs"],
  argTypes: {
    value: {
      description: "Current image URL (controlled mode)",
      control: { type: "text" },
    },
    onChange: {
      description: "Callback when image URL changes",
      table: { category: "Events" },
    },
    label: {
      description: "Upload button label text",
      control: { type: "text" },
    },
    aspectRatio: {
      description: "Crop aspect ratio (1 = square)",
      control: { type: "number" },
      table: { defaultValue: { summary: "1" } },
    },
    shape: {
      description: "Avatar shape: circle or square",
      control: { type: "select" },
      options: ["circle", "square"],
      table: { defaultValue: { summary: "circle" } },
    },
    maxSize: {
      description: "Maximum file size in MB",
      control: { type: "number" },
      table: { defaultValue: { summary: "5" } },
    },
    disabled: {
      description: "Whether the upload is disabled",
      control: { type: "boolean" },
      table: { defaultValue: { summary: "false" } },
    },
  },
  args: {
    onChange: fn(),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// ── Default (no image) ──
export const Default: Story = {
  args: {
    label: "Upload Avatar",
  },
};

// ── With Existing Image ──
export const WithImage: Story = {
  args: {
    value: "https://api.dicebear.com/7.x/avataaars/svg?seed=Felix",
    label: "Change Avatar",
  },
};

// ── Square Shape ──
export const SquareShape: Story = {
  args: {
    value: "https://api.dicebear.com/7.x/avataaars/svg?seed=Aneka",
    label: "Upload Logo",
    shape: "square",
  },
};

// ── Disabled ──
export const Disabled: Story = {
  args: {
    label: "Upload Avatar",
    disabled: true,
  },
};

// ── Custom Aspect Ratio (16:9 banner) ──
export const BannerCrop: Story = {
  args: {
    label: "Upload Banner",
    shape: "square",
    aspectRatio: 16 / 9,
  },
};

// ── Small Max Size ──
export const SmallMaxSize: Story = {
  args: {
    label: "Upload Icon",
    maxSize: 1,
    shape: "square",
  },
};
