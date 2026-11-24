import {
  App,
  Button,
  Col,
  Dropdown,
  Form,
  Input,
  Modal,
  Row,
  Segmented,
  Space,
  Switch,
  Typography,
} from "antd";
import type { FormInstance } from "antd";
import { ExpandOutlined, RedoOutlined, UndoOutlined } from "@ant-design/icons";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { FormSection } from "@/shared/ui/FormSection";
import { CodeEditor } from "@/shared/ui/CodeEditor";
import SigninItemsEditor from "@/shared/ui/SigninItemsEditor";
import { SignupItemsEditor } from "@/shared/ui/SignupItemsEditor";
import { LoginPreviewPanel } from "./LoginPreviewPanel";
import { SignupPreviewFrame } from "@/pages/applications/SignupPreviewFrame";
import { getSigninItems, type SigninItemConfig } from "@/shared/lib/auth-signin-items";
import { getDefaultSignupItems, getSignupItems } from "@/shared/lib/signup-field-config";
import type { ApplicationConfigFormValues } from "./types";
import { UI_CUSTOMIZATION_FIELDS } from "../application-payload";
import { useUiConfigHistory } from "../useUiConfigHistory";
import { DEFAULT_FOOTER_HTML, UI_CODE_TEMPLATES, type UiCodeFieldKey } from "./ui-code-templates";
import { defaultSigninUrl, defaultSignupUrl, effectiveAccessUrl } from "../application-urls";

type UiCustomizationTabProps = {
  form: FormInstance<ApplicationConfigFormValues>;
  organizationSlug: string;
  /** 实体键（应用 id / "new"）：切换时清空撤销历史。 */
  entityKey: string;
  onResetToSaved: () => void;
  onResetToDefaults: () => void;
  /** setFieldValue 不经过 onValuesChange，程序化写入后手动补脏检查。 */
  onProgrammaticChange: () => void;
};

type PreviewBreakpoint = "desktop" | "mobile";

function SignupItemsEditorInput({
  value,
  onChange,
}: {
  value?: ApplicationConfigFormValues["signupItems"];
  onChange?: (v: ApplicationConfigFormValues["signupItems"]) => void;
}) {
  const initialItems = value?.length ? value : getDefaultSignupItems();
  return <SignupItemsEditor value={initialItems} onChange={(v) => onChange?.(v)} />;
}

type PreviewWatchValues = readonly [
  formBackgroundUrl: string | undefined,
  formOffset: string | undefined,
  formCss: string | undefined,
  formSideHtml: string | undefined,
  headerHtml: string | undefined,
  footerHtml: string | undefined,
  customCss: string | undefined,
  formBackgroundUrlMobile: string | undefined,
  formCssMobile: string | undefined,
  signinHtml: string | undefined,
  signupHtml: string | undefined,
  displayName: string | undefined,
  description: string | undefined,
  logo: string | undefined,
  signinItems: ApplicationConfigFormValues["signinItems"],
  signupItems: ApplicationConfigFormValues["signupItems"],
];

const selectPreviewValues = (store: ApplicationConfigFormValues): PreviewWatchValues => [
  store.formBackgroundUrl,
  store.formOffset,
  store.formCss,
  store.formSideHtml,
  store.headerHtml,
  store.footerHtml,
  store.customCss,
  store.formBackgroundUrlMobile,
  store.formCssMobile,
  store.signinHtml,
  store.signupHtml,
  store.displayName,
  store.description,
  store.logo,
  store.signinItems,
  store.signupItems,
];

const FRAME_BORDER: React.CSSProperties = {
  border: "1px solid var(--ant-color-border)",
  borderRadius: 8,
  boxShadow: "0 1px 2px rgba(0,0,0,0.06)",
  overflow: "auto",
};

const FRAME_LABEL: React.CSSProperties = {
  position: "sticky",
  top: 0,
  zIndex: 1,
  background: "var(--ant-color-bg-container)",
  padding: "6px 12px",
  borderBottom: "1px solid var(--ant-color-border)",
  fontWeight: 500,
  fontSize: 13,
  letterSpacing: 0.3,
  display: "flex",
  alignItems: "center",
  justifyContent: "space-between",
  gap: 8,
};

