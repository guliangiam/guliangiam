import { Col, Form, Input, InputNumber, Row, Select, Switch } from "antd";
import type { FormInstance } from "antd";
import { useTranslation } from "react-i18next";
import { FormSection } from "@/shared/ui/FormSection";
import type { ApplicationConfigFormValues } from "./types";

type SecurityTabProps = {
  form: FormInstance<ApplicationConfigFormValues>;
};

export function SecurityTab({ form: _form }: SecurityTabProps) {
  void _form;
  const { t: tr } = useTranslation("resource");

  return (
    <>
      <FormSection title={tr("application.oauthTokenSecuritySection", "OAuth / Token Security")}>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={8}>
            <Form.Item label={tr("application.consentMode", "Consent Mode")} name="consentMode">
              <Select
                options={[
                  {
                    label: tr("application.consentModeOption.explicit", "Explicit"),
                    value: "explicit",
                  },
                  {
                    label: tr("application.consentModeOption.implicit", "Implicit"),
                    value: "implicit",
                  },
                  { label: tr("application.consentModeOption.none", "None"), value: "none" },
                ]}
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={4}>
            <Form.Item
              label={tr("application.dpopRequired", "DPoP Required")}
              name="dpopRequired"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
          {/*
            PKCE is mandatory for every client (OAuth 2.1, ADR-004), so the
            per-application switch no longer controls anything: the backend
            ignores the flag and refuses any authorization request without an
            S256 challenge. The field stays registered but hidden so editing an
            application does not silently reset its stored value.
          */}
          <Form.Item
            label={tr("application.pkceRequired", "PKCE Required")}
            name="pkceRequired"
            hidden
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Col xs={24} sm={12} md={4}>
            <Form.Item
              label={tr("application.enableBetaOidc", "Beta OIDC")}
              name="enableBetaOidc"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      <FormSection title={tr("application.ipRestrictionsSection", "IP Restrictions")}>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={12}>
            <Form.Item
              label={tr("application.ipWhitelist", "IP Whitelist")}
              name="ipWhitelist"
              extra={tr("application.ipListExtra", "Comma-separated IPs or CIDR ranges")}
            >
              <Input.TextArea
                autoComplete="off"
                spellCheck={false}
                rows={3}
                placeholder="192.168.1.0/24, 10.0.0.1"
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={12}>
            <Form.Item
              label={tr("application.ipBlacklist", "IP Blacklist")}
              name="ipBlacklist"
              extra={tr("application.ipListExtra", "Comma-separated IPs or CIDR ranges")}
            >
              <Input.TextArea
                autoComplete="off"
                spellCheck={false}
                rows={3}
                placeholder="10.0.0.0/8"
              />
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      <FormSection
        title={tr("application.loginFailureProtectionSection", "Login Failure Protection")}
      >
        <Row gutter={16}>
          <Col xs={24} sm={12} md={8}>
            <Form.Item
              label={tr("application.failedSigninLimit", "Failed Signin Limit")}
              name="failedSigninLimit"
              extra={tr("application.failedSigninLimitExtra", "Max failed attempts before freeze")}
            >
              <InputNumber style={{ width: "100%" }} min={0} placeholder="5" />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item
              label={tr("application.failedSigninFrozenTime", "Frozen Time (min)")}
              name="failedSigninFrozenTime"
              extra={tr(
                "application.failedSigninFrozenTimeExtra",
                "Minutes to freeze after limit reached",
              )}
            >
              <InputNumber style={{ width: "100%" }} min={0} placeholder="15" />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item
              label={tr("application.codeResendTimeout", "Code Resend Timeout (s)")}
              name="codeResendTimeout"
              extra={tr(
                "application.codeResendTimeoutExtra",
                "Seconds before allowing code resend",
              )}
            >
              <InputNumber style={{ width: "100%" }} min={0} placeholder="60" />
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      <FormSection title={tr("application.corsAdvancedSection", "CORS & Advanced")}>
        <Form.Item
          label={tr("application.corsAllowedOrigins", "CORS Allowed Origins")}
          name="corsAllowedOrigins"
          extra={tr("application.corsOriginsExtra", "Comma-separated origins")}
        >
          <Input autoComplete="off" placeholder="https://app1.com, https://app2.com" />
        </Form.Item>
      </FormSection>
    </>
  );
}
