import type { Rule } from "antd/es/form";
import type { TFunction } from "i18next";
import { DEFAULT_PASSWORD_POLICY } from "@/shared/api/iam/auth";
import type { SignupItem } from "@/shared/api/iam/application";

const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const phoneRegex = /^\+?[\d\s()-]{7,20}$/;

/** 根据 signupItem.type 或字段名推断控件类型。优先使用 item.type（编辑器可配置），
 * 若未设置则按字段名做保守推断，最后 fallback 为 text 保持向后兼容。 */
function resolveInputType(item: SignupItem): SignupFieldConfig["inputType"] {
  const t = (item.type ?? "").toLowerCase();
  if (t === "select" || t === "dropdown") return "select";
  if (t === "textarea") return "textarea";
  if (t === "password") return "password";
  if (t === "email") return "email";
  if (t === "phone" || t === "tel") return "tel";
  if (t === "hidden") return "hidden";
  if (t) return "text"; // 其他已声明 type 按 text 处理
  // 无 type：按字段名推断
  switch (item.name) {
    case "Gender":
    case "Country code":
    case "Region":
    case "Education":
      return "select";
    case "Bio":
    case "Address":
    case "Location":
      return "textarea";
    case "Email":
      return "email";
    case "Phone":
      return "tel";
    case "Password":
    case "Confirm password":
      return "password";
    default:
      return "text";
  }
}

export type SignupFieldConfig = {
  name: string;
  label: string;
  placeholder: string;
  inputType: "text" | "email" | "password" | "tel" | "textarea" | "select" | "hidden";
  autoComplete: string;
  rules: Rule[];
  /** Select 控件的选项，由 SignupItem.options 透传。 */
  options?: string[];
};