/** 带「载入模板 / 清空」工具条的代码字段。模板与清空都是程序化写入，
 * 写入后补触发脏检查；编辑器键入走 Form.Item 注入的 onChange 正常触发。 */
function CodeFieldWithTools({
  form,
  fieldKey,
  language,
  height,
  label,
  placeholder,
  onProgrammaticChange,
}: {
  form: FormInstance<ApplicationConfigFormValues>;
  fieldKey: UiCodeFieldKey;
  language: "html" | "css";
  height: number;
  label: string;
  placeholder?: string;
  onProgrammaticChange: () => void;
}) {
  const { t } = useTranslation("resource");
  const { modal, message } = App.useApp();
  const value = (Form.useWatch(fieldKey, form) as string | undefined) ?? "";

  const apply = (next: string) => {
    form.setFieldValue(fieldKey, next);
    onProgrammaticChange();
  };
  const loadTemplate = () => {
    const template = UI_CODE_TEMPLATES[fieldKey];
    if (!value.trim()) {
      apply(template);
      return;
    }
    modal.confirm({
      title: t("application.styleCode.overwriteTitle"),
      content: t("application.styleCode.overwriteContent"),
      onOk: () => apply(template),
    });
  };

  return (
    <div style={{ marginBottom: 24 }}>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          marginBottom: 6,
          gap: 8,
          flexWrap: "wrap",
        }}
      >
        <Typography.Text strong>{label}</Typography.Text>
        <Space size={8}>
          <Button size="small" onClick={loadTemplate}>
            {t("application.styleCode.loadTemplate")}
          </Button>
          <Button
            size="small"
            disabled={!value.trim()}
            onClick={() => {
              apply("");
              message.info(t("application.styleCode.resetHint"));
            }}
          >
            {t("application.resetToEmpty", "Clear")}
          </Button>
        </Space>
      </div>
      <Form.Item name={fieldKey} noStyle>
        <CodeEditor language={language} height={height} placeholder={placeholder} />
      </Form.Item>
    </div>
  );
}

