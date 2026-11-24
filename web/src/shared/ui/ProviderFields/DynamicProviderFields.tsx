import { useReducer } from "react";
import { Form } from "antd";
import type { FormInstance } from "antd";
import { OAuthProviderFields } from "./OAuthProviderFields";
import { SamlProviderFields } from "./SamlProviderFields";
import { EmailProviderFields } from "./EmailProviderFields";
import { SmsProviderFields } from "./SmsProviderFields";
import { StorageProviderFields } from "./StorageProviderFields";
import { PaymentProviderFields } from "./PaymentProviderFields";
import { NotificationProviderFields } from "./NotificationProviderFields";
import { CaptchaProviderFields } from "./CaptchaProviderFields";
import MfaProviderFields from "./MfaProviderFields";
import { FaceIdProviderFields } from "./FaceIdProviderFields";
import { ScanProviderFields } from "./ScanProviderFields";
import { IDVerificationProviderFields } from "./IDVerificationProviderFields";
import { LogProviderFields } from "./LogProviderFields";
import { AuditProviderFields } from "./AuditProviderFields";

export type ProviderFieldComponentProps = {
  form: FormInstance;
};

/**
 * Mapping from component value keys to form field names.
 * Used by ProviderFieldBridge to sync between the form store
 * and the controlled value/onChange interface.
 *
 * 约束：目标列必须是 providers 表真实存在且语义匹配的列；同一时刻可见的两个
 * 控件不得写同一列（否则保存值互相覆盖）。没有落库列的表单项不得出现在本表，
 * 也不得继续渲染成可编辑控件（历史 extras 机制已移除）。
 */
const CATEGORY_FIELD_MAP: Record<string, Record<string, string>> = {
  OAuth: {
    clientId: "clientId",
    clientSecret: "clientSecret",
    authUrl: "authUrl",
    tokenUrl: "tokenUrl",
    userInfoUrl: "userInfoUrl",
    scope: "scopes",
    redirectUri: "callbackUrl",
    clientId2: "clientId2",
    agentId: "agentId",
    domain: "issuerUrl",
    customAuthUrl: "authUrl",
    customTokenUrl: "tokenUrl",
    customUserInfoUrl: "userInfoUrl",
  },
  OIDC: {
    clientId: "clientId",
    clientSecret: "clientSecret",
    authUrl: "authUrl",
    tokenUrl: "tokenUrl",
    userInfoUrl: "userInfoUrl",
    scope: "scopes",
    redirectUri: "callbackUrl",
    domain: "issuerUrl",
  },
  Email: {
    host: "host",
    port: "port",
    username: "admin",
    password: "password",
    sslMode: "subType",
    endpoint: "customApi",
    method: "method",
    contentType: "contentType",
    httpHeaders: "customHeader",
    accessKeyId: "admin",
    accessKeySecret: "password",
    apiKey: "password",
    apiToken: "password",
    appKey: "password",
    serverToken: "password",
    accessToken: "password",
    sender: "sender",
    appId: "appId",
    accountId: "appId",
    region: "region",
    domain: "host",
  },
  SMS: {
    accessKeyId: "admin",
    accessKeySecret: "password",
    signName: "sender",
    templateCode: "template",
    appId: "appId",
    region: "region",
    endpoint: "host",
    method: "method",
    contentType: "contentType",
    httpHeaders: "customHeader",
    customBody: "customBody",
  },
  Storage: {
    bucket: "appId",
    region: "region",
    endpoint: "host",
    accessKeyId: "admin",
    accessKeySecret: "password",
  },
  SAML: {
    metadataUrl: "issuerUrl",
    metadataXml: "metadata",
    ssoUrl: "ssoUrl",
    sloUrl: "sloUrl",
    spAcsUrl: "callbackUrl",
    spEntityId: "idpEntityId",
  },
};

/** Categories that use the new value/onChange interface via ProviderFieldBridge */
const BRIDGED_CATEGORIES = new Set(Object.keys(CATEGORY_FIELD_MAP));

/** Categories that use the legacy Form.Item name binding (no value/onChange) */
const LEGACY_PROVIDER_COMPONENTS: Record<string, React.ComponentType<{ form?: FormInstance }>> = {
  Payment: PaymentProviderFields,
  Notification: NotificationProviderFields,
  Captcha: CaptchaProviderFields,
  MFA: MfaProviderFields,
  FaceId: FaceIdProviderFields,
  Scan: ScanProviderFields,
  IDVerification: IDVerificationProviderFields,
  Log: LogProviderFields,
  Audit: AuditProviderFields,
};

