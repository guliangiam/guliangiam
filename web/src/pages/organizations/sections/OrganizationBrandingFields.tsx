/**
 * 组织品牌与默认应用分区 —— 创建页与详情页共用。
 *
 * 详情页走 `ImageCropUpload`（受控 value/onChange，由 Form.Item 注入）；
 * 创建页的 slug 尚未落库，上传归属不成立，因此保留 URL 输入。
 */
import { Col, Form, Input, Row, Select, Switch } from "antd";
import { useTranslation } from "react-i18next";
import { ConditionalField } from "@/shared/ui/ConditionalField";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ImageCropUpload } from "@/shared/ui/ImageCropUpload";
import type { ApplicationOption } from "@/shared/api/iam/application";
import type { OrganizationSectionMode } from "./types";

type OrganizationBrandingFieldsProps = {
  mode: OrganizationSectionMode;
  organizationName: string;
  applicationOptions: ApplicationOption[];
  applicationsLoading?: boolean;
  applicationsError?: boolean;
};

export function OrganizationBrandingFields({
  mode,
  organizationName,
  applicationOptions,
  applicationsLoading = false,
  applicationsError = false,
}: OrganizationBrandingFieldsProps) {
  const { t: tr } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const editable = mode === "edit";
  // 设计允许完整 http(s) URL 或本地存储产生的根相对路径 /uploads/...，
  // antd type:"url" 会拒绝相对路径，故用自定义校验。
  const urlRule = {
    validator: (_: unknown, v: string) =>
      !v || /^(https?:\/\/\S+|\/(?!\/)\S*)$/.test(v)
        ? Promise.resolve()
        : Promise.reject(new Error(tr("organization.validation.urlInvalid", {}))),
  };

  return (
    <Row gutter={16}>
      <Col xs={24} md={12}>
        <Form.Item name="logoUrl" rules={[urlRule]}>
          <GlFormRow label={tr("organization.logoUrl")}>
            {editable ? (
              <ImageCropUpload
                tag="logo"
                owner={organizationName}
                name="logo"
                shape="square"
                allowUrlInput
              />
            ) : (
              <Input placeholder={tr("organization.placeholder.logoUrl")} />
            )}
          </GlFormRow>
        </Form.Item>
      </Col>
      <Col xs={24} md={12}>
        <Form.Item name="enableDarkLogo" valuePropName="checked">
          <GlFormRow label={tr("organization.enableDarkLogo")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
      </Col>
      <ConditionalField dependencies={["enableDarkLogo"]} when={(v) => !!v.enableDarkLogo}>
        <Col xs={24} md={12}>
          <Form.Item name="logoDarkUrl" rules={[urlRule]}>
            <GlFormRow label={tr("organization.logoDarkUrl")}>
              {editable ? (
                <ImageCropUpload
                  tag="logo-dark"
                  owner={organizationName}
                  name="logo-dark"
                  shape="square"
                  allowUrlInput
                />
              ) : (
                <Input placeholder="https://example.com/logo-dark.png" />
              )}
            </GlFormRow>
          </Form.Item>
        </Col>
      </ConditionalField>
      <Col xs={24} md={12}>
        <Form.Item name="faviconUrl" rules={[urlRule]}>
          <GlFormRow label={tr("organization.faviconUrl")}>
            {editable ? (
              <ImageCropUpload
                tag="favicon"
                owner={organizationName}
                name="favicon"
                shape="square"
                allowUrlInput
              />
            ) : (
              <Input placeholder="https://example.com/favicon.ico" />
            )}
          </GlFormRow>
        </Form.Item>
      </Col>
      <Col xs={24} md={12}>
        <Form.Item name="defaultAvatar" rules={[urlRule]}>
          <GlFormRow label={tr("organization.defaultAvatar")}>
            {editable ? (
              <ImageCropUpload
                tag="avatar"
                owner={organizationName}
                name="default-avatar"
                allowUrlInput
              />
            ) : (
              <Input placeholder="https://example.com/avatar.png" />
            )}
          </GlFormRow>
        </Form.Item>
      </Col>
      <Col xs={24} md={12}>
        <Form.Item name="defaultApplicationId">
          <GlFormRow label={tr("organization.defaultApplication")}>
            <Select
              allowClear
              showSearch
              optionFilterProp="label"
              loading={applicationsLoading}
              disabled={applicationsError}
              placeholder={
                applicationsError
                  ? tc("loadFailed")
                  : tr("organization.placeholder.defaultApplication", {})
              }
              options={applicationOptions.map((item) => ({
                label: item.displayName ? `${item.displayName} (${item.slug})` : item.slug,
                value: item.id,
              }))}
            />
          </GlFormRow>
        </Form.Item>
      </Col>
      {editable && (
        <Col xs={24} md={12}>
          <Form.Item name="usePermanentAvatar" valuePropName="checked">
            <GlFormRow label={tr("organization.usePermanentAvatar")} inline>
              <Switch />
            </GlFormRow>
          </Form.Item>
        </Col>
      )}
    </Row>
  );
}
