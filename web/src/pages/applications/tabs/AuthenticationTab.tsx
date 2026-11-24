import { Alert, Col, Form, InputNumber, Row, Select, Switch, Typography } from "antd";
import type { FormInstance } from "antd";
import { useTranslation } from "react-i18next";
import { SigninMethodTable, getDefaultRule } from "@/shared/ui/SigninMethodTable";
import type { SigninMethodValue } from "@/shared/ui/SigninMethodTable";
import { FormSection } from "@/shared/ui/FormSection";
import { GroupTreeSelect } from "@/shared/ui/GroupTreeSelect";
import type { GroupItem } from "@/shared/api/iam/group";
import type { ApplicationConfigFormValues } from "./types";

/** Bridge component that converts string[] ↔ SigninMethodValue[] for Form integration */
function SigninMethodTableInput({
  value,
  onChange,
}: {
  value?: string[];
  onChange?: (value: string[]) => void;
}) {
  const asObjects: SigninMethodValue[] = (value ?? []).map((name) => ({
    name,
    displayName: name,
    rule: getDefaultRule(name),
  }));
  return (
    <SigninMethodTable
      value={asObjects}
      onChange={(items) => onChange?.(items.map((i) => i.name))}
    />
  );
}

type AuthenticationTabProps = {
  form: FormInstance<ApplicationConfigFormValues>;
  /** 应用所属组织的用户组树，供 defaultGroup 选择（name 口径存储）。 */
  groups?: GroupItem[];
};

export function AuthenticationTab({ form, groups = [] }: AuthenticationTabProps) {
  const { t: tr } = useTranslation("resource");
  const signinMethods = (Form.useWatch("signinMethods", form) ?? []) as string[];

  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        title={tr("application.signinMethodsSummary")}
        description={signinMethods.length > 0 ? signinMethods.join(", ") : "-"}
      />

      <FormSection title={tr("application.signinSettingsSection")}>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.enableSignUp")}
              name="enableSignup"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.enableGuestSignin")}
              name="enableGuestSignin"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.enableExclusiveSignin")}
              name="enableExclusiveSignin"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.enableSigninSession")}
              name="enableSigninSession"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.enableAutoSignin")}
              name="enableAutoSignin"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.enableLinkWithEmail")}
              name="enableLinkWithEmail"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.disableSignin")}
              name="disableSignin"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item label={tr("application.cookieExpireInHours")} name="cookieExpireInHours">
              <InputNumber style={{ width: "100%" }} min={1} />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={tr("application.defaultGroup")} name="defaultGroup">
              <GroupTreeSelect
                groups={groups}
                valueKey="name"
                placeholder={tr("application.placeholder.defaultGroup")}
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={tr("application.orgChoiceMode")} name="orgChoiceMode">
              <Select
                allowClear
                placeholder={tr("application.placeholder.orgChoiceMode", "Select mode")}
                options={[
                  { label: tr("application.orgChoiceModeOption.none", "None"), value: "" },
                  {
                    label: tr("application.orgChoiceModeOption.direct", "Direct"),
                    value: "direct",
                  },
                  {
                    label: tr("application.orgChoiceModeOption.search", "Search"),
                    value: "search",
                  },
                ]}
              />
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      <FormSection title={tr("application.signinMethodsSection")}>
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          {tr(
            "application.signinMethodsDesc",
            "Configure which authentication methods are available for this application and their display rules.",
          )}
        </Typography.Paragraph>
        <Form.Item name="signinMethods" noStyle>
          <SigninMethodTableInput />
        </Form.Item>
      </FormSection>
    </>
  );
}
