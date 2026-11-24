/**
 * 组织详情页字段矩阵契约测试。
 *
 * 目标：把「不丢失任何表单字段 / 配置项」变成会失败的断言。
 *   1. 渲染出的 `Form.Item` / `Form.List` name 集合 == 基线；
 *   2. 基线中的每个字段都有明确的 payload 落点（或显式排除理由）；
 *   3. 按下保存后，四个真实 payload 的键集合与嵌套键集合 == 基线，且字段值确实落盘。
 *
 * 字段清单从当前代码实际渲染结果取得（含 `enableDarkLogo` / `passwordObfuscatorType`
 * 打开后的条件字段），不抄文档；payload 断言调用真实 `organization-payload.ts` 装配函数。
 */
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import type { FormInstance } from "antd";
import type { ReactNode } from "react";
import { createTestWrapper } from "@/test-utils/render";
import { recordedFormItemNames } from "@/test-utils/record-form-items";
import type { OrganizationDetail } from "@/shared/api/iam/organization";
import OrganizationDetailPage from "./OrganizationDetailPage";

const { mockIdentity, mockDetail, mockSave, state } = vi.hoisted(() => ({
  mockIdentity: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockDetail: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockSave: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  state: { form: undefined as FormInstance | undefined },
}));

vi.mock("antd", async (importOriginal) => {
  const actual = await importOriginal<typeof import("antd")>();
  const { instrumentForm } = await import("@/test-utils/record-form-items");
  const AppWithMock = Object.assign({}, actual.App, {
    useApp: () => ({
      message: {
        success: vi.fn<(...args: unknown[]) => void>(),
        error: vi.fn<(...args: unknown[]) => void>(),
        warning: vi.fn<(...args: unknown[]) => void>(),
        info: vi.fn<(...args: unknown[]) => void>(),
      },
      notification: {},
      modal: {},
    }),
  });
  return {
    ...actual,
    App: AppWithMock,
    Form: instrumentForm(actual.Form as unknown as Record<string, unknown>),
  };
});

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key, i18n: { language: "en" } }),
}));

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    permissions: { menus: [], actions: [], resources: [] },
    isLoading: false,
    hasMenu: () => true,
    hasAction: () => true,
    hasResource: () => true,
    refetch: vi.fn<(...args: unknown[]) => unknown>(),
  }),
}));

vi.mock("@/shared/api/iam/organization", () => ({
  getOrganizationIdentityBySlug: (...args: unknown[]) => mockIdentity(...args),
  getOrganization: (...args: unknown[]) => mockDetail(...args),
  getOrganizationOptions: async () => [],
  getDeploymentRegions: async () => [],
  deleteOrganization: vi.fn<(...args: unknown[]) => void>(),
  saveOrganizationConfiguration: (...args: unknown[]) => mockSave(...args),
}));

vi.mock("@/shared/api/iam/application", () => ({
  getApplicationOptions: async () => [],
}));

vi.mock("@/shared/api/iam/ldap", () => ({
  getIamLdaps: async () => [],
}));

vi.mock("@/shared/api/iam/transaction", () => ({
  getIamTransactions: async () => [],
}));

vi.mock("@/shared/api/iam/b2b", () => ({
  listDomains: async () => [],
  claimDomain: vi.fn<(...args: unknown[]) => void>(),
  verifyDomain: vi.fn<(...args: unknown[]) => void>(),
  deleteDomain: vi.fn<(...args: unknown[]) => void>(),
}));

vi.mock("@/shared/ui/ResourceFormPage", async () => {
  const { Button, Form } = await import("antd");
  return {
    FORM_ITEM_FULL_WIDTH_CLASS: "gl-form-item-full",
    ResourceFormPage: (props: {
      children?: ReactNode;
      form?: FormInstance;
      onFinish?: (values: unknown) => void | Promise<void>;
    }) => {
      const { children, form, onFinish } = props;
      state.form = form;
      recordedFormItemNames.clear();
      return (
        <div data-testid="organization-form">
          <Form form={form} layout="vertical">
            {children}
          </Form>
          <Button
            type="primary"
            onClick={async () => {
              const values = await form!.validateFields();
              await onFinish?.({ ...form!.getFieldsValue(true), ...values });
            }}
          >
            save-organization
          </Button>
        </div>
      );
    },
  };
});

