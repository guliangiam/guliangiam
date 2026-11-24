import { Alert, Col, Form, Input, Modal, Row, Segmented, Select, Switch, Typography } from "antd";
import type { FormInstance } from "antd";
import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import SamlAttributeTable from "@/shared/ui/SamlAttributeTable";
import type { SamlAttribute } from "@/shared/ui/SamlAttributeTable";
import { FormSection } from "@/shared/ui/FormSection";
import { CodeEditor } from "@/shared/ui/CodeEditor";
import { SamlMetadataViewer } from "@/shared/ui/SamlMetadataViewer";
import type { ApplicationConfigFormValues } from "./types";

/** Bridge: connects SamlAttributeTable to Ant Design Form value/onChange pattern */
function SamlAttributeTableInput({
  value,
  onChange,
}: {
  value?: SamlAttribute[];
  onChange?: (v: SamlAttribute[]) => void;
}) {
  return <SamlAttributeTable value={value ?? []} onChange={(v) => onChange?.(v)} />;
}

type SamlTabProps = {
  form: FormInstance<ApplicationConfigFormValues>;
};

export function SamlTab({ form }: SamlTabProps) {
  const { t: tr } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const handleSourceChange = useCallback(
    (next: "url" | "xml", current: "url" | "xml") => {
      if (next === current) return;
      // Nothing is erased in the form: the non-selected side is only omitted at
      // submit time, so switching back and forth stays lossless.
      const url = (form.getFieldValue("samlMetadataUrl") as string | undefined)?.trim();
      const xml = (form.getFieldValue("samlMetadata") as string | undefined)?.trim();
      const cert = (form.getFieldValue("certificate") as string | undefined)?.trim();
      const oppositeHasValue = next === "xml" ? !!(url || cert) : !!xml;
      const apply = () => form.setFieldValue("samlMetadataSource", next);
      if (!oppositeHasValue) {
        apply();
        return;
      }
      Modal.confirm({
        title: tr("application.samlMetadataSource.switchTitle"),
        content: tr("application.samlMetadataSource.switchConfirm"),
        okText: tc("ok"),
        cancelText: tc("cancel"),
        onOk: apply,
      });
    },
    [form, tr, tc],
  );

  return (
    <>
      {/*
       * 无 UI 的注册项：保存链路是 `form.validateFields()`（只含已注册字段），
       * 不注册的话 `values.samlMetadataSource` 恒为 undefined，
       * application-payload 里「显式切 url/xml 才清空对侧」的分支永远不会生效。
       */}
      <Form.Item name="samlMetadataSource" noStyle initialValue="keep" />
      <FormSection title={tr("application.samlEndpointsSection", "SAML Endpoints")}>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={12}>
            <Form.Item
              label={tr("application.samlReplyUrl", "SAML Reply URL (ACS)")}
              name="samlReplyUrl"
            >
              <Input
                type="url"
                inputMode="url"
                autoComplete="url"
                placeholder="https://example.com/saml/acs"
              />
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      <FormSection title={tr("application.samlMetadataSourceSection")}>
        {/*
         * 来源切换需要同时观察 `samlMetadataSource` 与两侧内容的变化，
         * `shouldUpdate` 直接对比整个 store，比多个 `useWatch` 更省一次状态同步。
         */}
        <Form.Item
          noStyle
          shouldUpdate={(prev, cur) =>
            prev.samlMetadataSource !== cur.samlMetadataSource ||
            prev.samlMetadataUrl !== cur.samlMetadataUrl ||
            prev.samlMetadata !== cur.samlMetadata
          }
        >
          {({ getFieldValue }) => {
            const urlValue = (getFieldValue("samlMetadataUrl") as string | undefined) ?? "";
            const xmlValue = (getFieldValue("samlMetadata") as string | undefined) ?? "";
            const rawSource = getFieldValue("samlMetadataSource") as
              | "keep"
              | "url"
              | "xml"
              | undefined;
            // `"keep"` is the reset default: derive the visible tab from stored data and
            // leave both sides untouched. Only an explicit switch flips this to "url"/"xml".
            const derivedSource: "url" | "xml" = urlValue.trim()
              ? "url"
              : xmlValue.trim()
                ? "xml"
                : "url";
            const source: "url" | "xml" =
              rawSource === "url" || rawSource === "xml" ? rawSource : derivedSource;
            const hasBothSources = !!(urlValue.trim() && xmlValue.trim());
            return (
              <>
                <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
                  {tr("application.samlMetadataSource.hint")}
                </Typography.Paragraph>
                {hasBothSources ? (
                  <Alert
                    type="warning"
                    showIcon
                    style={{ marginBottom: 12 }}
                    title={tr("application.samlMetadataSource.bothHint")}
                  />
                ) : null}
                <Segmented
                  value={source}
                  onChange={(value) => handleSourceChange(value as "url" | "xml", source)}
                  options={[
                    { label: tr("application.samlMetadataSource.url"), value: "url" },
                    { label: tr("application.samlMetadataSource.xml"), value: "xml" },
                  ]}
                />
                <div style={{ marginTop: 16 }}>
                  {source === "url" ? (
                    <>
                      <Form.Item
                        label={tr("application.samlMetadataUrl", "SAML Metadata URL")}
                        name="samlMetadataUrl"
                      >
                        <Input
                          type="url"
                          inputMode="url"
                          autoComplete="url"
                          placeholder="https://idp.example.com/metadata"
                        />
                      </Form.Item>
                      <Form.Item
                        label={tr("application.certificate", "Certificate")}
                        name="certificate"
                      >
                        <Input
                          autoComplete="off"
                          spellCheck={false}
                          placeholder={tr("application.placeholder.certificate", "Certificate ID")}
                        />
                      </Form.Item>
                    </>
                  ) : (
                    <>
                      <Form.Item
                        label={tr("application.samlMetadataXml", "SAML Metadata XML")}
                        name="samlMetadata"
                      >
                        <CodeEditor
                          language="xml"
                          height={200}
                          placeholder={tr(
                            "application.placeholder.samlMetadataXml",
                            "Paste SAML XML metadata here",
                          )}
                        />
                      </Form.Item>
                      <Form.Item
                        noStyle
                        shouldUpdate={(prev, cur) => prev.samlMetadata !== cur.samlMetadata}
                      >
                        {({ getFieldValue: getNestedFieldValue }) => (
                          <SamlMetadataViewer xml={getNestedFieldValue("samlMetadata") ?? ""} />
                        )}
                      </Form.Item>
                    </>
                  )}
                </div>
                <Typography.Text type="secondary">
                  {tr("application.samlMetadataSource.clearHint")}
                </Typography.Text>
              </>
            );
          }}
        </Form.Item>
      </FormSection>

      <FormSection title={tr("application.samlProtocolOptionsSection", "SAML Protocol Options")}>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.enableSamlCompress", "SAML Compress")}
              name="enableSamlCompress"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.enableSamlC14n10", "C14N 1.0")}
              name="enableSamlC14n10"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.enableSamlPostBinding", "POST Binding")}
              name="enableSamlPostBinding"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.samlSignRequest", "Sign Request")}
              name="samlSignRequest"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.disableSamlAttributes", "Disable Attributes")}
              name="disableSamlAttributes"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.enableSamlAssertionSignature", "Assertion Signature")}
              name="enableSamlAssertionSignature"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.useEmailAsSamlNameId", "Email as NameID")}
              name="useEmailAsSamlNameId"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Form.Item
              label={tr("application.samlHashAlgorithm", "Hash Algorithm")}
              name="samlHashAlgorithm"
            >
              <Select
                allowClear
                placeholder="SHA256"
                options={[
                  { label: "SHA1", value: "SHA1" },
                  { label: "SHA256", value: "SHA256" },
                  { label: "SHA512", value: "SHA512" },
                ]}
              />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={8}>
            <Form.Item
              noStyle
              shouldUpdate={(prev, cur) => prev.enableSamlC14n10 !== cur.enableSamlC14n10}
            >
              {({ getFieldValue }) =>
                getFieldValue("enableSamlC14n10") ? (
                  <Form.Item
                    label={tr("application.samlC14nPrefix", "C14N Prefix")}
                    name="samlC14nPrefix"
                  >
                    <Select
                      allowClear
                      placeholder={tr("application.placeholder.selectPrefix", "Select prefix")}
                      options={[
                        { label: tr("application.noneOption", "(none)"), value: "" },
                        { label: "xs", value: "xs" },
                      ]}
                    />
                  </Form.Item>
                ) : null
              }
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      <FormSection title={tr("application.samlNameIdConfigSection", "SAML NameID Configuration")}>
        <Row gutter={16}>
          <Col xs={24} sm={12} md={8}>
            <Form.Item
              label={tr("application.samlNameIdFormat", "NameID Format")}
              name="samlNameIdFormat"
            >
              <Select
                allowClear
                placeholder={tr("application.placeholder.selectFormat", "Select format")}
                options={[
                  {
                    label: tr("application.samlNameIdFormatOption.unspecified", "Unspecified"),
                    value: "unspecified",
                  },
                  {
                    label: tr("application.samlNameIdFormatOption.emailAddress", "EmailAddress"),
                    value: "emailAddress",
                  },
                  {
                    label: tr("application.samlNameIdFormatOption.persistent", "Persistent"),
                    value: "persistent",
                  },
                  {
                    label: tr("application.samlNameIdFormatOption.transient", "Transient"),
                    value: "transient",
                  },
                ]}
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} md={8}>
            <Form.Item
              label={tr("application.samlNameIdContent", "NameID Content")}
              name="samlNameIdContent"
            >
              <Select
                allowClear
                placeholder={tr("application.placeholder.selectContent", "Select content")}
                options={[
                  {
                    label: tr("application.samlNameIdContentOption.userId", "User ID"),
                    value: "userId",
                  },
                  {
                    label: tr("application.samlNameIdContentOption.email", "Email"),
                    value: "email",
                  },
                  {
                    label: tr("application.samlNameIdContentOption.username", "Username"),
                    value: "username",
                  },
                  {
                    label: tr("application.samlNameIdContentOption.displayName", "Display Name"),
                    value: "displayName",
                  },
                ]}
              />
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      <FormSection title={tr("application.samlAttributeMappingSection", "SAML Attribute Mapping")}>
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          {tr(
            "application.samlAttributeMappingDesc",
            "Map application attributes to SAML assertion attributes.",
          )}
        </Typography.Paragraph>
        <Form.Item name="samlAttributes" noStyle>
          <SamlAttributeTableInput />
        </Form.Item>
      </FormSection>
    </>
  );
}