/** New-style components that accept value/onChange props */
const NEW_PROVIDER_COMPONENTS: Record<
  string,
  React.ComponentType<{
    value: Record<string, string>;
    onChange: (field: string, val: string) => void;
    subType?: string;
  }>
> = {
  OAuth: OAuthProviderFields,
  OIDC: OAuthProviderFields,
  SAML: SamlProviderFields,
  Email: EmailProviderFields,
  SMS: SmsProviderFields,
  Storage: StorageProviderFields,
};

/** Bridge component that adapts between Ant Design Form instance
 * and the controlled value/onChange interface of new-style components.
 *
 * 注意：antd 的 `Form.useWatch(namePath, form)` 只接受**一个字段路径**；传入
 * 字段名数组会被当成嵌套路径（`values['clientId']['clientSecret']…`）从而恒为
 * undefined，受控输入框读到的永远是空串，表现为「输入后立刻被清空、无法输入」。
 * 因此这里用 `Form.useWatch([], form)` 订阅整份表单值，再按字段名取值。
 *
 * IME 约束：useWatch 的通知走 macroTask 异步批处理，只靠它回读会让受控 value
 * 晚于 DOM 一个任务——中文组合输入期间旧值被回写，表现为拼音重复/闪跳。
 * 所以渲染值直接读活 store，onChange 写 store 后同步重渲染，保证 value 与
 * DOM 同帧一致；useWatch 仅负责触发外部变更（详情回填、类别切换）的重渲染。
 */
function ProviderFieldBridge({ category, form }: { category: string; form: FormInstance }) {
  const fieldMap = CATEGORY_FIELD_MAP[category];
  // 这些 provider 字段没有对应的 Form.Item（组件用受控输入 + onChange 写表单），
  // 因此必须用 preserve:true 读取整份 store；默认只返回已注册字段的值，读不到
  // setFieldValue 写入的键。
  Form.useWatch([], { form, preserve: true });
  const watchedProviderType = Form.useWatch("providerType", form);
  const [, rerender] = useReducer((n: number) => n + 1, 0);

  if (!fieldMap) return null;

  const allValues = form.getFieldsValue(true) as Record<string, unknown>;
  const value: Record<string, string> = {};
  Object.entries(fieldMap).forEach(([componentField, formField]) => {
    const raw = allValues?.[formField];
    if (formField === "scopes" && Array.isArray(raw)) {
      value[componentField] = raw.join(", ");
    } else if (formField === "port" && typeof raw === "number") {
      value[componentField] = String(raw);
    } else {
      value[componentField] = raw === null || raw === undefined ? "" : String(raw);
    }
  });

  const onChange = (componentField: string, val: string) => {
    const formField = fieldMap[componentField];
    // 不再接受映射表以外的字段：能填但不落库的输入一律不渲染。
    if (!formField) return;
    if (formField === "scopes") {
      form.setFieldValue(
        formField,
        val
          ? val
              .split(",")
              .map((s: string) => s.trim())
              .filter(Boolean)
          : [],
      );
    } else if (formField === "port") {
      form.setFieldValue(formField, val ? Number(val) : undefined);
    } else {
      // 保留空串而非 undefined：后端 applyOptionalString 对非 nil 指针赋值，
      // 显式传 "" 才能清空已落库的旧值（FE-B-13）；undefined 会被 PATCH 省略。
      form.setFieldValue(formField, val);
    }
    // 同步重渲染：受控 value 必须在同一帧追上 DOM，否则 IME 组合被打断。
    rerender();
  };

  const Component = NEW_PROVIDER_COMPONENTS[category];
  if (!Component) return null;
  return <Component value={value} onChange={onChange} subType={watchedProviderType} />;
}

export function DynamicProviderFields({
  category,
  form,
}: {
  category: string;
  form?: FormInstance;
}) {
  // New-style component with value/onChange bridge
  if (BRIDGED_CATEGORIES.has(category) && form) {
    return <ProviderFieldBridge category={category} form={form} />;
  }

  // Legacy component using Form.Item name binding
  const Component = LEGACY_PROVIDER_COMPONENTS[category];
  if (!Component) return null;
  return <Component form={form} />;
}
