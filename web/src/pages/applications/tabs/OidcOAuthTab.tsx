import {
  Alert,
  Button,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Row,
  Col,
  Select,
  Space,
  Tag,
  Typography,
} from "antd";
import type { FormInstance } from "antd";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { ApplicationClientCertificate } from "@/shared/api/iam/application";
import { certificateFingerprint } from "../mtls-certificate";
import RedirectUriTable from "@/shared/ui/RedirectUriTable";
import type { RedirectUri } from "@/shared/ui/RedirectUriTable";
import TokenAttributeTable from "@/shared/ui/TokenAttributeTable";
import type { TokenAttribute } from "@/shared/ui/TokenAttributeTable";
import { FormSection } from "@/shared/ui/FormSection";
import { ConditionalField } from "@/shared/ui/ConditionalField";
import { TOKEN_SIGNING_METHODS } from "@/shared/constants/enums";
import type { ApplicationConfigFormValues } from "./types";
import {
  grantTypeOptions,
  getTokenEndpointAuthMethodOptions,
  isClientSecretAuthMethod,
  isMTLSAuthMethod,
  parseJwksKids,
  tokenEndpointAuthMethodHintKey,
} from "./types";
import { evaluateOidcIssues } from "../application-oidc-validation";

/** Bridge: converts string[] ↔ RedirectUri[] for Form.Item integration */
function RedirectUriTableInput({
  value,
  onChange,
}: {
  value?: string[];
  onChange?: (v: string[]) => void;
}) {
  const items: RedirectUri[] = (value ?? []).map((uri) => ({ uri }));
  return <RedirectUriTable value={items} onChange={(v) => onChange?.(v.map((r) => r.uri))} />;
}

/** Bridge: converts TokenAttribute[] ↔ form value for Form.Item integration */
function TokenAttributeTableInput({
  value,
  onChange,
}: {
  value?: TokenAttribute[];
  onChange?: (v: TokenAttribute[]) => void;
}) {
  return <TokenAttributeTable value={value ?? []} onChange={(v) => onChange?.(v)} />;
}

type OidcOAuthTabProps = {
  form: FormInstance<ApplicationConfigFormValues>;
  isNew: boolean;
  clientSecretExtra: string;
  clientSecretSet: boolean;
  onRotateSecret?: () => void;
  isRotatingSecret?: boolean;
  /** 二次确认框内展示对象名称而非 UUID（interaction.md 破坏性写操作口径）。 */
  applicationName?: string;
  /** Certificates already bound to the application (fingerprints only). */
  existingClientCertificates?: ApplicationClientCertificate[];
  /** kids already registered for private_key_jwt in app_client_jwks. */
  existingJwksKids?: string[];
  /** 已持久化的认证方式/授权类型，用于实时提示时区分「历史遗留」与「本次改动」。 */
  persistedAuthMethod?: ApplicationConfigFormValues["tokenEndpointAuthMethod"];
  persistedGrantTypes?: string[];
};

