import { Tag } from "antd";
import { memo } from "react";
import { getCategoryColor } from "@/shared/hooks/useProviderCategories";

export type ProviderCategoryBadgeProps = {
  /** Provider category string (e.g. "OAuth", "SAML") */
  category: string;
};

/**
 * A colored Tag badge for displaying a provider category.
 *
 * Uses the centralized getCategoryColor mapping so both the
 * detail page and list page render categories consistently.
 */
export const ProviderCategoryBadge = memo(function ProviderCategoryBadge({
  category,
}: ProviderCategoryBadgeProps) {
  if (!category) return <>-</>;
  return <Tag color={getCategoryColor(category)}>{category}</Tag>;
});