function organizationDetail(overrides: Partial<OrganizationDetail> = {}): OrganizationDetail {
  return {
    id: "org-1",
    parentId: "",
    regionCode: "",
    slug: "my-org",
    displayName: "My Org",
    websiteUrl: "",
    logoUrl: "",
    logoDarkUrl: "",
    faviconUrl: "",
    status: "active",
    passwordType: "argon2id",
    passwordSaltSet: false,
    passwordSaltMasked: "",
    // 打开条件字段：logoDarkUrl 依赖 enableDarkLogo，passwordObfuscatorKey 依赖非 Plain。
    passwordObfuscatorType: "AES",
    passwordObfuscatorKeySet: false,
    passwordObfuscatorKeyMasked: "",
    passwordOptions: [],
    defaultPasswordSet: false,
    defaultPasswordMasked: "",
    enableSoftDeletion: false,
    ipWhitelist: "",
    initiateLoginUrl: "",
    balance: 0,
    currency: "USD",
    disableSignin: false,
    enableDeposit: false,
    balanceCredit: 0,
    mu: 0,
    defaultQuota: 0,
    defaultCurrency: "USD",
    initScore: 0,
    defaultAvatar: "",
    enableDarkLogo: true,
    hasPrivilegeConsent: false,
    ldapAttributes: [],
    settings: {},
    theme: {},
    securityPolicySummary: {},
    masterPasswordSet: false,
    masterPasswordMasked: "",
    masterVerificationCodeSet: false,
    masterVerificationCodeMasked: "",
    isProfilePublic: false,
    useEmailAsUsername: false,
    usePermanentAvatar: false,
    enableTour: false,
    mfaRememberInHours: 12,
    kerberosRealm: "",
    kerberosKdcHost: "",
    kerberosKeytabSet: false,
    kerberosKeytabMasked: "",
    kerberosServiceName: "",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    revision: 1,
    ...overrides,
  };
}

function renderPage() {
  const router = createMemoryRouter(
    [
      { path: "/organizations/:organizationName", element: <OrganizationDetailPage /> },
      { path: "/organizations", element: <div>organizations-list</div> },
    ],
    { initialEntries: ["/organizations/my-org"] },
  );
  return render(<RouterProvider router={router} />, { wrapper: createTestWrapper() });
}

/** 当前代码实际渲染出的唯一字段名（含两个条件字段与 MFA 列表子字段）。 */
const ORGANIZATION_FIELD_MATRIX = [
  "_0.name",
  "_0.rule",
  "_1.name",
  "_1.rule",
  "accountItems",
  "accountMenu",
  "balance",
  "balanceCredit",
  "countryCodes",
  "currency",
  "defaultApplicationId",
  "defaultAvatar",
  "defaultCurrency",
  "defaultPassword",
  "defaultQuota",
  "disableSignin",
  "displayName",
  "enableDarkLogo",
  "enableDeposit",
  "enableSoftDeletion",
  "enableTour",
  "faviconUrl",
  "featureFlag",
  "featureFlagsExtraJson",
  "hasPrivilegeConsent",
  "initScore",
  "initiateLoginUrl",
  "ipPolicyJson",
  "ipWhitelist",
  "isProfilePublic",
  "kerberosKdcHost",
  "kerberosKeytab",
  "kerberosRealm",
  "kerberosServiceName",
  "languages",
  "ldapAttributes",
  "logoDarkUrl",
  "logoUrl",
  "masterPassword",
  "masterVerificationCode",
  "mfaItems",
  "mfaPolicyExtraJson",
  "mfaRememberInHours",
  "mfaRequired",
  "mu",
  "navPreferencesJson",
  "parentId",
  "passwordExpireDays",
  "passwordHistoryCount",
  "passwordMaxLength",
  "passwordMinLength",
  "passwordObfuscatorKey",
  "passwordObfuscatorType",
  "passwordOptions",
  "passwordPolicyExtraJson",
  "passwordRejectCommonPasswords",
  "passwordRejectUsername",
  "passwordRequireDigit",
  "passwordRequireLowercase",
  "passwordRequireSpecial",
  "passwordRequireUppercase",
  "passwordSalt",
  "passwordType",
  "regionCode",
  "riskPolicyJson",
  "sessionIdleTimeoutMinutes",
  "sessionPolicyExtraJson",
  "slug",
  "status",
  "tags",
  "themeAssetsJson",
  "themeCustomCss",
  "themeName",
  "themeTokensJson",
  "useEmailAsUsername",
  "usePermanentAvatar",
  "userTypes",
  "websiteUrl",
].sort();