export function OidcOAuthTab({
  form,
  isNew,
  clientSecretExtra,
  clientSecretSet,
  onRotateSecret,
  isRotatingSecret = false,
  applicationName,
  existingClientCertificates = [],
  existingJwksKids = [],
  persistedAuthMethod,
  persistedGrantTypes,
}: OidcOAuthTabProps) {
  const { t: tr } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const authMethod = Form.useWatch("tokenEndpointAuthMethod", form) ?? "none";
  const usesClientSecret = isClientSecretAuthMethod(authMethod);
  const usesMTLS = isMTLSAuthMethod(authMethod);
  // Fingerprint preview per editor row; computed on blur, never submitted as
  // the source of truth (the server recomputes from the PEM).
  const [fingerprintPreview, setFingerprintPreview] = useState<Record<number, string>>({});
  const rawJwks = Form.useWatch("jwks", form);
  const jwksKids = useMemo(() => parseJwksKids(rawJwks).kids, [rawJwks]);
  const watchedJwksUri = Form.useWatch("jwksUri", form);
  // 实时提示：与保存链路共用同一套纯函数规则，避免两处校验漂移。
  const watchedSecret = Form.useWatch("clientSecret", form);
  const watchedGrants = Form.useWatch("grantTypes", form);
  const watchedCertificates = Form.useWatch("mtlsCertificates", form);
  const liveIssues = useMemo(
    () =>
      evaluateOidcIssues(
        {
          displayName: "",
          status: "",
          tokenEndpointAuthMethod: authMethod,
          clientSecret: watchedSecret ?? "",
          jwks: rawJwks ?? "",
          jwksUri: watchedJwksUri ?? "",
          grantTypes: watchedGrants ?? [],
          mtlsCertificates: watchedCertificates ?? [],
        },
        {
          // 编辑态沿用已存 JWKS 时不应提示缺失。
          requireClientJwks: isNew || authMethod !== persistedAuthMethod,
          persistedAuthMethod,
          persistedGrantTypes,
          existingCertificateCount: existingClientCertificates.length,
        },
      ),
    [
      authMethod,
      watchedSecret,
      rawJwks,
      watchedJwksUri,
      watchedGrants,
      watchedCertificates,
      isNew,
      persistedAuthMethod,
      persistedGrantTypes,
      existingClientCertificates.length,
    ],
  );

  // Methods are mutually exclusive: drop material the newly selected method
  // does not use so a hidden value cannot be submitted by accident.
  const handleAuthMethodChange = (next: ApplicationConfigFormValues["tokenEndpointAuthMethod"]) => {
    if (!isClientSecretAuthMethod(next)) {
      form.setFieldValue("clientSecret", "");
    }
    if (next !== "private_key_jwt") {
      form.setFieldValue("jwks", "");
      form.setFieldValue("jwksUri", "");
    }
    if (!isMTLSAuthMethod(next)) {
      form.setFieldValue("mtlsCertificates", []);
    }
    // A public client can only redeem authorization_code, and the token
    // endpoint rejects client_credentials for it. Keep the grant selection on
    // a combination the server will actually accept.
    if (next === "none") {
      const currentGrants = (form.getFieldValue("grantTypes") as string[] | undefined) ?? [];
      const nextGrants = currentGrants.filter((grant) => grant !== "client_credentials");
      if (!nextGrants.includes("authorization_code")) {
        nextGrants.unshift("authorization_code");
      }
      form.setFieldValue("grantTypes", nextGrants);
    }
  };

  const previewFingerprint = async (index: number, pem: string | undefined) => {
    const value = (pem ?? "").trim();
    if (value === "") {
      setFingerprintPreview((previous) => ({ ...previous, [index]: "" }));
      return;
    }
    try {
      const fingerprint = await certificateFingerprint(value);
      setFingerprintPreview((previous) => ({ ...previous, [index]: fingerprint }));
    } catch {
      setFingerprintPreview((previous) => ({
        ...previous,
        [index]: tr("application.validation.certificateInvalid", "Invalid certificate"),
      }));
    }
  };

  return (
    <>
      {liveIssues.length > 0 ? (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          title={tr("application.oidcIssuesTitle")}
          description={
            <ul style={{ margin: 0, paddingInlineStart: 20 }}>
              {liveIssues.map((issue) => (
                <li key={issue.code}>{tr(`application.validation.${issue.code}`, issue.params)}</li>
              ))}
            </ul>
          }
        />
      ) : null}
      <FormSection title={tr("application.oidcClientSection", "OIDC Client Credentials")}>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              label={tr("application.clientId")}
              name="clientId"
              tooltip={isNew ? tr("application.clientIdAutoGenHint") : undefined}
              rules={[
                // 非空时校验格式; isNew 时允许空值 (后端 auto-gen)
                {
                  pattern: /^$|^gl_cid_[A-Za-z0-9_-]+$|^[a-zA-Z][a-zA-Z0-9_-]*$/,
                  message: tr(
                    "application.validation.clientIdFormat",
                    "留空自动生成; 或输入 gl_cid_ 前缀格式 / 字母开头的自定义 ID",
                  ),
                },
              ]}
            >
              <Input
                autoComplete="off"
                spellCheck={false}
                placeholder={tr("application.placeholder.clientId")}
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              label={tr("application.tokenEndpointAuthMethod")}
              name="tokenEndpointAuthMethod"
            >
              <Select
                options={getTokenEndpointAuthMethodOptions(tr)}
                onChange={handleAuthMethodChange}
              />
            </Form.Item>
            <Typography.Text type="secondary">
              {tr(tokenEndpointAuthMethodHintKey(authMethod), "")}
            </Typography.Text>
          </Col>
        </Row>
        {usesClientSecret ? (
          <Form.Item
            label={tr("application.clientSecret")}
            name="clientSecret"
            extra={
              <Space orientation="vertical" size={0}>
                <span>{clientSecretExtra}</span>
                {!isNew && clientSecretSet && onRotateSecret ? (
                  <Popconfirm
                    title={tr("application.rotateConfirmTitle", "Rotate client secret?")}
                    description={tr(
                      "application.rotateConfirmDesc",
                      "The current secret of {{name}} becomes invalid immediately. The new secret is shown only once.",
                      { name: applicationName ?? "" },
                    )}
                    okText={tr("application.rotateClientSecret", "Rotate secret")}
                    cancelText={tc("cancel")}
                    okButtonProps={{ danger: true, loading: isRotatingSecret }}
                    onConfirm={onRotateSecret}
                  >
                    <Button type="link" size="small" loading={isRotatingSecret}>
                      {tr("application.rotateClientSecret", "Rotate secret")}
                    </Button>
                  </Popconfirm>
                ) : null}
              </Space>
            }
          >
            <Input.Password
              autoComplete="new-password"
              placeholder={
                isNew
                  ? tr("application.placeholder.clientSecretNew")
                  : tr("application.placeholder.clientSecretEdit")
              }
            />
          </Form.Item>
        ) : null}
        {authMethod === "private_key_jwt" ? (
          <Form.Item
            label={tr("application.clientJwks", "JWKS (public keys)")}
            name="jwks"
            extra={
              <Space orientation="vertical" size={4}>
                <span>
                  {tr(
                    "application.clientJwksExtra",
                    'Paste a public JWKS object {"keys":[...]}. Private parameters are rejected.',
                  )}
                </span>
                {existingJwksKids.length > 0 ? (
                  <span>
                    {tr("application.clientJwksExisting", "Currently registered kid:")}{" "}
                    <Space size={[4, 4]} wrap>
                      {existingJwksKids.map((kid) => (
                        <Tag key={kid} color="default">
                          {kid}
                        </Tag>
                      ))}
                    </Space>
                  </span>
                ) : null}
                {jwksKids.length > 0 ? (
                  <Space size={[4, 4]} wrap>
                    {jwksKids.map((kid) => (
                      <Tag key={kid} color="geekblue">
                        {kid}
                      </Tag>
                    ))}
                  </Space>
                ) : null}
              </Space>
            }
          >
            <Input.TextArea
              rows={6}
              spellCheck={false}
              placeholder={
                '{"keys":[{"kty":"RSA","kid":"...","use":"sig","alg":"RS256","n":"...","e":"..."}]}'
              }
            />
          </Form.Item>
        ) : null}
        {authMethod === "private_key_jwt" ? (
          <Form.Item
            label={tr("application.clientJwksUri", "JWKS URI (remote)")}
            name="jwksUri"
            extra={tr(
              "application.clientJwksUriExtra",
              "HTTPS URL of a remotely hosted JWKS; an alternative to pasting inline keys. Either one satisfies private_key_jwt. Clearing a saved URI removes it on save.",
            )}
          >
            <Input
              type="url"
              maxLength={2048}
              allowClear
              placeholder="https://client.example.com/jwks.json"
            />
          </Form.Item>
        ) : null}
        {usesMTLS ? (
          <Form.Item
            label={tr("application.mtlsCertificates", "Client certificates")}
            extra={tr(
              "application.mtlsCertificatesExtra",
              "Paste the public certificate (PEM). The browser derives the 43-character x5t#S256 fingerprint and the server recomputes it before storing.",
            )}
          >
            {existingClientCertificates.length > 0 ? (
              <Space orientation="vertical" size={2} style={{ marginBottom: 8 }}>
                <Typography.Text type="secondary">
                  {tr(
                    "application.mtlsExistingCertificates",
                    "Currently registered certificates (fingerprints only):",
                  )}
                </Typography.Text>
                <Space size={[4, 4]} wrap>
                  {existingClientCertificates.map((certificate) => (
                    <Tag key={certificate.fingerprintSha256} color="geekblue">
                      {certificate.fingerprintSha256}
                    </Tag>
                  ))}
                </Space>
              </Space>
            ) : null}
            <Form.List name="mtlsCertificates">
              {(fields, { add, remove }) => (
                <Space orientation="vertical" size={8} style={{ width: "100%" }}>
                  {fields.map((field) => (
                    <Space key={field.key} align="start" style={{ width: "100%" }}>
                      <Form.Item name={[field.name, "certificate"]} noStyle>
                        <Input.TextArea
                          rows={4}
                          spellCheck={false}
                          placeholder={
                            "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"
                          }
                          onBlur={(event) =>
                            void previewFingerprint(field.name, event.target.value)
                          }
                        />
                      </Form.Item>
                      <Button danger type="text" onClick={() => remove(field.name)}>
                        {tr("application.mtlsRemoveCertificate", "Remove")}
                      </Button>
                      {fingerprintPreview[field.name] ? (
                        <Typography.Text type="secondary">
                          {fingerprintPreview[field.name]}
                        </Typography.Text>
                      ) : null}
                    </Space>
                  ))}
                  <Button type="dashed" onClick={() => add({ certificate: "" })}>
                    {tr("application.mtlsAddCertificate", "Add certificate")}
                  </Button>
                </Space>
              )}
            </Form.List>
            <Alert
              type="info"
              showIcon
              style={{ marginTop: 8 }}
              title={tr(
                "application.mtlsReplaceNotice",
                "Saving replaces the whole certificate set. Leave the editor empty to keep the currently registered certificates.",
              )}
            />
            <Alert
              type="warning"
              showIcon
              style={{ marginTop: 8 }}
              title={tr(
                "application.mtlsDeploymentHint",
                "tls_client_auth requires this service to terminate TLS with TLS_CLIENT_CA_FILE configured; certificates cannot be verified when a reverse proxy terminates TLS. self_signed_tls_client_auth matches the registered fingerprint only and does not validate a chain.",
              )}
            />
          </Form.Item>
        ) : null}
        {authMethod === "none" ? (
          <Alert
            type="info"
            showIcon
            title={tr(
              "application.publicClientPkceRequired",
              "Public client: no secret, no JWKS and no certificate. PKCE and the authorization_code grant are mandatory.",
            )}
          />
        ) : null}
      </FormSection>

      <FormSection title={tr("application.redirectAndGrantSection", "Redirect URIs & Grant Types")}>
        <Form.Item
          label={tr("application.redirectUris")}
          name="redirectUris"
          tooltip={tr(
            "application.redirectUrisTooltip",
            "Allowed callback URL whitelist. Unregistered URLs are rejected.",
          )}
          rules={[
            {
              validator: async (_rule, uris?: string[]) => {
                for (const uri of uris ?? []) {
                  const value = (uri ?? "").trim();
                  if (!value) continue;
                  let parsed: URL;
                  try {
                    parsed = new URL(value);
                  } catch {
                    throw new Error(tr("application.oidcFieldErrors.invalidRedirectUri", { uri }));
                  }
                  if (!/^https?:$/.test(parsed.protocol) || value.includes("#")) {
                    throw new Error(tr("application.oidcFieldErrors.invalidRedirectUri", { uri }));
                  }
                }
              },
            },
          ]}
        >
          <RedirectUriTableInput />
        </Form.Item>
        <Form.Item
          label={tr("application.forcedRedirectOrigin")}
          name="forcedRedirectOrigin"
          rules={[
            {
              validator: async (_rule, origin?: string) => {
                const value = (origin ?? "").trim();
                if (!value) return;
                if (!/^https?:\/\/[^/\s]+$/i.test(value)) {
                  throw new Error(tr("application.oidcFieldErrors.invalidOrigin"));
                }
              },
            },
          ]}
        >
          <Input type="url" inputMode="url" autoComplete="url" placeholder="https://example.com" />
        </Form.Item>
        <Form.Item
          label={tr("application.grantTypes")}
          name="grantTypes"
          tooltip={tr(
            "application.grantTypesTooltip",
            "Allowed OAuth flows this application may initiate.",
          )}
        >
          <Select
            mode="multiple"
            options={grantTypeOptions}
            placeholder={tr("application.placeholder.grantTypes")}
          />
        </Form.Item>
      </FormSection>

      <FormSection title={tr("application.tokenConfigSection", "Token Configuration")}>
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item label={tr("application.tokenFormat", "Token Format")} name="tokenFormat">
              <Select
                allowClear
                options={[
                  { label: "JWT", value: "JWT" },
                  { label: "JWT-Empty", value: "JWT-Empty" },
                  { label: "JWT-Custom", value: "JWT-Custom" },
                  { label: "JWT-Standard", value: "JWT-Standard" },
                  { label: "Opaque", value: "Opaque" },
                ]}
              />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              label={tr("application.tokenSigningMethod", "Token Signing Method")}
              name="tokenSigningMethod"
              extra={tr("application.tokenSigningMethodHint", "Token Signing Method Hint")}
            >
              <Select
                allowClear
                options={TOKEN_SIGNING_METHODS.map((value) => ({ label: value, value }))}
              />
            </Form.Item>
          </Col>
          <Col span={8}>
            <ConditionalField
              dependencies={["tokenFormat"]}
              when={(values) => values.tokenFormat === "JWT-Custom"}
            >
              <Form.Item
                label={tr("application.tokenAttributes", "Token Attributes")}
                name="tokenAttributes"
              >
                <TokenAttributeTableInput />
              </Form.Item>
            </ConditionalField>
            <ConditionalField
              dependencies={["tokenFormat"]}
              when={(values) => values.tokenFormat !== "JWT-Custom"}
            >
              <Form.Item label={tr("application.tokenFields", "Token Fields")} name="tokenFields">
                <Select mode="tags" tokenSeparators={[","]} placeholder={tc("jwtCustomFields")} />
              </Form.Item>
            </ConditionalField>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item
              label={tr("application.expireInHours", "Expire In Hours")}
              name="expireInHours"
              rules={[
                {
                  type: "number",
                  min: 1,
                  max: 8760,
                  message: tr("application.oidcFieldErrors.expireRange", { min: 1, max: 8760 }),
                },
              ]}
            >
              <InputNumber min={1} max={8760} precision={0} style={{ width: "100%" }} />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              label={tr("application.refreshTokenExpireInHours", "Refresh Token Expire (h)")}
              name="refreshTokenExpireInHours"
              rules={[
                {
                  type: "number",
                  min: 1,
                  max: 8760,
                  message: tr("application.oidcFieldErrors.expireRange", { min: 1, max: 8760 }),
                },
              ]}
            >
              <InputNumber min={1} max={8760} precision={0} style={{ width: "100%" }} />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              label={tr("application.codeExpire", "Auth Code Expire (min)")}
              name="codeExpire"
              rules={[
                {
                  type: "number",
                  min: 1,
                  max: 3600,
                  message: tr("application.oidcFieldErrors.expireRange", { min: 1, max: 3600 }),
                },
              ]}
            >
              <InputNumber
                min={1}
                max={3600}
                precision={0}
                style={{ width: "100%" }}
                placeholder="5"
              />
            </Form.Item>
          </Col>
        </Row>
      </FormSection>
    </>
  );
}
