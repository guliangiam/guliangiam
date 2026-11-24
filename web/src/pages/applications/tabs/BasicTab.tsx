import {
  App,
  Button,
  Col,
  Form,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
} from "antd";
import type { FormInstance } from "antd";
import { useTranslation } from "react-i18next";
import { CopyOutlined, LinkOutlined } from "@ant-design/icons";
import { FormSection } from "@/shared/ui/FormSection";
import { ImageCropUpload } from "@/shared/ui/ImageCropUpload";
import type { ApplicationConfigFormValues } from "./types";
import {
  defaultSigninUrl,
  defaultSignupUrl,
  effectiveAccessUrl,
  isCustomAccessUrl,
} from "../application-urls";

type BasicTabProps = {
  form: FormInstance<ApplicationConfigFormValues>;
  isNew: boolean;
  existingApplicationSlugs: Set<string>;
  isSlugValidationLoading: boolean;
  owner?: string;
  appName?: string;
  /** 当前应用所属组织的 slug，用于拼默认登录/注册地址。 */
  organizationSlug?: string;
};

function AccessUrlRow({
  label,
  hint,
  value,
  custom,
  disabled,
  onCopy,
}: {
  label: string;
  hint: string;
  value: string;
  custom: boolean;
  disabled: boolean;
  onCopy: (value: string) => void;
}) {
  const { t: tr } = useTranslation("resource");
  return (
    <Space orientation="vertical" size={4} style={{ width: "100%" }}>
      <Space size={8} wrap>
        <Typography.Text strong>{label}</Typography.Text>
        <Tag color={custom ? "gold" : "blue"}>
          {custom ? tr("application.accessUrl.custom") : tr("application.accessUrl.default")}
        </Tag>
      </Space>
      <Space.Compact style={{ width: "100%" }}>
        <Input readOnly value={value} prefix={<LinkOutlined />} />
        <Button icon={<CopyOutlined />} onClick={() => onCopy(value)} disabled={disabled || !value}>
          {tr("application.accessUrl.copy")}
        </Button>
      </Space.Compact>
      <Typography.Text type="secondary">{hint}</Typography.Text>
    </Space>
  );
}

