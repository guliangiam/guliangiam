/**
 * Organization 详情页表单值类型
 *
 * 从 OrganizationDetailPage.tsx 拆分而来。表单值 ≠ API 响应：API 把
 * `settings` / `theme` / `securityPolicySummary` 作为三个嵌套对象返回，
 * 表单则把它们展平成扁平字段，提交时再重新拼装
 * （见 ../organization-payload.ts）。
 */

export type OrganizationAccountItemFormValue = {
  name?: string;
  visible?: boolean;
  viewRule?: string;
  modifyRule?: string;
  regex?: string;
  tab?: string;
};

export type OrganizationMfaItemFormValue = {
  name?: string;
  rule?: string;
};

export type OrganizationFormValues = {
  parentId?: string;
  regionCode?: string;
  slug: string;
  displayName?: string;
  websiteUrl?: string;
  logoUrl?: string;
  logoDarkUrl?: string;
  faviconUrl?: string;
  status?: string;
  passwordType?: string;
  passwordSalt?: string;
  passwordObfuscatorType?: string;
  passwordObfuscatorKey?: string;
  passwordOptions?: string[];
  defaultPassword?: string;
  enableSoftDeletion?: boolean;
  ipWhitelist?: string;
  initiateLoginUrl?: string;
  balance?: number;
  currency?: string;
  disableSignin?: boolean;
  enableDeposit?: boolean;
  balanceCredit?: number;
  mu?: number;
  defaultQuota?: number;
  defaultCurrency?: string;
  initScore?: number;
  defaultAvatar?: string;
  enableDarkLogo?: boolean;
  hasPrivilegeConsent?: boolean;
  ldapAttributes?: string[];
  masterPassword?: string;
  masterVerificationCode?: string;
  isProfilePublic?: boolean;
  useEmailAsUsername?: boolean;
  usePermanentAvatar?: boolean;
  accountMenu?: string;
  mfaRememberInHours?: number;
  enableTour?: boolean;
  mfaItems?: OrganizationMfaItemFormValue[];
  kerberosRealm?: string;
  kerberosKdcHost?: string;
  kerberosKeytab?: string;
  kerberosServiceName?: string;
  languages?: string[];
  defaultApplicationId?: string;
  countryCodes?: string[];
  userTypes?: string[];
  tags?: string[];
  accountItems?: OrganizationAccountItemFormValue[];
  navPreferencesJson?: string;
  featureFlag?: boolean;
  featureFlagsExtraJson?: string;
  themeName?: string;
  themeTokensJson?: string;
  themeCustomCss?: string;
  themeAssetsJson?: string;
  passwordMinLength?: number;
  passwordMaxLength?: number;
  passwordExpireDays?: number;
  passwordHistoryCount?: number;
  passwordRequireUppercase?: boolean;
  passwordRequireLowercase?: boolean;
  passwordRequireDigit?: boolean;
  passwordRequireSpecial?: boolean;
  passwordRejectCommonPasswords?: boolean;
  passwordRejectUsername?: boolean;
  passwordPolicyExtraJson?: string;
  mfaRequired?: boolean;
  mfaPolicyExtraJson?: string;
  ipPolicyJson?: string;
  sessionIdleTimeoutMinutes?: number;
  sessionPolicyExtraJson?: string;
  riskPolicyJson?: string;
  // 无独立 UI，但保存时必须原样回传，否则子表端点会整行覆盖导致字段被清空。
  passwordDeriveAlgorithm?: string;
  dcrPolicy?: string;
};