type PayloadSection = "organization" | "settings" | "theme" | "securityPolicy";
type PathSegment = string | number;

type FieldSpec = {
  field: string;
  value: unknown;
  path: [PayloadSection, ...PathSegment[]];
  /**
   * payload 侧期望值。表单输入与落盘形态不一致时使用：JSON 扩展字段在表单里
   * 是字符串，落盘前被解析成对象，此时 `value` 是表单输入、`payloadValue` 是解析后的值。
   */
  payloadValue?: unknown;
};

/**
 * 每个渲染字段 → 提交 payload 的落点。
 * `slug` 显式排除：它是组织身份的一部分，更新接口不接收，`buildOrganizationUpdatePayload`
 * 也不包含；编辑态下同样禁用输入（`BasicInfoTab`）。
 */
const FIELD_SPECS: FieldSpec[] = [
  { field: "parentId", value: "PARENT_ID", path: ["organization", "parentId"] },
  { field: "regionCode", value: "REGION_CODE", path: ["organization", "regionCode"] },
  { field: "displayName", value: "DISPLAY_NAME", path: ["organization", "displayName"] },
  {
    field: "websiteUrl",
    value: "https://example.com/WEBSITE_URL",
    path: ["organization", "websiteUrl"],
  },
  { field: "logoUrl", value: "https://example.com/LOGO_URL", path: ["organization", "logoUrl"] },
  {
    field: "logoDarkUrl",
    value: "https://example.com/LOGO_DARK_URL",
    path: ["organization", "logoDarkUrl"],
  },
  {
    field: "faviconUrl",
    value: "https://example.com/FAVICON_URL",
    path: ["organization", "faviconUrl"],
  },
  { field: "status", value: "disabled", path: ["organization", "status"] },
  { field: "passwordType", value: "bcrypt", path: ["organization", "passwordType"] },
  {
    field: "passwordObfuscatorType",
    value: "AES",
    path: ["organization", "passwordObfuscatorType"],
  },
  { field: "passwordSalt", value: "PASSWORD_SALT", path: ["organization", "passwordSalt"] },
  {
    field: "passwordObfuscatorKey",
    value: "OBFUSCATOR_KEY",
    path: ["organization", "passwordObfuscatorKey"],
  },
  {
    field: "defaultPassword",
    value: "DEFAULT_PASSWORD",
    path: ["organization", "defaultPassword"],
  },
  { field: "masterPassword", value: "MASTER_PASSWORD", path: ["organization", "masterPassword"] },
  {
    field: "masterVerificationCode",
    value: "MASTER_CODE",
    path: ["organization", "masterVerificationCode"],
  },
  { field: "kerberosKeytab", value: "KEYTAB", path: ["organization", "kerberosKeytab"] },
  { field: "ipWhitelist", value: "IP_WHITELIST", path: ["organization", "ipWhitelist"] },
  {
    field: "initiateLoginUrl",
    value: "https://example.com/INITIATE_URL",
    path: ["organization", "initiateLoginUrl"],
  },
  { field: "currency", value: "CNY", path: ["organization", "currency"] },
  { field: "defaultCurrency", value: "CNY", path: ["organization", "defaultCurrency"] },
  {
    field: "defaultAvatar",
    value: "https://example.com/AVATAR",
    path: ["organization", "defaultAvatar"],
  },
  { field: "kerberosRealm", value: "REALM", path: ["organization", "kerberosRealm"] },
  { field: "kerberosKdcHost", value: "KDC", path: ["organization", "kerberosKdcHost"] },
  { field: "kerberosServiceName", value: "SERVICE", path: ["organization", "kerberosServiceName"] },
  { field: "enableSoftDeletion", value: true, path: ["organization", "enableSoftDeletion"] },
  { field: "disableSignin", value: true, path: ["organization", "disableSignin"] },
  { field: "enableDeposit", value: true, path: ["organization", "enableDeposit"] },
  { field: "enableDarkLogo", value: true, path: ["organization", "enableDarkLogo"] },
  { field: "hasPrivilegeConsent", value: true, path: ["organization", "hasPrivilegeConsent"] },
  { field: "useEmailAsUsername", value: true, path: ["organization", "useEmailAsUsername"] },
  { field: "usePermanentAvatar", value: true, path: ["organization", "usePermanentAvatar"] },
  { field: "isProfilePublic", value: true, path: ["organization", "isProfilePublic"] },
  { field: "enableTour", value: true, path: ["organization", "enableTour"] },
  { field: "balance", value: 11, path: ["organization", "balance"] },
  { field: "balanceCredit", value: 12, path: ["organization", "balanceCredit"] },
  { field: "mu", value: 13, path: ["organization", "mu"] },
  { field: "defaultQuota", value: 14, path: ["organization", "defaultQuota"] },
  { field: "initScore", value: 15, path: ["organization", "initScore"] },
  { field: "passwordOptions", value: ["OPTION_A"], path: ["organization", "passwordOptions"] },
  { field: "ldapAttributes", value: ["LDAP_ATTR"], path: ["organization", "ldapAttributes"] },
  { field: "accountMenu", value: "ACCOUNT_MENU", path: ["settings", "accountMenu"] },
  {
    field: "defaultApplicationId",
    value: "APPLICATION_ID",
    path: ["settings", "defaultApplicationId"],
  },
  { field: "mfaRememberInHours", value: 18, path: ["settings", "mfaRememberInHours"] },
  { field: "featureFlag", value: true, path: ["settings", "featureFlags", "iam"] },
  {
    field: "featureFlagsExtraJson",
    value: '{"sentinel_ff":"FF"}',
    payloadValue: "FF",
    path: ["settings", "featureFlags", "sentinel_ff"],
  },
  {
    field: "navPreferencesJson",
    value: '{"sentinel_nav":"NAV"}',
    payloadValue: "NAV",
    path: ["settings", "navPreferences", "sentinel_nav"],
  },
  { field: "languages", value: ["LANG_A"], path: ["settings", "languages"] },
  { field: "countryCodes", value: ["COUNTRY_A"], path: ["settings", "countryCodes"] },
  { field: "userTypes", value: ["USER_TYPE_A"], path: ["settings", "userTypes"] },
  { field: "tags", value: ["TAG_A"], path: ["settings", "tags"] },
  {
    field: "accountItems",
    value: [{ name: "ITEM_A", visible: true, viewRule: "", modifyRule: "", regex: "", tab: "" }],
    path: ["settings", "accountItems"],
  },
  {
    field: "mfaItems",
    // mfaRequired=true 时至少需要一个 Required 方法，否则保存会被闭环校验拦截。
    value: [{ name: "MFA_ITEM_A", rule: "Required" }],
    path: ["settings", "mfaItems"],
  },
  { field: "themeName", value: "THEME_NAME", path: ["theme", "themeName"] },
  { field: "themeCustomCss", value: "CSS_RULES", path: ["theme", "customCss"] },
  {
    field: "themeTokensJson",
    value: '{"sentinel_tok":"TOK"}',
    payloadValue: "TOK",
    path: ["theme", "tokens", "sentinel_tok"],
  },
  {
    field: "themeAssetsJson",
    value: '{"sentinel_asset":"ASSET"}',
    payloadValue: "ASSET",
    path: ["theme", "assets", "sentinel_asset"],
  },
  {
    field: "passwordMinLength",
    value: 16,
    path: ["securityPolicy", "passwordPolicy", "minLength"],
  },
  {
    field: "passwordExpireDays",
    value: 17,
    path: ["securityPolicy", "passwordPolicy", "passwordMaxAgeDays"],
  },
  {
    field: "passwordPolicyExtraJson",
    value: '{"sentinel_pp":"PP"}',
    payloadValue: "PP",
    path: ["securityPolicy", "passwordPolicy", "sentinel_pp"],
  },
  {
    field: "passwordMaxLength",
    value: 64,
    path: ["securityPolicy", "passwordPolicy", "maxLength"],
  },
  {
    field: "passwordHistoryCount",
    value: 7,
    path: ["securityPolicy", "passwordPolicy", "passwordHistoryCount"],
  },
  {
    field: "passwordRequireUppercase",
    value: false,
    path: ["securityPolicy", "passwordPolicy", "requireUppercase"],
  },
  {
    field: "passwordRequireLowercase",
    value: false,
    path: ["securityPolicy", "passwordPolicy", "requireLowercase"],
  },
  {
    field: "passwordRequireDigit",
    value: false,
    path: ["securityPolicy", "passwordPolicy", "requireDigit"],
  },
  {
    field: "passwordRequireSpecial",
    value: false,
    path: ["securityPolicy", "passwordPolicy", "requireSpecial"],
  },
  {
    field: "passwordRejectCommonPasswords",
    value: false,
    path: ["securityPolicy", "passwordPolicy", "rejectCommonPasswords"],
  },
  {
    field: "passwordRejectUsername",
    value: false,
    path: ["securityPolicy", "passwordPolicy", "rejectUsername"],
  },
  { field: "mfaRequired", value: true, path: ["securityPolicy", "mfaPolicy", "required"] },
  {
    field: "mfaPolicyExtraJson",
    value: '{"sentinel_mp":"MP"}',
    payloadValue: "MP",
    path: ["securityPolicy", "mfaPolicy", "sentinel_mp"],
  },
  {
    field: "sessionIdleTimeoutMinutes",
    value: 19,
    path: ["securityPolicy", "sessionPolicy", "idleTimeoutMinutes"],
  },
  {
    field: "sessionPolicyExtraJson",
    value: '{"sentinel_sp":"SP"}',
    payloadValue: "SP",
    path: ["securityPolicy", "sessionPolicy", "sentinel_sp"],
  },
  {
    field: "ipPolicyJson",
    value: '{"sentinel_ip":"IP"}',
    payloadValue: "IP",
    path: ["securityPolicy", "ipPolicy", "sentinel_ip"],
  },
  {
    field: "riskPolicyJson",
    value: '{"sentinel_risk":"RISK"}',
    payloadValue: "RISK",
    path: ["securityPolicy", "riskPolicy", "sentinel_risk"],
  },
];

