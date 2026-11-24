/**
 * Organization 详情页 · 「基本与品牌」分区的主题配置。
 *
 * 结构：主题预设 → token 编辑器 → 自定义 CSS → 静态资源 JSON。
 * 所有值都写入既有表单字段（themeName / themeTokensJson / themeCustomCss /
 * themeAssetsJson），保存后由 `OrganizationThemeProvider` 在控制台即时生效。
 */
import { AutoComplete, Form } from "antd";
import { useTranslation } from "react-i18next";
import { CodeEditor } from "@/shared/ui/CodeEditor";
import { HelpLabel } from "@/shared/ui/FieldHelp";
import { ThemeTokenEditor } from "@/shared/ui/ThemeTokenEditor";
import { StructuredJsonField } from "../StructuredJsonEditor";

const THEME_PRESETS = [
  { value: "default", labelKey: "default" },
  { value: "enterprise", labelKey: "enterprise" },
  { value: "high-contrast", labelKey: "highContrast" },
] as const;

export function ThemeTab() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");

  return (
    <>
      <Form.Item
        label={
          <HelpLabel helpKey="organizations.theme.name">{tr("organization.themeName")}</HelpLabel>
        }
        name="themeName"
        rules={[{ required: true, message: t("required") }]}
        extra={tr("organization.themePresetHint")}
      >
        <AutoComplete
          placeholder="default"
          options={THEME_PRESETS.map((preset) => ({
            value: preset.value,
            label: tr(`organization.themePresets.${preset.labelKey}`),
          }))}
          filterOption={(input, option) =>
            String(option?.value ?? "")
              .toLowerCase()
              .includes(input.toLowerCase())
          }
        />
      </Form.Item>

      <Form.Item
        label={
          <HelpLabel helpKey="organizations.theme.tokens">
            {tr("organization.themeTokensJson")}
          </HelpLabel>
        }
        name="themeTokensJson"
        extra={tr("organization.themeTokensHint")}
      >
        <ThemeTokenEditor />
      </Form.Item>

      <Form.Item
        label={
          <HelpLabel helpKey="organizations.theme.customCss">
            {tr("organization.themeCustomCss")}
          </HelpLabel>
        }
        name="themeCustomCss"
        extra={tr("organization.themeCustomCssHint")}
      >
        <CodeEditor language="css" height={200} placeholder={tr("organization.themeCustomCss")} />
      </Form.Item>

      <StructuredJsonField
        label={tr("organization.themeAssetsJson")}
        name="themeAssetsJson"
        rows={4}
      />
    </>
  );
}
