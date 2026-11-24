/**
 * 组织认证与安全分区 —— 创建页与详情页共用。
 *
 * 创建接口只接收基础开关；访问控制、MFA、会话与风险策略仅在详情页渲染。
 */
import { Col, Form, Input, InputNumber, Row, Switch } from "antd";
import { useTranslation } from "react-i18next";
import { GlFormRow } from "@/shared/ui/GlForm";
import { StructuredJsonField } from "../StructuredJsonEditor";
import { MfaItemsEditor } from "./MfaItemsEditor";
import type { OrganizationSectionMode } from "./types";

type OrganizationSecurityFieldsProps = {
  mode: OrganizationSectionMode;
  /**
   * 详情页把安全字段拆成三个业务分区：
   * - access: 登录开关、访问控制
   * - mfa: 多因素认证
   * - session: 会话与风险策略
   * 创建页不传，保持一次性渲染全部（后端创建只接受基础开关）。
   */
  section?: "access" | "mfa" | "session" | "all";
};

export function OrganizationSecurityFields({
  mode,
  section = "all",
}: OrganizationSecurityFieldsProps) {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const editable = mode === "edit";
  const showAccess = section === "all" || section === "access";
  const showMfa = section === "all" || section === "mfa";
  const showSession = section === "all" || section === "session";

  return (
    <Row gutter={16}>
      {showAccess && (
        <>
          <Col xs={24} md={8}>
            <Form.Item name="disableSignin" valuePropName="checked">
              <GlFormRow label={tr("organization.disableSignin")} inline>
                <Switch />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="enableTour" valuePropName="checked">
              <GlFormRow label={tr("organization.enableTour")} inline>
                <Switch />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="useEmailAsUsername" valuePropName="checked">
              <GlFormRow label={tr("organization.useEmailAsUsername")} inline>
                <Switch />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="isProfilePublic" valuePropName="checked">
              <GlFormRow label={tr("organization.isProfilePublic")} inline>
                <Switch />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="hasPrivilegeConsent" valuePropName="checked">
              <GlFormRow label={tr("organization.hasPrivilegeConsent")} inline>
                <Switch />
              </GlFormRow>
            </Form.Item>
          </Col>
          {editable && (
            <>
              <Col xs={24} md={8}>
                <Form.Item name="enableSoftDeletion" valuePropName="checked">
                  <GlFormRow label={tr("organization.enableSoftDeletion")} inline>
                    <Switch />
                  </GlFormRow>
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item name="ipWhitelist">
                  <GlFormRow label={tr("organization.ipWhitelist")}>
                    <Input placeholder="e.g. 10.0.0.0/8, 192.168.1.0/24" />
                  </GlFormRow>
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item name="initiateLoginUrl">
                  <GlFormRow label={tr("organization.initiateLoginUrl")}>
                    <Input />
                  </GlFormRow>
                </Form.Item>
              </Col>
            </>
          )}
        </>
      )}

      {showMfa && editable && (
        <>
          <Col xs={24} md={12}>
            <Form.Item name="mfaRequired" valuePropName="checked">
              <GlFormRow
                label={tr("organization.mfaRequired")}
                extra={tr("organization.mfaRequiredHint")}
                inline
              >
                <Switch />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item name="mfaRememberInHours">
              <GlFormRow
                label={tr("organization.mfaRememberInHours")}
                extra={tr("organization.mfaRememberHint")}
              >
                <InputNumber min={0} max={8760} style={{ width: "100%" }} suffix={t("hours")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col span={24}>
            <StructuredJsonField
              name="mfaPolicyExtraJson"
              label={tr("organization.mfaPolicyExtraJson")}
              rows={4}
            />
          </Col>
          <Col span={24}>
            <MfaItemsEditor />
          </Col>
        </>
      )}

      {showSession && editable && (
        <>
          <Col xs={24} md={12}>
            <StructuredJsonField
              name="ipPolicyJson"
              label={tr("organization.ipPolicyJson")}
              rows={4}
            />
          </Col>
          <Col xs={24} md={12}>
            <StructuredJsonField
              name="sessionPolicyExtraJson"
              label={tr("organization.sessionPolicyExtraJson")}
              rows={4}
            />
          </Col>
          <Col xs={24} md={12}>
            <Form.Item
              name="sessionIdleTimeoutMinutes"
              label={tr("organization.sessionIdleTimeoutMinutes")}
            >
              <InputNumber min={1} style={{ width: "100%" }} />
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <StructuredJsonField
              name="riskPolicyJson"
              label={tr("organization.riskPolicyJson")}
              rows={4}
            />
          </Col>
        </>
      )}
    </Row>
  );
}
