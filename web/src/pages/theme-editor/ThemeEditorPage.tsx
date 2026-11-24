import {
  Button,
  Card,
  ColorPicker,
  InputNumber,
  Segmented,
  Select,
  Space,
  Tag,
  Typography,
  message,
} from "antd";
import { useTranslation } from "react-i18next";
import { useThemeStore } from "@/features/theme/theme-store";
import { PRESET_THEMES } from "@/app/theme/theme-presets";
import { useLayoutStore, type LayoutMode } from "@/features/layout/layout-store";
import {
  selectResolvedSectionNavScheme,
  useSectionNavStore,
} from "@/features/section-nav/section-nav-store";
import type { SectionNavScheme } from "@/features/section-nav/types";

export default function ThemeEditorPage() {
  const { t } = useTranslation("common");
  const themeMode = useThemeStore((s) => s.themeMode);
  const setThemeMode = useThemeStore((s) => s.setThemeMode);
  const config = useThemeStore((s) => s.config);
  const setConfig = useThemeStore((s) => s.setConfig);
  const applyConfig = useThemeStore((s) => s.applyConfig);

  // 分区导航落位：显示的是解析后的生效值，偏好来源与「恢复默认」按本机偏好是否存在切换。
  const sectionNavScheme = useSectionNavStore(selectResolvedSectionNavScheme);
  const hasLocalSectionNav = useSectionNavStore((s) => s.scheme !== null);
  const setSectionNavScheme = useSectionNavStore((s) => s.setScheme);
  const resetSectionNavScheme = useSectionNavStore((s) => s.resetScheme);

  const layoutMode = useLayoutStore((s) => s.layoutMode);
  const setLayoutMode = useLayoutStore((s) => s.setLayoutMode);

  const handleExport = () => {
    const json = JSON.stringify(config, null, 2);
    void navigator.clipboard.writeText(json).then(
      () => message.success(t("copiedToClipboard")),
      () => message.error(t("copyFailed")),
    );
  };

  const handleImport = async () => {
    try {
      const text = await navigator.clipboard.readText();
      const parsed: unknown = JSON.parse(text);
      if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
        const candidate = parsed as Record<string, unknown>;
        // 至少携带一个主题 token，且出现的每个 token 类型都必须正确，
        // 否则拒绝导入，避免把非法值写进主题配置。
        const hasToken =
          candidate.primaryColor !== undefined ||
          candidate.darkPrimaryColor !== undefined ||
          candidate.borderRadius !== undefined;
        const validTypes =
          (candidate.primaryColor === undefined || typeof candidate.primaryColor === "string") &&
          (candidate.darkPrimaryColor === undefined ||
            typeof candidate.darkPrimaryColor === "string") &&
          (candidate.borderRadius === undefined || typeof candidate.borderRadius === "number");
        if (hasToken && validTypes) {
          applyConfig({
            primaryColor: (candidate.primaryColor as string | undefined) ?? config.primaryColor,
            darkPrimaryColor:
              (candidate.darkPrimaryColor as string | undefined) ?? config.darkPrimaryColor,
            borderRadius: (candidate.borderRadius as number | undefined) ?? config.borderRadius,
          });
          message.success(t("importSuccess"));
          return;
        }
      }
      message.error(t("importFailed"));
    } catch {
      message.error(t("importFailed"));
    }
  };

  return (
    <div className="gl-content__inner--reading">
      <Typography.Title level={2}>{t("themeEditor")}</Typography.Title>
      <Typography.Paragraph type="secondary">{t("themeEditorDesc")}</Typography.Paragraph>

      <Card variant="borderless" style={{ marginBottom: 24 }}>
        <Typography.Title level={4}>{t("themeMode")}</Typography.Title>
        <Select
          value={themeMode}
          aria-label={t("themeMode")}
          style={{ width: 200 }}
          options={[
            { label: t("light"), value: "light" },
            { label: t("dark"), value: "dark" },
            { label: t("systemMode"), value: "system" },
          ]}
          onChange={setThemeMode}
        />
      </Card>

      <Card variant="borderless" style={{ marginBottom: 24 }}>
        <Typography.Title level={4}>{t("sectionNav.layoutTitle")}</Typography.Title>
        <Typography.Paragraph type="secondary">{t("sectionNav.layoutDesc")}</Typography.Paragraph>
        <Space orientation="vertical" size={16} style={{ width: "100%" }}>
          <Space wrap align="center">
            <Segmented
              value={sectionNavScheme}
              onChange={(value) => setSectionNavScheme(String(value) as SectionNavScheme)}
              options={[
                { label: t("sectionNav.optionRailRight"), value: "rail-right" },
                { label: t("sectionNav.optionRailLeft"), value: "rail-left" },
                { label: t("sectionNav.optionRailTop"), value: "rail-top" },
              ]}
            />
            <Tag color={hasLocalSectionNav ? "blue" : "default"}>
              {hasLocalSectionNav
                ? t("sectionNav.preferenceLocal")
                : t("sectionNav.preferenceDefault")}
            </Tag>
            <Button disabled={!hasLocalSectionNav} onClick={resetSectionNavScheme}>
              {t("sectionNav.reset")}
            </Button>
          </Space>
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <Typography.Text style={{ minWidth: 120 }}>
              {t("sectionNav.mainNavTitle")}
            </Typography.Text>
            <Segmented
              value={layoutMode}
              onChange={(value) => setLayoutMode(String(value) as LayoutMode)}
              options={[
                { label: t("sectionNav.mainNavSidebar"), value: "sidebar" },
                { label: t("sectionNav.mainNavTop"), value: "top" },
              ]}
            />
          </div>
        </Space>
      </Card>

      <Card variant="borderless" style={{ marginBottom: 24 }}>
        <Typography.Title level={4}>{t("presetThemes")}</Typography.Title>
        <Space wrap>
          {PRESET_THEMES.map((theme) => (
            <Button
              key={theme.label}
              type={config.primaryColor === theme.value.primaryColor ? "primary" : "default"}
              onClick={() =>
                setConfig({
                  primaryColor: theme.value.primaryColor,
                  darkPrimaryColor: theme.value.darkPrimaryColor,
                })
              }
            >
              <Space>
                <span
                  style={{
                    display: "inline-block",
                    width: 16,
                    height: 16,
                    borderRadius: "50%",
                    background: `linear-gradient(135deg, ${theme.value.primaryColor} 50%, ${theme.value.darkPrimaryColor} 50%)`,
                  }}
                />
                {theme.label}
              </Space>
            </Button>
          ))}
        </Space>
      </Card>

      <Card variant="borderless" style={{ marginBottom: 24 }}>
        <Typography.Title level={4}>{t("customColors")}</Typography.Title>
        <Space orientation="vertical" size={16} style={{ width: "100%" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <Typography.Text style={{ minWidth: 120 }}>{t("primaryColor")}</Typography.Text>
            <ColorPicker
              value={config.primaryColor}
              onChangeComplete={(value) => setConfig({ primaryColor: value.toHexString() })}
            />
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <Typography.Text style={{ minWidth: 120 }}>{t("darkPrimaryColor")}</Typography.Text>
            <ColorPicker
              value={config.darkPrimaryColor}
              onChangeComplete={(value) => setConfig({ darkPrimaryColor: value.toHexString() })}
            />
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <Typography.Text style={{ minWidth: 120 }}>{t("borderRadius")}</Typography.Text>
            <InputNumber
              min={0}
              max={24}
              aria-label={t("borderRadius")}
              value={config.borderRadius}
              onChange={(val) => val !== null && setConfig({ borderRadius: val })}
            />
          </div>
        </Space>
      </Card>

      <Card variant="borderless">
        <Typography.Title level={4}>{t("importExport")}</Typography.Title>
        <Space>
          <Button onClick={handleExport}>{t("exportConfig")}</Button>
          <Button onClick={handleImport}>{t("importConfig")}</Button>
        </Space>
      </Card>
    </div>
  );
}