function readPath(root: unknown, path: PathSegment[]): unknown {
  let current: unknown = root;
  for (const segment of path) {
    if (current === null || typeof current !== "object") return undefined;
    current = (current as Record<PathSegment, unknown>)[segment];
  }
  return current;
}

function sectionOf(
  payload: Record<PayloadSection, unknown>,
  path: [PayloadSection, ...PathSegment[]],
) {
  const [section, ...rest] = path;
  return readPath(payload[section], rest);
}

beforeEach(() => {
  vi.clearAllMocks();
  recordedFormItemNames.clear();
  mockIdentity.mockResolvedValue({ id: "org-1", slug: "my-org", displayName: "My Org" });
  mockDetail.mockResolvedValue(organizationDetail());
  mockSave.mockResolvedValue(undefined);
});

describe("organization field matrix contract", () => {
  it("渲染字段集合等于基线，且 Domains 子页不贡献主表单字段", async () => {
    renderPage();
    await waitFor(() => expect(state.form).toBeDefined());
    await screen.findByTestId("organization-form");
    // DomainsTab 在权限允许时真实渲染，但只有子资源操作，不注册任何 Form.Item。
    expect(await screen.findByText("b2b.claimTitle")).toBeInTheDocument();

    await waitFor(() => expect(state.form!.getFieldValue("displayName")).toBe("My Org"));

    const rendered = Array.from(recordedFormItemNames).sort();
    expect(rendered).toEqual(ORGANIZATION_FIELD_MATRIX);
    expect(rendered).not.toContain("domain");
  });

  it("每个渲染字段都能落到真实 payload；payload 键集合与基线一致", async () => {
    renderPage();
    await waitFor(() => expect(state.form).toBeDefined());
    await screen.findByTestId("organization-form");
    await waitFor(() => expect(state.form!.getFieldValue("displayName")).toBe("My Org"));

    const formValues = Object.fromEntries(FIELD_SPECS.map((spec) => [spec.field, spec.value]));
    state.form!.setFieldsValue(formValues as never);

    await userEvent.click(screen.getByRole("button", { name: "save-organization" }));
    await waitFor(() => expect(mockSave).toHaveBeenCalled());

    const [savedOrgId, payload] = mockSave.mock.calls[0] as [
      string,
      Record<PayloadSection, Record<string, unknown>>,
    ];
    expect(savedOrgId).toBe("org-1");

    // 1) 值确实落盘：每个字段在自己的声明路径上取到写入值。
    for (const spec of FIELD_SPECS) {
      expect({ field: spec.field, value: sectionOf(payload, spec.path) }).toEqual({
        field: spec.field,
        value: spec.payloadValue ?? spec.value,
      });
    }

    // 2) 键集合锁定：任何一侧新增/遗漏字段都会改变集合而失败。
    expect(Object.keys(payload.organization).sort()).toEqual(
      [
        "parentId",
        "regionCode",
        "displayName",
        "websiteUrl",
        "logoUrl",
        "logoDarkUrl",
        "faviconUrl",
        "status",
        "passwordType",
        "passwordSalt",
        "passwordObfuscatorType",
        "passwordObfuscatorKey",
        "defaultPassword",
        "masterPassword",
        "masterVerificationCode",
        "kerberosKeytab",
        "passwordOptions",
        "enableSoftDeletion",
        "ipWhitelist",
        "initiateLoginUrl",
        "balance",
        "currency",
        "disableSignin",
        "enableDeposit",
        "balanceCredit",
        "mu",
        "defaultQuota",
        "defaultCurrency",
        "initScore",
        "defaultAvatar",
        "enableDarkLogo",
        "hasPrivilegeConsent",
        "useEmailAsUsername",
        "usePermanentAvatar",
        "isProfilePublic",
        "enableTour",
        "kerberosRealm",
        "kerberosKdcHost",
        "kerberosServiceName",
        "ldapAttributes",
        "revision",
      ].sort(),
    );
    expect(Object.keys(payload.settings).sort()).toEqual(
      [
        "languages",
        "defaultApplicationId",
        "accountItems",
        "mfaItems",
        "accountMenu",
        "mfaRememberInHours",
        "dcrPolicy",
        "navPreferences",
        "featureFlags",
        "countryCodes",
        "userTypes",
        "tags",
      ].sort(),
    );
    expect(Object.keys(payload.theme).sort()).toEqual(
      ["themeName", "tokens", "customCss", "assets"].sort(),
    );
    expect(Object.keys(payload.securityPolicy).sort()).toEqual(
      [
        "passwordPolicy",
        "passwordDeriveAlgorithm",
        "mfaPolicy",
        "ipPolicy",
        "sessionPolicy",
        "riskPolicy",
      ].sort(),
    );

    // 3) 显式排除项：slug 不属于更新 payload；无 UI 字段仍由后端直通。
    expect(payload.organization).not.toHaveProperty("slug");
    expect(payload.settings.dcrPolicy).toBe("");
    // navPreferences 不再是「无 UI 直通字段」：导航树已注册 navPreferencesJson 表单字段。
    expect(payload.settings.navPreferences).toEqual({ sentinel_nav: "NAV" });
    expect(payload.securityPolicy.passwordDeriveAlgorithm).toBe("");
  });
});