export function BasicTab({
  form,
  isNew,
  existingApplicationSlugs,
  isSlugValidationLoading,
  owner,
  appName,
  organizationSlug,
}: BasicTabProps) {
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const watchedSlug = Form.useWatch("slug", form) ?? "";
  const watchedSigninUrl = Form.useWatch("signinUrl", form) ?? "";
  const watchedSignupUrl = Form.useWatch("signupUrl", form) ?? "";

  const origin = typeof window === "undefined" ? "" : window.location.origin;
  const context = {
    origin,
    organizationSlug: (organizationSlug ?? "").trim(),
    // 表单里的 slug 实时优先（编辑后复制到的地址应立即反映新值）。
    applicationSlug: (watchedSlug || appName || "").trim(),
  };
  const signinUrl = effectiveAccessUrl(watchedSigninUrl, defaultSigninUrl(context));
  const signupUrl = effectiveAccessUrl(watchedSignupUrl, defaultSignupUrl(context));
  const canCopy = !isNew && context.organizationSlug !== "" && context.applicationSlug !== "";

  const handleCopy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      message.success(tr("application.copied"));
    } catch {
      message.error(tr("application.copyFailed"));
    }
  };

  return (
    <>
      <FormSection title={tr("application.generalSection", "General")}>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={12}>
            <Form.Item
              label={tr("application.slug")}
              name="slug"
              tooltip={tr(
                "application.slugTooltip",
                "Unique application identifier used in callback URLs and OIDC discovery.",
              )}
              rules={[
                {
                  required: true,
                  whitespace: true,
                  message: tr("application.validation.slugRequired"),
                },
                {
                  validator: async (_, value: string | undefined) => {
                    const slug = value?.trim() ?? "";
                    if (slug === "") {
                      return;
                    }
                    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug)) {
                      throw new Error(tr("application.validation.slugInvalid"));
                    }
                    if (existingApplicationSlugs.has(slug.toLowerCase())) {
                      throw new Error(tr("application.validation.slugDuplicate"));
                    }
                  },
                },
              ]}
              extra={
                isSlugValidationLoading
                  ? tr("application.slugValidationLoading")
                  : isNew
                    ? tr("application.slugExtraNew")
                    : tr("application.slugExtraEdit")
              }
            >
              <Input
                autoComplete="off"
                spellCheck={false}
                placeholder={tr("application.placeholder.slug")}
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={12}>
            <Form.Item
              label={tr("application.displayName")}
              name="displayName"
              tooltip={tr(
                "application.displayNameTooltip",
                "Application name shown on the consent screen.",
              )}
              rules={[
                {
                  required: true,
                  whitespace: true,
                  message: tr("application.validation.displayNameRequired"),
                },
              ]}
            >
              <Input autoComplete="off" placeholder={tr("application.placeholder.displayName")} />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={tr("application.category")} name="category">
              <Select
                allowClear
                placeholder={tr("application.placeholder.category")}
                options={[
                  {
                    label: tr("application.categoryOption.identity", "identity"),
                    value: "identity",
                  },
                  { label: tr("application.categoryOption.web", "web"), value: "web" },
                  { label: tr("application.categoryOption.spa", "spa"), value: "spa" },
                  { label: tr("application.categoryOption.native", "native"), value: "native" },
                  { label: tr("application.categoryOption.machine", "machine"), value: "machine" },
                  { label: tr("application.categoryOption.service", "service"), value: "service" },
                  { label: tr("application.categoryOption.agent", "agent"), value: "agent" },
                ]}
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={tr("application.appType")} name="appType">
              <Select
                allowClear
                placeholder={tr("application.placeholder.appType")}
                options={[
                  { label: tr("application.appTypeOption.Web", "Web"), value: "web" },
                  { label: tr("application.appTypeOption.Native", "Native"), value: "native" },
                  { label: tr("application.appTypeOption.SPA", "SPA"), value: "spa" },
                  { label: tr("application.appTypeOption.Machine", "Machine"), value: "machine" },
                  { label: tr("application.appTypeOption.SAML", "SAML"), value: "saml" },
                  { label: tr("application.appTypeOption.CAS", "CAS"), value: "cas" },
                ]}
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item
              label={tr("application.status")}
              name="status"
              rules={[
                {
                  required: true,
                  whitespace: true,
                  message: tr("application.validation.statusRequired"),
                },
              ]}
            >
              <Select
                placeholder={tr("application.placeholder.status")}
                options={[
                  { label: tr("application.statusOption.active", "Active"), value: "active" },
                  { label: tr("application.statusOption.inactive", "Inactive"), value: "inactive" },
                  {
                    label: tr("application.statusOption.suspended", "Suspended"),
                    value: "suspended",
                  },
                ]}
              />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={12}>
            <Form.Item label={tr("application.logoUrl")} name="logo">
              <ImageCropUpload
                tag="logo"
                owner={owner ?? ""}
                name={appName ?? "app"}
                shape="square"
                allowUrlInput
                label={tr("application.uploadLogo", "Upload Logo")}
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={12}>
            <Form.Item label={tr("application.favicon")} name="favicon">
              <ImageCropUpload
                tag="favicon"
                owner={owner ?? ""}
                name={appName ?? "app"}
                shape="square"
                allowUrlInput
                label={tr("application.uploadFavicon", "Upload Favicon")}
              />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={12}>
            <Form.Item label={tr("application.domain")} name="domain">
              <Input
                autoComplete="off"
                placeholder={tr("application.placeholder.domain")}
                spellCheck={false}
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={12}>
            <Form.Item
              label={tr("application.homepageUrl")}
              name="homepageUrl"
              rules={[
                {
                  validator: (_, value: string | undefined) => {
                    if (!value || value.trim() === "") return Promise.resolve();
                    try {
                      const parsed = new URL(value);
                      if (!parsed.protocol || !parsed.host)
                        return Promise.reject(new Error(tr("application.validation.urlInvalid")));
                      return Promise.resolve();
                    } catch {
                      return Promise.reject(new Error(tr("application.validation.urlInvalid")));
                    }
                  },
                },
              ]}
            >
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder={tr("application.placeholder.homepageUrl")}
              />
            </Form.Item>
          </Col>
        </Row>
        <Form.Item label={tr("application.description")} name="description">
          <Input.TextArea
            autoComplete="off"
            rows={2}
            placeholder={tr("application.placeholder.description")}
          />
        </Form.Item>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={tr("application.title")} name="title">
              <Input autoComplete="off" placeholder={tr("application.placeholder.title")} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={tr("application.order")} name="menuOrder">
              <InputNumber style={{ width: "100%" }} min={0} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={tr("application.menuMode")} name="menuMode">
              <Select
                allowClear
                placeholder={tr("application.placeholder.menuMode")}
                options={[
                  { label: tr("application.menuModeOption.side", "Side"), value: "side" },
                  { label: tr("application.menuModeOption.top", "Top"), value: "top" },
                  { label: tr("application.menuModeOption.mixed", "Mixed"), value: "mixed" },
                ]}
              />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={6}>
            <Form.Item label={tr("application.isShared")} name="isShared" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item label={tr("application.isPortal")} name="isPortal" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item label={tr("application.isVisible")} name="isVisible" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Col>
        </Row>
        <Form.Item label={tr("application.tags")} name="tags">
          <Select
            mode="tags"
            tokenSeparators={[","]}
            placeholder={tr("application.placeholder.tags")}
          />
        </Form.Item>
      </FormSection>

      <FormSection
        title={tr("application.accessUrl.sectionTitle")}
        description={tr("application.accessUrl.sectionDesc")}
      >
        <Row gutter={[16, 16]}>
          <Col xs={24} md={12}>
            <AccessUrlRow
              label={tr("application.accessUrl.signinLabel")}
              hint={tr("application.accessUrl.signinHint")}
              value={signinUrl}
              custom={isCustomAccessUrl(watchedSigninUrl)}
              disabled={!canCopy}
              onCopy={handleCopy}
            />
          </Col>
          <Col xs={24} md={12}>
            <AccessUrlRow
              label={tr("application.accessUrl.signupLabel")}
              hint={tr("application.accessUrl.signupHint")}
              value={signupUrl}
              custom={isCustomAccessUrl(watchedSignupUrl)}
              disabled={!canCopy}
              onCopy={handleCopy}
            />
          </Col>
        </Row>
        <Row gutter={16} style={{ marginTop: 16 }}>
          <Col xs={24} md={12}>
            <Form.Item
              label={tr("application.accessUrl.customSigninLabel")}
              name="signinUrl"
              extra={tr("application.accessUrl.customHint")}
            >
              <Input
                autoComplete="off"
                spellCheck={false}
                placeholder={tr("application.accessUrl.customSigninPlaceholder")}
              />
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item
              label={tr("application.accessUrl.customSignupLabel")}
              name="signupUrl"
              extra={tr("application.accessUrl.customHint")}
            >
              <Input
                autoComplete="off"
                spellCheck={false}
                placeholder={tr("application.accessUrl.customSignupPlaceholder")}
              />
            </Form.Item>
          </Col>
        </Row>
        {!isNew ? (
          <Typography.Text type="secondary">
            {tr("application.accessUrl.savedHint")}
          </Typography.Text>
        ) : null}
      </FormSection>
    </>
  );
}