export function getFieldConfig(item: SignupItem, t: TFunction): SignupFieldConfig {
  const label = item.label || defaultLabel(item.name, t);
  const placeholder = item.placeholder || defaultPlaceholder(item.name, t);
  const requiredMsg = (l: string) => t("auth:fieldRequired", { field: l });
  const inputType = resolveInputType(item);
  const base = {
    inputType,
    autoComplete: "off",
    rules: [{ required: item.required, message: requiredMsg(label) }] as Rule[],
    options: item.options?.length ? item.options : undefined,
  };
  // 每个 case 从 base 继承 inputType（resolveInputType 推断）和 options，
  // 只覆盖语义化的 name / autoComplete / 特化 rules。
  switch (item.name) {
    case "Username":
      return { ...base, name: "username", label, placeholder, autoComplete: "username" };
    case "Email":
      return {
        ...base,
        name: "email",
        label,
        placeholder,
        autoComplete: "email",
        rules: [
          { required: item.required, message: requiredMsg(label) },
          { pattern: emailRegex, message: t("common:form.invalidEmail") },
        ],
      };
    case "Phone":
      return {
        ...base,
        name: "phone",
        label,
        placeholder,
        autoComplete: "tel",
        rules: [
          { required: item.required, message: requiredMsg(label) },
          { pattern: phoneRegex, message: t("common:form.invalidPhone") },
        ],
      };
    case "Password":
      return {
        ...base,
        name: "password",
        label,
        placeholder,
        autoComplete: "new-password",
        rules: [
          { required: item.required, message: requiredMsg(label) },
          {
            min: DEFAULT_PASSWORD_POLICY.minLength,
            message: t("auth:passwordMinLength", { min: DEFAULT_PASSWORD_POLICY.minLength }),
          },
        ],
      };
    case "Confirm password":
      return {
        ...base,
        name: "confirmPassword",
        label,
        placeholder,
        autoComplete: "new-password",
        rules: [
          { required: item.required, message: requiredMsg(label) },
          ({ getFieldValue }: { getFieldValue: (name: string) => unknown }) => ({
            validator(_: unknown, value: string) {
              if (!value || getFieldValue("password") === value) return Promise.resolve();
              return Promise.reject(new Error(t("auth:passwordMismatch")));
            },
          }),
        ],
      };
    case "Display name":
      return { ...base, name: "displayName", label, placeholder, autoComplete: "name" };
    case "First name":
      return { ...base, name: "firstName", label, placeholder, autoComplete: "given-name" };
    case "Last name":
      return { ...base, name: "lastName", label, placeholder, autoComplete: "family-name" };
    case "Country code":
      return { ...base, name: "countryCode", label, placeholder, autoComplete: "tel-country-code" };
    case "Region":
      return { ...base, name: "region", label, placeholder, autoComplete: "address-level1" };
    case "Location":
      return { ...base, name: "location", label, placeholder, autoComplete: "street-address" };
    case "Affiliation":
      return { ...base, name: "affiliation", label, placeholder, autoComplete: "organization" };
    case "Title":
      return { ...base, name: "title", label, placeholder, autoComplete: "organization-title" };
    case "Homepage":
      return {
        ...base,
        name: "homepage",
        label,
        placeholder,
        autoComplete: "url",
        rules: [
          { required: item.required, message: requiredMsg(label) },
          { type: "url", message: t("common:form.invalidUrl") },
        ],
      };
    case "Address":
      return { ...base, name: "address", label, placeholder, autoComplete: "street-address" };
    case "Bio":
      return { ...base, name: "bio", label, placeholder, autoComplete: "off" };
    case "Gender":
      return { ...base, name: "gender", label, placeholder, autoComplete: "sex" };
    case "Birthday":
      return { ...base, name: "birthday", label, placeholder, autoComplete: "bday" };
    case "Education":
      return { ...base, name: "education", label, placeholder, autoComplete: "off" };
    case "Real name":
      return { ...base, name: "realName", label, placeholder, autoComplete: "name" };
    case "ID card":
      return { ...base, name: "idCard", label, placeholder, autoComplete: "off" };
    default:
      return {
        ...base,
        name: item.name.replace(/\s+/g, "_").toLowerCase(),
        label,
        placeholder,
      };
  }
}
// 默认注册字段的标签/占位符复用既有键（14 语言齐全）；
// auth.field.* / auth.placeholder.* 子树并不存在，裸查只会回显原始英文名并刷告警。
// 扩展标准字段与用户创建页同源（resource:user.* / user.placeholder.*），显式命名空间前缀对任意绑定 t 生效。
const STANDARD_LABEL_KEYS: Record<string, string> = {
  username: "auth:username",
  email: "auth:email",
  phone: "auth:phone",
  password: "auth:password",
  confirm_password: "auth:confirmPassword",
  display_name: "resource:user.displayName",
  first_name: "resource:user.firstName",
  last_name: "resource:user.lastName",
  country_code: "auth:countryCode",
  region: "common:region",
  location: "resource:user.location",
  affiliation: "resource:user.affiliation",
  title: "resource:user.jobTitle",
  homepage: "resource:user.homepage",
  address: "resource:user.addressLabel",
  bio: "resource:user.bio",
  gender: "resource:user.gender",
  birthday: "resource:user.birthday",
  education: "resource:user.education",
  real_name: "resource:user.realName",
  id_card: "resource:user.idCard",
};
const STANDARD_PLACEHOLDER_KEYS: Record<string, string> = {
  username: "auth:enterUsername",
  email: "auth:enterEmail",
  phone: "auth:enterPhone",
  password: "auth:enterPassword",
  confirm_password: "auth:enterConfirmPassword",
  display_name: "common:displayName",
  first_name: "resource:user.firstName",
  last_name: "resource:user.lastName",
  country_code: "resource:user.placeholder.countryCode",
  region: "resource:user.placeholder.region",
  location: "resource:user.placeholder.location",
  affiliation: "resource:user.placeholder.affiliation",
  title: "resource:user.placeholder.title",
  address: "resource:user.placeholder.address",
  bio: "resource:user.placeholder.bio",
  gender: "resource:user.placeholder.gender",
  education: "resource:user.placeholder.education",
  real_name: "resource:user.placeholder.realName",
  id_card: "resource:user.placeholder.idCard",
};
// 与用户创建页一致的非文字占位符（示例 URL / 日期格式），无需翻译。
const LITERAL_PLACEHOLDERS: Record<string, string> = {
  homepage: "https://example.com",
  birthday: "YYYY-MM-DD",
};
function fieldKey(name: string): string {
  return name.replace(/\s+/g, "_").toLowerCase();
}
function defaultLabel(name: string, t: TFunction): string {
  const key = fieldKey(name);
  const mapped = STANDARD_LABEL_KEYS[key];
  if (mapped) return t(mapped);
  return name;
}
function defaultPlaceholder(name: string, t: TFunction): string {
  const key = fieldKey(name);
  const literal = LITERAL_PLACEHOLDERS[key];
  if (literal) return literal;
  const mapped = STANDARD_PLACEHOLDER_KEYS[key];
  if (mapped) return t(mapped);
  return `Enter your ${name.toLowerCase()}`;
}
export function getDefaultSignupItems(): SignupItem[] {
  return [
    { name: "Username", visible: true, required: true, prompted: false },
    { name: "Email", visible: true, required: false, prompted: false },
    { name: "Phone", visible: true, required: false, prompted: false },
    { name: "Password", visible: true, required: true, prompted: false },
    { name: "Confirm password", visible: true, required: true, prompted: false },
  ];
}

/** 与 getSigninItems 同策略：默认项为基底，用户配置按 name 覆盖/追加。
 * 预览帧与真实注册页共用，保证必需字段始终可渲染。 */
export function getSignupItems(application: { signupItems?: SignupItem[] | null }): SignupItem[] {
  const custom = application.signupItems ?? [];
  if (custom.length === 0) return getDefaultSignupItems();

  const customByName = new Map<string, SignupItem>();
  for (const item of custom) {
    const name = (item?.name ?? "").trim();
    if (name) customByName.set(name, item);
  }
  const defaults = getDefaultSignupItems();
  const merged = defaults.map((def) => customByName.get(def.name) ?? def);
  const defaultNames = new Set(defaults.map((it) => it.name));
  for (const item of custom) {
    const name = (item?.name ?? "").trim();
    if (name && !defaultNames.has(name)) merged.push(item);
  }
  return merged;
}
