import { Card, Col, DatePicker, Form, Input, Row, Select } from "antd";
import dayjs from "dayjs";
import { useTranslation } from "react-i18next";
import PropertyTable from "@/shared/ui/PropertyTable";
import { CountryCodeSelect } from "@/shared/ui/CountryCodeSelect";
import { getGenderOptions, getLanguageOptions } from "./types";
import type { CustomPropertyRow } from "./types";
import type { CustomAttributeDefinition } from "@/shared/api/iam/user";

interface UserProfileTabProps {
  customProperties: CustomPropertyRow[];
  /** 与「资料」表单共享同一份 state，保存时随 profile 一起提交。 */
  onCustomPropertiesChange: (properties: CustomPropertyRow[]) => void;
  definitions?: CustomAttributeDefinition[];
  /** 无 users:write 时整页只读：控件禁用，避免「能输入但存不进去」。 */
  readOnly?: boolean;
}

export default function UserProfileTab({
  customProperties,
  onCustomPropertiesChange,
  definitions,
  readOnly = false,
}: UserProfileTabProps) {
  const { t } = useTranslation();

  // i18n-aware option constants
  const GENDER_OPTIONS = getGenderOptions(t);
  const LANGUAGE_OPTIONS = getLanguageOptions(t);

  return (
    <>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            label={t("resource:user.homepage")}
            name="homepage"
            rules={[{ type: "url", message: t("common:form.invalidUrl") }]}
          >
            <Input placeholder="https://example.com" disabled={readOnly} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item label={t("resource:user.education")} name="education">
            <Input placeholder={t("resource:user.placeholder.education")} disabled={readOnly} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item label={t("language")} name="language">
            <Select options={LANGUAGE_OPTIONS} allowClear disabled={readOnly} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item label={t("resource:user.countryCode")} name="countryCode">
            <CountryCodeSelect
              style={{ width: "100%" }}
              disabled={readOnly}
              id="user-profile-country-code"
              ariaLabel={t("resource:user.countryCode")}
            />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            label={t("resource:user.province", "Province / State")}
            name="region"
            extra={t("resource:user.provinceHint", {
              defaultValue: "Country is selected above; this is the province or state.",
            })}
          >
            {/* 原先复用国家选择器，导致「国家码」与「区域」重复；这里改为省/州文本。 */}
            <Input disabled={readOnly} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item label={t("resource:user.location")} name="location">
            <Input disabled={readOnly} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item label={t("resource:user.gender")} name="gender">
            <Select options={GENDER_OPTIONS} allowClear disabled={readOnly} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
            label={t("resource:user.birthday")}
            name="birthday"
            getValueProps={(v) => (v ? { value: dayjs(v) } : { value: undefined })}
          >
            <DatePicker format="YYYY-MM-DD" style={{ width: "100%" }} disabled={readOnly} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={24}>
          <Form.Item label={t("resource:user.bio")} name="bio">
            <Input.TextArea rows={2} disabled={readOnly} />
          </Form.Item>
        </Col>
      </Row>

      <Card
        title={t("resource:user.tabs.properties", "Properties")}
        size="small"
        style={{ marginTop: 16 }}
      >
        <PropertyTable
          value={customProperties}
          onChange={onCustomPropertiesChange}
          readOnly={readOnly}
          definitions={definitions}
        />
      </Card>
    </>
  );
}