export function UiCustomizationTab({
  form,
  organizationSlug,
  entityKey,
  onResetToSaved,
  onResetToDefaults,
  onProgrammaticChange,
}: UiCustomizationTabProps) {
  const { t } = useTranslation("resource");
  const { modal } = App.useApp();
  const watch = Form.useWatch(selectPreviewValues, { form, preserve: true });

  const [
    formBackgroundUrl,
    formOffset,
    formCss,
    formSideHtml,
    headerHtml,
    footerHtml,
    customCss,
    formBackgroundUrlMobile,
    formCssMobile,
    signinHtml,
    signupHtml,
    displayName,
    description,
    logo,
    rawSigninItems,
    rawSignupItems,
  ] = watch;

  const history = useUiConfigHistory(form, UI_CUSTOMIZATION_FIELDS, {
    entityKey,
    onApply: onProgrammaticChange,
  });

  // 与真实登录页 getSigninItems / 注册页 getSignupItems 共用同一合并策略，
  // 预览帧总有必需项，且不再手工复刻合并逻辑。
  const mergedSigninItems = useMemo(
    () =>
      getSigninItems({
        signinItems: rawSigninItems as unknown as SigninItemConfig[] | null,
      }),
    [rawSigninItems],
  );
  const mergedSignupItems = useMemo(
    () => getSignupItems({ signupItems: rawSignupItems }),
    [rawSignupItems],
  );

  const [signupBreakpoint, setSignupBreakpoint] = useState<PreviewBreakpoint>("desktop");
  const [signinBreakpoint, setSigninBreakpoint] = useState<PreviewBreakpoint>("mobile");
  const [fullscreen, setFullscreen] = useState(false);

  const applicationSlug = Form.useWatch("slug", form) ?? "";
  const accessUrlContext = {
    origin: window.location.origin,
    organizationSlug,
    applicationSlug,
  };
  const signinUrl = effectiveAccessUrl(
    Form.useWatch("signinUrl", form),
    defaultSigninUrl(accessUrlContext),
  );
  const signupUrl = effectiveAccessUrl(
    Form.useWatch("signupUrl", form),
    defaultSignupUrl(accessUrlContext),
  );

  const setUiField = (key: keyof ApplicationConfigFormValues, value: unknown) => {
    form.setFieldValue(key, value);
    onProgrammaticChange();
  };

  const confirmReset = (mode: "saved" | "defaults") => {
    modal.confirm({
      title: t(
        mode === "saved"
          ? "application.uiCustomization.resetSavedTitle"
          : "application.uiCustomization.resetDefaultsTitle",
      ),
      content: t(
        mode === "saved"
          ? "application.uiCustomization.resetSavedContent"
          : "application.uiCustomization.resetDefaultsContent",
      ),
      onOk: mode === "saved" ? onResetToSaved : onResetToDefaults,
    });
  };

  const breakpointOptions = [
    { label: t("application.previewDesktop", "Desktop"), value: "desktop" },
    { label: t("application.previewMobile", "Mobile"), value: "mobile" },
  ];

  const renderPreviewFrames = (frameHeight: number) => (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: "repeat(auto-fit, minmax(320px, 1fr))",
        gap: 16,
      }}
    >
      <div style={{ ...FRAME_BORDER, height: frameHeight }}>
        <div style={FRAME_LABEL}>
          <span>{t("application.signupPreview", "注册预览")}</span>
          <Segmented
            size="small"
            options={breakpointOptions}
            value={signupBreakpoint}
            onChange={(v) => setSignupBreakpoint(v as PreviewBreakpoint)}
          />
        </div>
        <SignupPreviewFrame
          signupItems={mergedSignupItems}
          compactBreakpoint={signupBreakpoint}
          formBackgroundUrl={formBackgroundUrl}
          formOffset={formOffset}
          formCss={formCss}
          formSideHtml={formSideHtml}
          headerHtml={headerHtml}
          footerHtml={footerHtml}
          customCss={customCss}
          formBackgroundUrlMobile={formBackgroundUrlMobile}
          formCssMobile={formCssMobile}
          signupHtml={signupHtml}
          applicationName={displayName ?? undefined}
          applicationDescription={description ?? undefined}
        />
      </div>

      <div style={{ ...FRAME_BORDER, height: frameHeight }}>
        <div style={FRAME_LABEL}>
          <span>{t("application.signinPreview", "登录预览")}</span>
          <Segmented
            size="small"
            options={breakpointOptions}
            value={signinBreakpoint}
            onChange={(v) => setSigninBreakpoint(v as PreviewBreakpoint)}
          />
        </div>
        <LoginPreviewPanel
          compact
          compactBreakpoint={signinBreakpoint}
          formBackgroundUrl={formBackgroundUrl}
          formOffset={formOffset}
          formCss={formCss}
          formSideHtml={formSideHtml}
          headerHtml={headerHtml}
          footerHtml={footerHtml}
          customCss={customCss}
          formBackgroundUrlMobile={formBackgroundUrlMobile}
          formCssMobile={formCssMobile}
          signinHtml={signinHtml}
          applicationName={displayName ?? undefined}
          applicationDescription={description ?? undefined}
          logoUrl={logo ?? undefined}
          signinItems={mergedSigninItems as unknown as ApplicationConfigFormValues["signinItems"]}
        />
      </div>
    </div>
  );

  return (
    <>
      {/* 工具条：撤销 / 重做 / 整组重置 */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 12,
          flexWrap: "wrap",
          marginBottom: 16,
        }}
      >
        <Space size={8} wrap>
          <Button
            size="small"
            icon={<UndoOutlined />}
            disabled={!history.canUndo}
            onClick={history.undo}
          >
            {t("application.uiCustomization.undo", "Undo")}
          </Button>
          <Button
            size="small"
            icon={<RedoOutlined />}
            disabled={!history.canRedo}
            onClick={history.redo}
          >
            {t("application.uiCustomization.redo", "Redo")}
          </Button>
          <Dropdown
            menu={{
              items: [
                {
                  key: "saved",
                  label: t("application.uiCustomization.resetToSaved", "Restore saved values"),
                },
                {
                  key: "defaults",
                  label: t("application.uiCustomization.resetToDefaults", "Reset to defaults"),
                },
              ],
              onClick: ({ key }) => confirmReset(key as "saved" | "defaults"),
            }}
          >
            <Button size="small">
              {t("application.uiCustomization.resetGroup", "Reset customization")}
            </Button>
          </Dropdown>
        </Space>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {t("application.styleCode.urlHint")}
        </Typography.Text>
      </div>
      <div style={{ marginBottom: 16, wordBreak: "break-all" }}>
        <Typography.Text code copyable>
          {signinUrl}
        </Typography.Text>
        <br />
        <Typography.Text code copyable>
          {signupUrl}
        </Typography.Text>
      </div>

      {/* 1. 基本设置（orgChoiceMode 唯一入口在「认证」Tab，此处不再重复配置） */}
      <FormSection title={t("application.basicSettings", "基本设置")}>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={12}>
            <Form.Item label={t("application.formPosition")} name="formOffset">
              <Segmented
                options={[
                  { label: t("application.positionTopLeft"), value: "1" },
                  { label: t("application.positionCenter"), value: "2" },
                  { label: t("application.positionBottomCenter"), value: "3" },
                  { label: t("application.positionSplit"), value: "4" },
                ]}
              />
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      {/* 2. 登录项 */}
      <FormSection title={t("application.signinItems")}>
        <SigninItemsEditor
          value={
            (rawSigninItems?.length
              ? rawSigninItems
              : mergedSigninItems) as unknown as ApplicationConfigFormValues["signinItems"]
          }
          onChange={(items) => setUiField("signinItems", items)}
        />
      </FormSection>

      {/* 3. 注册项 */}
      <FormSection title={t("application.signupItemsSection")}>
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          {t("application.signupItemsDesc")}
        </Typography.Paragraph>
        <Form.Item
          name="signupItems"
          rules={[
            {
              validator: async (_rule, value: ApplicationConfigFormValues["signupItems"]) => {
                const items = value ?? [];
                const names = items.map((item) => (item.name ?? "").trim());
                if (names.some((name) => !name)) {
                  throw new Error(t("application.signupItems.validation.missing"));
                }
                const duplicate = names.find((name, idx) => names.indexOf(name) !== idx);
                if (duplicate) {
                  throw new Error(
                    t("application.signupItems.validation.duplicate", { name: duplicate }),
                  );
                }
              },
            },
          ]}
        >
          <SignupItemsEditorInput />
        </Form.Item>
        <Typography.Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>
          {t(
            "application.signupItemsCaptchaHint",
            "「Captcha」注册项决定该应用的注册页是否启用图形验证码：可见 = 启用，不可见（visible=false）= 对该应用关闭。实际验证码组件来自组织已启用的验证码供应商，两者缺一不可；未配置供应商时即使启用该注册项也不会展示验证码。",
          )}
        </Typography.Paragraph>
      </FormSection>

      {/* 4. 实时预览（双帧并排 + 断点切换 + 全屏） */}
      <FormSection
        title={t("application.previewLive", "实时预览")}
        extra={
          <Button size="small" icon={<ExpandOutlined />} onClick={() => setFullscreen(true)}>
            {t("application.uiCustomization.fullscreen", "Fullscreen")}
          </Button>
        }
      >
        {renderPreviewFrames(560)}
      </FormSection>
      <Modal
        open={fullscreen}
        onCancel={() => setFullscreen(false)}
        footer={null}
        width="min(1200px, 96vw)"
        title={t("application.previewLive", "实时预览")}
        styles={{ body: { maxHeight: "82vh", overflow: "auto" } }}
        destroyOnHidden
      >
        {renderPreviewFrames(640)}
      </Modal>

      {/* 5. 背景与位置 */}
      <FormSection title={t("application.backgroundImages")}>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={12}>
            <Form.Item label={t("application.backgroundImageUrl")} name="formBackgroundUrl">
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder="https://example.com/bg.jpg"
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={12}>
            <Form.Item
              label={t("application.backgroundImageMobile")}
              name="formBackgroundUrlMobile"
            >
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder="https://example.com/bg-mobile.jpg"
              />
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      {/* 6. 表单样式 */}
      <FormSection title={t("application.loginFormStyling")}>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={12}>
            <CodeFieldWithTools
              form={form}
              fieldKey="formCss"
              language="css"
              height={150}
              label={t("application.formCss")}
              placeholder=".login-form { max-width: 400px; }"
              onProgrammaticChange={onProgrammaticChange}
            />
          </Col>
          <Col xs={24} sm={12} md={12}>
            <CodeFieldWithTools
              form={form}
              fieldKey="formCssMobile"
              language="css"
              height={150}
              label={t("application.formCssMobile")}
              placeholder=".login-form { max-width: 100%; }"
              onProgrammaticChange={onProgrammaticChange}
            />
          </Col>
        </Row>
        <CodeFieldWithTools
          form={form}
          fieldKey="formSideHtml"
          language="html"
          height={120}
          label={t("application.formSideHtml")}
          placeholder="<div>Side panel content</div>"
          onProgrammaticChange={onProgrammaticChange}
        />
      </FormSection>

      {/* 7. 自定义 HTML/CSS */}
      <FormSection title={t("application.customHtmlCss")}>
        <CodeFieldWithTools
          form={form}
          fieldKey="headerHtml"
          language="html"
          height={120}
          label={t("application.headerHtml")}
          placeholder="<nav>Custom header</nav>"
          onProgrammaticChange={onProgrammaticChange}
        />
        <CodeFieldWithTools
          form={form}
          fieldKey="footerHtml"
          language="html"
          height={120}
          label={t("application.footerHtml")}
          placeholder="<footer>Custom footer</footer>"
          onProgrammaticChange={onProgrammaticChange}
        />
        <div style={{ marginTop: -16, marginBottom: 24, textAlign: "right" }}>
          <Button size="small" onClick={() => setUiField("footerHtml", DEFAULT_FOOTER_HTML)}>
            {t("application.resetToDefault", "Reset to Default")}
          </Button>
        </div>
        <Form.Item label={t("application.signupPageHtml")} name="signupHtml">
          <CodeEditor language="html" height={200} placeholder={t("common:customSignupPage")} />
        </Form.Item>
        <Form.Item label={t("application.signinPageHtml")} name="signinHtml">
          <CodeEditor language="html" height={200} placeholder={t("common:customSigninPage")} />
        </Form.Item>
        <CodeFieldWithTools
          form={form}
          fieldKey="customCss"
          language="css"
          height={150}
          label={t("application.customCss")}
          placeholder=".header { font-weight: 700; }"
          onProgrammaticChange={onProgrammaticChange}
        />
        <Form.Item
          label={t("application.customHeader")}
          name="customHeader"
          extra={t("application.uiCustomization.noScriptHint")}
        >
          <CodeEditor
            language="html"
            height={150}
            placeholder='<meta name="description" content="My app">'
          />
        </Form.Item>
      </FormSection>

      {/* 8. 导航策略 URL */}
      <FormSection title={t("application.navPolicyUrls")}>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={t("application.logoutUrl")} name="logoutUrl">
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder="https://example.com/logout"
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={t("application.logoutRedirectUrl")} name="logoutRedirectUrl">
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder="https://example.com"
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={t("application.termsOfUse")} name="termsOfUse">
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder="https://example.com/terms"
              />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={12}>
            <Form.Item label={t("application.frontchannelLogoutUri")} name="frontchannelLogoutUri">
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder="https://example.com/frontchannel-logout"
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={12}>
            <Form.Item
              label={t("application.frontchannelLogoutSessionRequired")}
              name="frontchannelLogoutSessionRequired"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={12}>
            <Form.Item label={t("application.privacyPolicy")} name="privacyPolicy">
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder="https://example.com/privacy"
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={12}>
            <Form.Item label={t("application.backchannelLogoutUri")} name="backchannelLogoutUri">
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder="https://example.com/backchannel-logout"
              />
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      {/* 9. 自定义 URL */}
      <FormSection title={t("application.customUrlsSection")}>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={t("application.signinUrl")} name="signinUrl">
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder={t("application.placeholder.signinUrl")}
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={t("application.signupUrl")} name="signupUrl">
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder={t("application.placeholder.signupUrl")}
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={t("application.forgetUrl")} name="forgetUrl">
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder={t("application.placeholder.forgetUrl")}
              />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={12}>
            <Form.Item label={t("application.affiliationUrl")} name="affiliationUrl">
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder={t("application.placeholder.affiliationUrl")}
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={12}>
            <Form.Item label={t("application.promptUrl")} name="promptUrl">
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder={t("application.placeholder.promptUrl")}
              />
            </Form.Item>
          </Col>
        </Row>
      </FormSection>
    </>
  );
}
