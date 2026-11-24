/**
 * 登录项 / 注册项默认 CSS 映射、名称常量与 rule 选项。
 *
 * CSS 选择器对齐真实登录页 `LoginSigninForm` 的 DOM 结构，
 * 因此在预览与线上登录页中效果一致。
 */

/** LoginSigninForm 识别的内置登录项名称（不含自定义 Text * 项）。 */
export const SIGNIN_ITEM_NAMES = [
  "Signin methods",
  "Select organization",
  "Username",
  "Password",
  "Verification code",
  "Agreement",
  "Captcha",
  "Login button",
  "Forgot password?",
  "Signup link",
  "Providers",
] as const;

export type SigninItemName = (typeof SIGNIN_ITEM_NAMES)[number];

export const SIGNIN_ITEM_DEFAULT_CSS: Record<string, string> = {
  "Signin methods": `.signin-methods{margin-bottom:16px}`,
  "Select organization": `.login-organization{margin-bottom:12px}`,
  Username: `.login-form input[name="username"]{border-radius:6px}`,
  Password: `.login-form input[name="password"]{border-radius:6px}`,
  "Verification code": `.login-form input[name="verificationCode"]{border-radius:6px}`,
  Agreement: `.login-agreement{margin:8px 0;font-size:13px}`,
  Captcha: `.login-captcha{margin:8px 0}`,
  "Login button": `.login-form button[type="submit"]{border-radius:8px;height:44px}`,
  "Forgot password?": `.login-forget-password{display:flex;justify-content:flex-end;margin-bottom:12px}`,
  "Signup link": `.login-signup-link{margin-bottom:16px;display:flex;justify-content:flex-end}`,
  Providers: `.provider-link{display:inline-block;border-radius:50%}.provider-img{width:30px;margin:5px;border-radius:50%}`,
  WebAuthn: `.login-custom-signin-item[data-name="webauthn"]{padding:8px 0}`,
  LDAP: `.login-custom-signin-item[data-name="ldap"]{padding:8px 0}`,
  "Face ID": `.login-custom-signin-item[data-name="faceId"]{padding:8px 0}`,
  "Device login": `.login-custom-signin-item[data-name="qrcode"]{padding:8px 0}`,
};

/** 注册项名称常量（与 SignupItemsEditor 列表保持一致）。 */
export const SIGNUP_ITEM_NAMES = [
  "Username",
  "Display name",
  "Email",
  "Phone",
  "Password",
  "Confirm password",
  "First name",
  "Last name",
  "Country code",
  "Region",
  "Location",
  "Affiliation",
  "Title",
  "Homepage",
  "Bio",
  "Gender",
  "Birthday",
  "Education",
  "Real name",
  "ID card",
  "Address",
  "Agreement",
  "Captcha",
] as const;

export const SIGNUP_ITEM_DEFAULT_CSS: Record<string, string> = {
  Username: `.login-form input[name="username"]{border-radius:6px}`,
  "Display name": `.login-form input[name="displayName"]{border-radius:6px}`,
  Email: `.login-form input[name="email"]{border-radius:6px}`,
  Phone: `.login-form input[name="phone"]{border-radius:6px}`,
  Password: `.login-form input[name="password"]{border-radius:6px}`,
  "Confirm password": `.login-form input[name="confirmPassword"]{border-radius:6px}`,
  Affiliation: `.login-form input[name="affiliation"]{border-radius:6px}`,
  "ID card": `.login-form input[name="idCard"]{border-radius:6px}`,
  Agreement: `.login-agreement{margin:8px 0;font-size:13px}`,
  Captcha: `.login-captcha{margin:8px 0}`,
};

/** 选择 signin item 时返回默认 customCss（已有值则不覆盖）。 */
export function getSigninItemDefaultCss(name: string): string {
  return SIGNIN_ITEM_DEFAULT_CSS[name] ?? "";
}

/** 选择 signup item 时返回默认 customCss（已有值则不覆盖）。 */
export function getSignupItemDefaultCss(name: string): string {
  return SIGNUP_ITEM_DEFAULT_CSS[name] ?? "";
}

/**
 * 根据 signin item 名称返回 rule 选项（与 LoginSigninForm 消费的 rule 语义对齐）。
 * - Providers: big / small
 * - Captcha: pop up / inline
 * - Forgot password?: Auto sign in - True / False
 */
export function getSigninItemRuleOptions(name: string): Array<{ value: string; label: string }> {
  switch (name) {
    case "Providers":
      return [
        { value: "big", label: "Big icon" },
        { value: "small", label: "Small icon" },
      ];
    case "Captcha":
      return [
        { value: "pop up", label: "Pop up" },
        { value: "inline", label: "Inline" },
      ];
    case "Forgot password?":
      return [
        { value: "None", label: "Auto sign in - True" },
        { value: "Auto sign in - False", label: "Auto sign in - False" },
      ];
    default:
      return [];
  }
}

/**
 * 根据 signup item 名称返回 rule 选项。
 * 通用规则：None / Required / Optional / Read Only；部分字段有专属规则。
 */
export function getSignupItemRuleOptions(name: string): Array<{ value: string; label: string }> {
  switch (name) {
    case "Agreement":
      return [
        { value: "None", label: "None" },
        { value: "Mandatory", label: "Mandatory" },
        { value: "Optional", label: "Optional" },
      ];
    case "Password":
    case "Confirm password":
      return [
        { value: "None", label: "None" },
        { value: "Required", label: "Required" },
        { value: "Read Only", label: "Read Only" },
      ];
    default:
      return [
        { value: "None", label: "None" },
        { value: "Required", label: "Required" },
        { value: "Optional", label: "Optional" },
        { value: "Read Only", label: "Read Only" },
      ];
  }
}

/** 根据 signup item 名称返回推荐输入类型。 */
export function getSignupItemType(name: string): string {
  switch (name) {
    case "Password":
    case "Confirm password":
      return "password";
    case "Email":
      return "email";
    case "Phone":
      return "phone";
    case "Gender":
    case "Country code":
    case "Region":
    case "Education":
      return "select";
    case "Agreement":
      return "checkbox";
    default:
      return "text";
  }
}
