import type { FormInstance } from "antd";
import { Col, Form, Input, Row, Select, Space, Switch, Tag } from "antd";
import { useTranslation } from "react-i18next";
import { FormSection } from "@/shared/ui/FormSection";
import { ImageCropUpload } from "@/shared/ui/ImageCropUpload";
import { ResetContactModal } from "@/shared/ui/ResetContactModal";
import { getAccountStatusOptions, getUserTypeOptions } from "./types";
import type { UserDetailFormValues } from "./types";
import {
  isFieldVisible,
  isFieldDisabled,
  getFieldRegex,
  type AccountItemsMap,
} from "@/features/account-items/useAccountItems";

interface UserBasicTabProps {
  form: FormInstance<UserDetailFormValues>;
  owner?: string;
  /** Human-readable organization name; `owner` keeps carrying the UUID. */
  organizationName?: string;
  name?: string;
  userId?: string;
  organizationId?: string;
  currentCountryCode?: string;
  fieldPermissions?: AccountItemsMap;
  /** 联系方式重置/验证成功后回调：刷新页面数据以同步 verified 标记。 */
  onContactUpdated?: () => void;
}

export default function UserBasicTab({
  form,
  owner,
  organizationName,
  name,
  userId,
  organizationId,
  currentCountryCode,
  fieldPermissions = {},
  onContactUpdated,
}: UserBasicTabProps) {
  const { t } = useTranslation();

  // 验证状态直接跟随表单值（保存/重置后由页面回填最新标记）。
  const emailVerified = Form.useWatch("emailVerified", form);
  const phoneVerified = Form.useWatch("phoneVerified", form);

  // i18n-aware option constants
  const ACCOUNT_STATUS_OPTIONS = getAccountStatusOptions(t);
  const USER_TYPE_OPTIONS = getUserTypeOptions(t);

  const fp = fieldPermissions;

  return (
    <>
      <FormSection title={t("resource:user.profileSection")}>
        <Row gutter={16}>
          {isFieldVisible(fp, "Organization") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item label={t("organization")} name="owner">
                {/* 表单值仍是组织 UUID；仅把展示换成组织名，避免暴露不可读 id。 */}
                <Select
                  disabled
                  options={owner ? [{ value: owner, label: organizationName || owner }] : []}
                />
              </Form.Item>
            </Col>
          )}
          {isFieldVisible(fp, "Name") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item label={t("name")} name="name">
                <Input disabled />
              </Form.Item>
            </Col>
          )}
        </Row>
        <Row gutter={16}>
          {isFieldVisible(fp, "Display name") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item
                label={t("displayName")}
                name="displayName"
                rules={[
                  {
                    required: true,
                    whitespace: true,
                    message: t("resource:user.validation.displayNameRequired"),
                  },
                  ...(getFieldRegex(fp, "Display name")
                    ? [
                        {
                          pattern: new RegExp(getFieldRegex(fp, "Display name")!),
                          message: t("resource:form.invalidFormat"),
                        },
                      ]
                    : []),
                ]}
              >
                <Input disabled={isFieldDisabled(fp, "Display name")} />
              </Form.Item>
            </Col>
          )}
          {isFieldVisible(fp, "ID") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item label={t("resource:user.externalId")} name="externalId">
                <Input disabled={isFieldDisabled(fp, "ID")} />
              </Form.Item>
            </Col>
          )}
        </Row>
        <Row gutter={16}>
          {isFieldVisible(fp, "Email") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item
                label={
                  <Space size={4}>
                    {t("email")}
                    <Tag
                      color={emailVerified ? "success" : "warning"}
                      style={{ marginInlineEnd: 0 }}
                    >
                      {emailVerified
                        ? t("resource:user.verification.verified", "Verified")
                        : t("resource:user.verification.unverified", "Unverified")}
                    </Tag>
                  </Space>
                }
                name="email"
                rules={
                  getFieldRegex(fp, "Email")
                    ? [
                        {
                          pattern: new RegExp(getFieldRegex(fp, "Email")!),
                          message: t("resource:form.invalidFormat"),
                        },
                      ]
                    : [{ type: "email", message: t("common:form.invalidEmail") }]
                }
              >
                <Input
                  type="email"
                  disabled={isFieldDisabled(fp, "Email")}
                  // antd v6 弃用 addonAfter；重置入口放进 suffix，保持 Form.Item
                  // 只有一个受控子元素（否则 value/onChange 无法注入）。
                  suffix={
                    userId ? (
                      <ResetContactModal
                        userId={userId}
                        organizationId={organizationId}
                        mode="email"
                        disabled={isFieldDisabled(fp, "Email")}
                        onSuccess={onContactUpdated}
                      />
                    ) : undefined
                  }
                />
              </Form.Item>
            </Col>
          )}
          {isFieldVisible(fp, "Phone") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item
                label={
                  <Space size={4}>
                    {t("resource:user.phone")}
                    <Tag
                      color={phoneVerified ? "success" : "warning"}
                      style={{ marginInlineEnd: 0 }}
                    >
                      {phoneVerified
                        ? t("resource:user.verification.verified", "Verified")
                        : t("resource:user.verification.unverified", "Unverified")}
                    </Tag>
                  </Space>
                }
                name="phone"
                rules={
                  getFieldRegex(fp, "Phone")
                    ? [
                        {
                          pattern: new RegExp(getFieldRegex(fp, "Phone")!),
                          message: t("resource:form.invalidFormat"),
                        },
                      ]
                    : [
                        {
                          pattern: /^[+]?[\d\s\-()]{6,20}$/,
                          message: t("common:form.invalidPhone"),
                        },
                      ]
                }
              >
                <Input
                  disabled={isFieldDisabled(fp, "Phone")}
                  suffix={
                    userId ? (
                      <ResetContactModal
                        userId={userId}
                        organizationId={organizationId}
                        mode="phone"
                        currentCountryCode={currentCountryCode}
                        disabled={isFieldDisabled(fp, "Phone")}
                        onSuccess={onContactUpdated}
                      />
                    ) : undefined
                  }
                />
              </Form.Item>
            </Col>
          )}
        </Row>
        <Row gutter={16}>
          {isFieldVisible(fp, "First name") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item label={t("resource:user.firstName")} name="firstName">
                <Input disabled={isFieldDisabled(fp, "First name")} />
              </Form.Item>
            </Col>
          )}
          {isFieldVisible(fp, "Last name") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item label={t("resource:user.lastName")} name="lastName">
                <Input disabled={isFieldDisabled(fp, "Last name")} />
              </Form.Item>
            </Col>
          )}
        </Row>
        <Row gutter={16}>
          {isFieldVisible(fp, "Avatar") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item label={t("resource:user.avatar")}>
                <ImageCropUpload
                  value={form.getFieldValue("avatarUrl")}
                  onChange={(url) => form.setFieldValue("avatarUrl", url)}
                  tag="avatar"
                  owner={owner}
                  name={name}
                  shape="circle"
                  label={t("uploadPhoto")}
                  disabled={isFieldDisabled(fp, "Avatar")}
                />
              </Form.Item>
            </Col>
          )}
          {isFieldVisible(fp, "Affiliation") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item label={t("affiliation")} name="affiliation">
                {/* affiliation 是 profile.properties 里的自由文本（院系/部门名称），
                    不是组织外键；用文本输入而不是复用组织选择器。 */}
                <Input
                  placeholder={t("resource:user.placeholder.affiliation")}
                  disabled={isFieldDisabled(fp, "Affiliation")}
                />
              </Form.Item>
            </Col>
          )}
        </Row>
        <Row gutter={16}>
          {isFieldVisible(fp, "Title") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item
                label={t("resource:user.jobTitle")}
                name="title"
                rules={
                  getFieldRegex(fp, "Title")
                    ? [
                        {
                          pattern: new RegExp(getFieldRegex(fp, "Title")!),
                          message: t("resource:form.invalidFormat"),
                        },
                      ]
                    : undefined
                }
              >
                <Input
                  placeholder={t("resource:user.placeholder.title")}
                  disabled={isFieldDisabled(fp, "Title")}
                />
              </Form.Item>
            </Col>
          )}
        </Row>
        <Row gutter={16}>
          {isFieldVisible(fp, "User type") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item label={t("userType")} name="userType">
                <Select options={USER_TYPE_OPTIONS} disabled={isFieldDisabled(fp, "User type")} />
              </Form.Item>
            </Col>
          )}
          {isFieldVisible(fp, "Status") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item label={t("resource:user.status")} name="accountStatus">
                <Select options={ACCOUNT_STATUS_OPTIONS} disabled={isFieldDisabled(fp, "Status")} />
              </Form.Item>
            </Col>
          )}
          {/* 管理员开关从「账号与权限」合并至此：与 userType 同属账号身份，
              避免两处都能改管理员属性。 */}
          {isFieldVisible(fp, "User type") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item
                label={t("isAdmin", "Administrator")}
                name="isAdmin"
                valuePropName="checked"
                extra={t("resource:user.adminHint", {
                  defaultValue:
                    "Administrators bypass role checks and can manage every tenant resource.",
                })}
              >
                <Switch disabled={isFieldDisabled(fp, "User type")} />
              </Form.Item>
            </Col>
          )}
        </Row>
        <Row gutter={16}>
          {isFieldVisible(fp, "Tag") && (
            <Col xs={24} sm={12} md={12}>
              <Form.Item label={t("resource:user.tag")} name="tag">
                <Input disabled={isFieldDisabled(fp, "Tag")} />
              </Form.Item>
            </Col>
          )}
        </Row>
      </FormSection>
    </>
  );
}
