export type CaptchaConfigFieldSpec = {
  /** Form field name inside the unified provider form. */
  formName: string;
  /** Key inside the captcha_providers.config JSONB object. */
  configKey: string;
  /** common-namespace i18n key for the label. */
  labelKey: string;
  input: "text" | "password" | "number" | "select";
  /** Option i18n keys/values for input === "select". */
  options?: readonly { labelKey: string; value: string }[];
};

/**
 * Backend kind → config extension fields. Keep aligned with
 * `internal/infra/captcha/provider_from_row.go`; the unified form has no
 * columns for these values, they are collected into `config` on submit.
 */
export const CAPTCHA_CONFIG_FIELDS: Readonly<Record<string, readonly CaptchaConfigFieldSpec[]>> = {
  recaptcha_v3: [
    {
      formName: "captchaScoreThreshold",
      configKey: "scoreThreshold",
      labelKey: "captcha.scoreThreshold",
      input: "number",
    },
    { formName: "captchaAction", configKey: "action", labelKey: "captcha.action", input: "text" },
  ],
  recaptcha_enterprise: [
    {
      formName: "captchaProjectId",
      configKey: "projectId",
      labelKey: "captcha.projectId",
      input: "text",
    },
    {
      formName: "captchaScoreThreshold",
      configKey: "scoreThreshold",
      labelKey: "captcha.scoreThreshold",
      input: "number",
    },
    { formName: "captchaAction", configKey: "action", labelKey: "captcha.action", input: "text" },
  ],
  aliyun_captcha: [
    {
      formName: "captchaSceneId",
      configKey: "sceneId",
      labelKey: "captcha.sceneId",
      input: "text",
    },
  ],
  tencent_captcha: [
    {
      formName: "captchaCamSecretId",
      configKey: "camSecretId",
      labelKey: "captcha.camSecretId",
      input: "password",
    },
    {
      formName: "captchaCamSecretKey",
      configKey: "camSecretKey",
      labelKey: "captcha.camSecretKey",
      input: "password",
    },
  ],
  netease_yidun: [
    {
      formName: "captchaYidunSecretId",
      configKey: "secretId",
      labelKey: "captcha.secretId",
      input: "password",
    },
  ],
  baidu_captcha: [
    {
      formName: "captchaAppName",
      configKey: "appName",
      labelKey: "captcha.appName",
      input: "text",
    },
  ],
  go_captcha: [
    {
      formName: "captchaShape",
      configKey: "shape",
      labelKey: "captcha.shape",
      input: "select",
      options: [
        { labelKey: "captcha.shapeClick", value: "click" },
        { labelKey: "captcha.shapeSlide", value: "slide" },
      ],
    },
  ],
};

/**
 * Backend kind → credential-slot labels. The dedicated table only has two
 * credential columns (siteKey/secretKey); each vendor names them differently
 * in its console, so the labels follow the official terminology.
 */
export const CAPTCHA_CREDENTIAL_LABELS: Readonly<
  Record<string, { idLabelKey: string; secretLabelKey: string }>
> = {
  default: { idLabelKey: "siteKey", secretLabelKey: "secretKey" },
  recaptcha: { idLabelKey: "siteKey", secretLabelKey: "secretKey" },
  recaptcha_v3: { idLabelKey: "siteKey", secretLabelKey: "secretKey" },
  recaptcha_enterprise: { idLabelKey: "siteKey", secretLabelKey: "apiKey" },
  hcaptcha: { idLabelKey: "siteKey", secretLabelKey: "secretKey" },
  turnstile: { idLabelKey: "siteKey", secretLabelKey: "secretKey" },
  geetest: { idLabelKey: "captcha.captchaId", secretLabelKey: "captcha.captchaKey" },
  aliyun_captcha: { idLabelKey: "accessKeyId", secretLabelKey: "accessKeySecret" },
  tencent_captcha: { idLabelKey: "captcha.captchaAppId", secretLabelKey: "captcha.appSecretKey" },
  netease_yidun: { idLabelKey: "captcha.captchaId", secretLabelKey: "secretKey" },
  baidu_captcha: { idLabelKey: "appId", secretLabelKey: "privateKey" },
};

/**
 * Collect the non-empty config extension values for one kind out of raw form
 * values. Empty strings and untouched numbers are omitted so the stored
 * config only carries what the operator actually filled in.
 */
export function assembleCaptchaConfig(
  kind: string,
  values: Record<string, unknown>,
): Record<string, string | number> {
  const out: Record<string, string | number> = {};
  for (const spec of CAPTCHA_CONFIG_FIELDS[kind] ?? []) {
    const raw = values[spec.formName];
    if (spec.input === "number") {
      if (typeof raw === "number" && Number.isFinite(raw)) out[spec.configKey] = raw;
      continue;
    }
    if (typeof raw === "string" && raw.trim() !== "") out[spec.configKey] = raw.trim();
  }
  return out;
}
