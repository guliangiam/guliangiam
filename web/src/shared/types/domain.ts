/**
 * Domain types — shared auth/login flow schemas.
 *
 * These schemas use the standard API field naming convention (owner, name, etc.)
 * and are consumed by auth flow pages and libraries.
 *
 * For GuliangIAM native API types, import from @/shared/api/iam/* instead:
 *   - UserDetail, OrganizationDetail, ApplicationDetail, ProviderDetail
 *
 * @deprecated New code should use types from @/shared/api/iam/* modules.
 */
import { z } from "zod";

const resourceListSchema = <T extends z.ZodTypeAny>(itemSchema: T) =>
  z.object({
    items: z.array(itemSchema),
    total: z.number(),
  });

export const sessionOrganizationSchema = z.object({
  id: z.string().optional().nullable(),
  name: z.string(),
  displayName: z.string().optional().nullable(),
  logoUrl: z.string().optional().nullable(),
  faviconUrl: z.string().optional().nullable(),
  navItems: z
    .array(z.string())
    .optional()
    .nullable()
    .transform((value) => value ?? []),
  userNavItems: z
    .array(z.string())
    .optional()
    .nullable()
    .transform((value) => value ?? []),
  navGroupOrder: z
    .array(z.string())
    .optional()
    .nullable()
    .transform((value) => value ?? []),
});

export const sessionAccountSchema = z.object({
  id: z.string().optional(),
  owner: z.string(),
  name: z.string(),
  displayName: z.string().optional().nullable(),
  avatar: z.string().optional().nullable(),
  isAdmin: z.boolean().optional().default(false),
  organization: z
    .object({
      id: z.string(),
      name: z.string().optional().nullable(),
      displayName: z.string().optional().nullable(),
    })
    .optional()
    .nullable(),
});

export const signinItemSchema = z
  .object({
    name: z.string().optional().nullable(),
    label: z.string().optional().nullable(),
    visible: z.boolean().optional().nullable(),
    rule: z.string().optional().nullable(),
    customCss: z.string().optional().nullable(),
    isCustom: z.boolean().optional().nullable(),
  })
  .passthrough();

export const signinMethodSchema = z
  .object({
    name: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    rule: z.string().optional().nullable(),
  })
  .passthrough();

export const applicationOrganizationSchema = z
  .object({
    languages: z.array(z.string()).optional().nullable(),
    countryCodes: z.array(z.string()).optional().nullable(),
  })
  .passthrough();

export const applicationProviderSchema = z
  .object({
    owner: z.string().optional().nullable(),
    name: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    category: z.string().optional().nullable(),
    type: z.string().optional().nullable(),
    clientId: z.string().optional().nullable(),
    clientId2: z.string().optional().nullable(),
    clientSecret2: z.string().optional().nullable(),
    domain: z.string().optional().nullable(),
    subType: z.string().optional().nullable(),
    method: z.string().optional().nullable(),
    disableSsl: z.boolean().optional().nullable(),
    signName: z.string().optional().nullable(),
    scopes: z.string().optional().nullable(),
  })
  .passthrough();

export const applicationProviderItemSchema = z
  .object({
    provider: applicationProviderSchema.optional().nullable(),
    rule: z.string().optional().nullable(),
    prompted: z.boolean().optional().nullable(),
    canSignIn: z.boolean().optional().nullable(),
    canSignUp: z.boolean().optional().nullable(),
  })
  .passthrough();

const countSeriesSchema = z.array(z.number());

export const dashboardSchema = z.object({
  organizationCounts: countSeriesSchema,
  userCounts: countSeriesSchema,
  providerCounts: countSeriesSchema,
  applicationCounts: countSeriesSchema,
  subscriptionCounts: countSeriesSchema,
  roleCounts: countSeriesSchema,
  groupCounts: countSeriesSchema,
  resourceCounts: countSeriesSchema,
  certCounts: countSeriesSchema,
  permissionCounts: countSeriesSchema,
  transactionCounts: countSeriesSchema,
  modelCounts: countSeriesSchema,
  adapterCounts: countSeriesSchema,
  enforcerCounts: countSeriesSchema,
});

export const userSchema = z.object({
  owner: z.string().optional(),
  name: z.string(),
  displayName: z.string().optional().nullable(),
  email: z.string().optional().nullable(),
  phone: z.string().optional().nullable(),
  tag: z.string().optional().nullable(),
});

export const mfaPropsSchema = z
  .object({
    enabled: z.boolean().optional().nullable(),
    isPreferred: z.boolean().optional().nullable(),
    mfaType: z.string().optional().nullable(),
    secret: z.string().optional().nullable(),
    countryCode: z.string().optional().nullable(),
    url: z.string().optional().nullable(),
    recoveryCodes: z.array(z.string()).optional().nullable(),
    mfaRememberInHours: z.union([z.string(), z.number()]).optional().nullable(),
  })
  .passthrough();

export const mfaAccountSchema = z
  .object({
    accountName: z.string().optional().nullable(),
    issuer: z.string().optional().nullable(),
    secretKey: z.string().optional().nullable(),
    origin: z.string().optional().nullable(),
  })
  .passthrough();

export const mfaItemSchema = z
  .object({
    name: z.string().optional().nullable(),
    rule: z.string().optional().nullable(),
  })
  .passthrough();

export const userDetailSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    updatedTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    avatar: z.string().optional().nullable(),
    email: z.string().optional().nullable(),
    phone: z.string().optional().nullable(),
    countryCode: z.string().optional().nullable(),
    type: z.string().optional().nullable(),
    tag: z.string().optional().nullable(),
    signupApplication: z.string().optional().nullable(),
    lastSigninTime: z.string().optional().nullable(),
    lastChangePasswordTime: z.string().optional().nullable(),
    preferredMfaType: z.string().optional().nullable(),
    recoveryCodes: z.array(z.string()).optional().nullable(),
    mfaPhoneEnabled: z.boolean().optional().nullable(),
    mfaEmailEnabled: z.boolean().optional().nullable(),
    mfaRadiusEnabled: z.boolean().optional().nullable(),
    mfaRadiusUsername: z.string().optional().nullable(),
    mfaRadiusProvider: z.string().optional().nullable(),
    mfaPushEnabled: z.boolean().optional().nullable(),
    mfaPushReceiver: z.string().optional().nullable(),
    mfaPushProvider: z.string().optional().nullable(),
    multiFactorAuths: z.array(mfaPropsSchema).optional().nullable(),
    mfaAccounts: z.array(mfaAccountSchema).optional().nullable(),
    mfaItems: z.array(mfaItemSchema).optional().nullable(),
    isAdmin: z.boolean().optional().nullable(),
    isForbidden: z.boolean().optional().nullable(),
  })
  .passthrough();

export const applicationSchema = z
  .object({
    id: z.string().optional().nullable(),
    // resolve/create 响应都会带组织 UUID（ApplicationListItem.OrganizationID）；
    // QR 扫码登录的 create 需要它，所以显式建模而不是靠 passthrough 兜着。
    organizationId: z.string().optional().nullable(),
    owner: z.string().optional(),
    organization: z.string().optional().nullable(),
    name: z.string(),
    clientId: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    description: z.string().optional().nullable(),
    logo: z.string().optional().nullable(),
    logoDark: z.string().optional().nullable(),
    homepageUrl: z.string().optional().nullable(),
    termsOfUse: z.string().optional().nullable(),
    forgetUrl: z.string().optional().nullable(),
    formBackgroundUrl: z.string().optional().nullable(),
    formBackgroundUrlMobile: z.string().optional().nullable(),
    formCss: z.string().optional().nullable(),
    formCssMobile: z.string().optional().nullable(),
    formOffset: z.union([z.string(), z.number()]).optional().nullable(),
    formSideHtml: z.string().optional().nullable(),
    signinItems: z.array(signinItemSchema).optional().nullable(),
    signupItems: z.array(signinItemSchema).optional().nullable(),
    signinMethods: z.array(z.string()).optional().nullable(),
    organizationObj: applicationOrganizationSchema.optional().nullable(),
    providers: z
      .array(z.union([applicationProviderItemSchema, z.string()]))
      .optional()
      .nullable(),
  })
  .passthrough();

export const providerSchema = z.object({
  id: z.string().optional().nullable(),
  owner: z.string().optional(),
  name: z.string(),
  category: z.string().optional().nullable(),
  type: z.string().optional().nullable(),
  clientId: z.string().optional().nullable(),
});

export const organizationSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    displayName: z.string().optional().nullable(),
    websiteUrl: z.string().optional().nullable(),
    logoUrl: z.string().optional().nullable(),
    logoDarkUrl: z.string().optional().nullable(),
    faviconUrl: z.string().optional().nullable(),
    defaultApplication: z.string().optional().nullable(),
    defaultAvatar: z.string().optional().nullable(),
    description: z.string().optional().nullable(),
    createdAt: z.string().optional().nullable(),
    updatedAt: z.string().optional().nullable(),
  })
  .passthrough();

export const groupSchema = z
  .object({
    id: z.string().optional().nullable(),
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    updatedTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    type: z.string().optional().nullable(),
    parentId: z.string().optional().nullable(),
    parentName: z.string().optional().nullable(),
    users: z.array(z.string()).optional().nullable(),
    isTopGroup: z.boolean().optional().nullable(),
    isEnabled: z.boolean().optional().nullable(),
    haveChildren: z.boolean().optional().nullable(),
  })
  .passthrough();

export const invitationSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    updatedTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    code: z.string().optional().nullable(),
    defaultCode: z.string().optional().nullable(),
    quota: z.union([z.string(), z.number()]).optional().nullable(),
    usedCount: z.union([z.string(), z.number()]).optional().nullable(),
    application: z.string().optional().nullable(),
    username: z.string().optional().nullable(),
    email: z.string().optional().nullable(),
    phone: z.string().optional().nullable(),
    signupGroup: z.string().optional().nullable(),
    state: z.string().optional().nullable(),
    tag: z.string().optional().nullable(),
  })
  .passthrough();

export const resourceSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    user: z.string().optional().nullable(),
    provider: z.string().optional().nullable(),
    application: z.string().optional().nullable(),
    tag: z.string().optional().nullable(),
    parent: z.string().optional().nullable(),
    fileName: z.string().optional().nullable(),
    fileType: z.string().optional().nullable(),
    fileFormat: z.string().optional().nullable(),
    fileSize: z.union([z.string(), z.number()]).optional().nullable(),
    url: z.string().optional().nullable(),
    description: z.string().optional().nullable(),
  })
  .passthrough();

export const certSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    scope: z.string().optional().nullable(),
    type: z.string().optional().nullable(),
    cryptoAlgorithm: z.string().optional().nullable(),
    bitSize: z.union([z.string(), z.number()]).optional().nullable(),
    expireInYears: z.union([z.string(), z.number()]).optional().nullable(),
    expireTime: z.string().optional().nullable(),
    domainExpireTime: z.string().optional().nullable(),
    provider: z.string().optional().nullable(),
    account: z.string().optional().nullable(),
    accessKey: z.string().optional().nullable(),
    accessSecret: z.string().optional().nullable(),
    certificate: z.string().optional().nullable(),
    privateKey: z.string().optional().nullable(),
  })
  .passthrough();

export const serverSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    updatedTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    url: z.string().optional().nullable(),
    application: z.string().optional().nullable(),
  })
  .passthrough();

export const ruleExpressionSchema = z
  .object({
    name: z.string().optional().nullable(),
    operator: z.string().optional().nullable(),
    value: z.string().optional().nullable(),
  })
  .passthrough();

export const ruleSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    updatedTime: z.string().optional().nullable(),
    type: z.string().optional().nullable(),
    expressions: z.array(ruleExpressionSchema).optional().nullable(),
    action: z.string().optional().nullable(),
    statusCode: z.union([z.string(), z.number()]).optional().nullable(),
    reason: z.string().optional().nullable(),
    isVerbose: z.boolean().optional().nullable(),
  })
  .passthrough();

export const siteNodeItemSchema = z
  .object({
    name: z.string().optional().nullable(),
    version: z.string().optional().nullable(),
    diff: z.string().optional().nullable(),
    pid: z.union([z.string(), z.number()]).optional().nullable(),
    status: z.string().optional().nullable(),
    message: z.string().optional().nullable(),
    provider: z.string().optional().nullable(),
  })
  .passthrough();

export const siteSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    updatedTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    tag: z.string().optional().nullable(),
    domain: z.string().optional().nullable(),
    otherDomains: z.array(z.string()).optional().nullable(),
    needRedirect: z.boolean().optional().nullable(),
    disableVerbose: z.boolean().optional().nullable(),
    rules: z.array(z.string()).optional().nullable(),
    enableAlert: z.boolean().optional().nullable(),
    alertInterval: z.union([z.string(), z.number()]).optional().nullable(),
    alertTryTimes: z.union([z.string(), z.number()]).optional().nullable(),
    alertProviders: z.array(z.string()).optional().nullable(),
    challenges: z.array(z.string()).optional().nullable(),
    host: z.string().optional().nullable(),
    port: z.union([z.string(), z.number()]).optional().nullable(),
    hosts: z.array(z.string()).optional().nullable(),
    sslMode: z.string().optional().nullable(),
    sslCert: z.string().optional().nullable(),
    publicIp: z.string().optional().nullable(),
    node: z.string().optional().nullable(),
    isSelf: z.boolean().optional().nullable(),
    status: z.string().optional().nullable(),
    nodes: z.array(siteNodeItemSchema).optional().nullable(),
    guliangiamApplication: z.string().optional().nullable(),
  })
  .passthrough();

export const formItemSchema = z
  .object({
    name: z.string().optional().nullable(),
    label: z.string().optional().nullable(),
    visible: z.boolean().optional().nullable(),
    width: z.string().optional().nullable(),
  })
  .passthrough();

export const formSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    type: z.string().optional().nullable(),
    tag: z.string().optional().nullable(),
    formItems: z.array(formItemSchema).optional().nullable(),
  })
  .passthrough();

export const webhookHeaderSchema = z
  .object({
    name: z.string().optional().nullable(),
    value: z.string().optional().nullable(),
  })
  .passthrough();

export const webhookSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    organization: z.string().optional().nullable(),
    url: z.string().optional().nullable(),
    method: z.string().optional().nullable(),
    contentType: z.string().optional().nullable(),
    headers: z.array(webhookHeaderSchema).optional().nullable(),
    events: z.array(z.string()).optional().nullable(),
    tokenFields: z.array(z.string()).optional().nullable(),
    objectFields: z.array(z.string()).optional().nullable(),
    isUserExtended: z.boolean().optional().nullable(),
    singleOrgOnly: z.boolean().optional().nullable(),
    isEnabled: z.boolean().optional().nullable(),
  })
  .passthrough();

export const ticketMessageSchema = z
  .object({
    author: z.string().optional().nullable(),
    text: z.string().optional().nullable(),
    timestamp: z.string().optional().nullable(),
    isAdmin: z.boolean().optional().nullable(),
  })
  .passthrough();

export const ticketSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    updatedTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    user: z.string().optional().nullable(),
    title: z.string().optional().nullable(),
    content: z.string().optional().nullable(),
    state: z.string().optional().nullable(),
    messages: z.array(ticketMessageSchema).optional().nullable(),
  })
  .passthrough();

export const ldapSchema = z
  .object({
    id: z.string().optional().nullable(),
    owner: z.string().optional().nullable(),
    createdTime: z.string().optional().nullable(),
    serverName: z.string().optional().nullable(),
    host: z.string().optional().nullable(),
    port: z.union([z.string(), z.number()]).optional().nullable(),
    enableSsl: z.boolean().optional().nullable(),
    allowSelfSignedCert: z.boolean().optional().nullable(),
    username: z.string().optional().nullable(),
    password: z.string().optional().nullable(),
    baseDn: z.string().optional().nullable(),
    filter: z.string().optional().nullable(),
    filterFields: z.array(z.string()).optional().nullable(),
    defaultGroup: z.string().optional().nullable(),
    passwordType: z.string().optional().nullable(),
    customAttributes: z.record(z.string(), z.string()).optional().nullable(),
    autoSync: z.union([z.string(), z.number()]).optional().nullable(),
    lastSync: z.string().optional().nullable(),
    enableGroups: z.boolean().optional().nullable(),
  })
  .passthrough();

export const ldapUserSchema = z
  .object({
    cn: z.string().optional().nullable(),
    uid: z.string().optional().nullable(),
    uidNumber: z.string().optional().nullable(),
    groupId: z.string().optional().nullable(),
    email: z.string().optional().nullable(),
    mobile: z.string().optional().nullable(),
    address: z.string().optional().nullable(),
    uuid: z.string().optional().nullable(),
  })
  .passthrough();

export const ldapUsersDataSchema = z
  .object({
    users: z.array(ldapUserSchema).optional().nullable(),
    existUuids: z.array(z.string()).optional().nullable(),
  })
  .passthrough();

export const ldapSyncResultSchema = z
  .object({
    exist: z.array(ldapUserSchema).optional().nullable(),
    failed: z.array(ldapUserSchema).optional().nullable(),
  })
  .passthrough();

export const syncerTableColumnSchema = z
  .object({
    name: z.string().optional().nullable(),
    type: z.string().optional().nullable(),
    guliangiamName: z.string().optional().nullable(),
    isKey: z.boolean().optional().nullable(),
    isHashed: z.boolean().optional().nullable(),
    values: z.array(z.string()).optional().nullable(),
  })
  .passthrough();

export const syncerSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    organization: z.string().optional().nullable(),
    type: z.string().optional().nullable(),
    databaseType: z.string().optional().nullable(),
    sslMode: z.string().optional().nullable(),
    sshType: z.string().optional().nullable(),
    host: z.string().optional().nullable(),
    port: z.union([z.string(), z.number()]).optional().nullable(),
    user: z.string().optional().nullable(),
    password: z.string().optional().nullable(),
    sshHost: z.string().optional().nullable(),
    sshPort: z.union([z.string(), z.number()]).optional().nullable(),
    sshUser: z.string().optional().nullable(),
    sshPassword: z.string().optional().nullable(),
    cert: z.string().optional().nullable(),
    database: z.string().optional().nullable(),
    table: z.string().optional().nullable(),
    tableColumns: z.array(syncerTableColumnSchema).optional().nullable(),
    affiliationTable: z.string().optional().nullable(),
    avatarBaseUrl: z.string().optional().nullable(),
    errorText: z.string().optional().nullable(),
    syncInterval: z.union([z.string(), z.number()]).optional().nullable(),
    isReadOnly: z.boolean().optional().nullable(),
    isEnabled: z.boolean().optional().nullable(),
  })
  .passthrough();

export const roleSchema = z
  .object({
    id: z.string().optional().nullable(),
    owner: z.string().optional(),
    name: z.string(),
    displayName: z.string().optional().nullable(),
    description: z.string().optional().nullable(),
    users: z.array(z.string()).optional().nullable(),
    groups: z.array(z.string()).optional().nullable(),
    roles: z.array(z.string()).optional().nullable(),
    domains: z.array(z.string()).optional().nullable(),
    isEnabled: z.boolean().optional().nullable(),
  })
  .passthrough();

export const permissionSchema = z
  .object({
    id: z.string().optional().nullable(),
    owner: z.string().optional(),
    name: z.string(),
    displayName: z.string().optional().nullable(),
    description: z.string().optional().nullable(),
    model: z.string().optional().nullable(),
    users: z.array(z.string()).optional().nullable(),
    groups: z.array(z.string()).optional().nullable(),
    roles: z.array(z.string()).optional().nullable(),
    domains: z.array(z.string()).optional().nullable(),
    resourceType: z.string().optional().nullable(),
    resources: z.array(z.string()).optional().nullable(),
    actions: z.array(z.string()).optional().nullable(),
    effect: z.string().optional().nullable(),
    state: z.string().optional().nullable(),
    submitter: z.string().optional().nullable(),
    isEnabled: z.boolean().optional().nullable(),
  })
  .passthrough();

export const modelSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    displayName: z.string().optional().nullable(),
    description: z.string().optional().nullable(),
    createdTime: z.string().optional().nullable(),
    modelText: z.string().optional().nullable(),
  })
  .passthrough();

export const adapterSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    description: z.string().optional().nullable(),
    createdTime: z.string().optional().nullable(),
    table: z.string().optional().nullable(),
    useSameDb: z.boolean().optional().nullable(),
    type: z.string().optional().nullable(),
    databaseType: z.string().optional().nullable(),
    host: z.string().optional().nullable(),
    port: z.union([z.string(), z.number()]).optional().nullable(),
    user: z.string().optional().nullable(),
    password: z.string().optional().nullable(),
    database: z.string().optional().nullable(),
  })
  .passthrough();

export const enforcerSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    description: z.string().optional().nullable(),
    model: z.string().optional().nullable(),
    adapter: z.string().optional().nullable(),
  })
  .passthrough();

export const sessionSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    application: z.string().optional().nullable(),
    createdTime: z.string().optional().nullable(),
    sessionId: z.array(z.string()).optional().nullable(),
  })
  .passthrough();

export const tokenSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    application: z.string().optional().nullable(),
    organization: z.string().optional().nullable(),
    user: z.string().optional().nullable(),
    code: z.string().optional().nullable(),
    accessToken: z.string().optional().nullable(),
    expiresIn: z.union([z.string(), z.number()]).optional().nullable(),
    scope: z.string().optional().nullable(),
    tokenType: z.string().optional().nullable(),
  })
  .passthrough();

export const recordSchema = z
  .object({
    id: z.union([z.string(), z.number()]).optional().nullable(),
    name: z.string().optional().nullable(),
    clientIp: z.string().optional().nullable(),
    createdTime: z.string().optional().nullable(),
    organization: z.string().optional().nullable(),
    user: z.string().optional().nullable(),
    method: z.string().optional().nullable(),
    requestUri: z.string().optional().nullable(),
    language: z.string().optional().nullable(),
    statusCode: z.union([z.string(), z.number()]).optional().nullable(),
    response: z.string().optional().nullable(),
    object: z.string().optional().nullable(),
    action: z.string().optional().nullable(),
    isTriggered: z.boolean().optional().nullable(),
  })
  .passthrough();

export const verificationSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    type: z.string().optional().nullable(),
    user: z.string().optional().nullable(),
    provider: z.string().optional().nullable(),
    remoteAddr: z.string().optional().nullable(),
    receiver: z.string().optional().nullable(),
    code: z.string().optional().nullable(),
    isUsed: z.boolean().optional().nullable(),
  })
  .passthrough();

export const productSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    image: z.string().optional().nullable(),
    tag: z.string().optional().nullable(),
    detail: z.string().optional().nullable(),
    description: z.string().optional().nullable(),
    currency: z.string().optional().nullable(),
    price: z.union([z.string(), z.number()]).optional().nullable(),
    isRecharge: z.boolean().optional().nullable(),
    disableCustomRecharge: z.boolean().optional().nullable(),
    rechargeOptions: z
      .array(z.union([z.string(), z.number()]))
      .optional()
      .nullable(),
    quantity: z.union([z.string(), z.number()]).optional().nullable(),
    sold: z.union([z.string(), z.number()]).optional().nullable(),
    successUrl: z.string().optional().nullable(),
    state: z.string().optional().nullable(),
    providers: z.array(z.string()).optional().nullable(),
  })
  .passthrough();

export const planSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    description: z.string().optional().nullable(),
    price: z.union([z.string(), z.number()]).optional().nullable(),
    currency: z.string().optional().nullable(),
    period: z.string().optional().nullable(),
    isEnabled: z.boolean().optional().nullable(),
    isExclusive: z.boolean().optional().nullable(),
    role: z.string().optional().nullable(),
    product: z.string().optional().nullable(),
    paymentProviders: z.array(z.string()).optional().nullable(),
    options: z.array(z.string()).optional().nullable(),
  })
  .passthrough();

export const pricingSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    application: z.string().optional().nullable(),
    plans: z.array(z.string()).optional().nullable(),
    isEnabled: z.boolean().optional().nullable(),
    trialDuration: z.union([z.string(), z.number()]).optional().nullable(),
  })
  .passthrough();

export const orderSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    updateTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    products: z.array(z.string()).optional().nullable(),
    user: z.string().optional().nullable(),
    payment: z.string().optional().nullable(),
    state: z.string().optional().nullable(),
    message: z.string().optional().nullable(),
    currency: z.string().optional().nullable(),
    price: z.union([z.string(), z.number()]).optional().nullable(),
    productInfos: z
      .array(
        z
          .object({
            owner: z.string().optional().nullable(),
            name: z.string().optional().nullable(),
            createdTime: z.string().optional().nullable(),
            displayName: z.string().optional().nullable(),
            image: z.string().optional().nullable(),
            detail: z.string().optional().nullable(),
            price: z.union([z.string(), z.number()]).optional().nullable(),
            currency: z.string().optional().nullable(),
            isRecharge: z.boolean().optional().nullable(),
            quantity: z.union([z.string(), z.number()]).optional().nullable(),
            pricingName: z.string().optional().nullable(),
            planName: z.string().optional().nullable(),
          })
          .passthrough(),
      )
      .optional()
      .nullable(),
  })
  .passthrough();

export const paymentSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    createdTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    provider: z.string().optional().nullable(),
    type: z.string().optional().nullable(),
    products: z.array(z.string()).optional().nullable(),
    productsDisplayName: z.string().optional().nullable(),
    productName: z.string().optional().nullable(),
    productDisplayName: z.string().optional().nullable(),
    detail: z.string().optional().nullable(),
    user: z.string().optional().nullable(),
    personName: z.string().optional().nullable(),
    personIdCard: z.string().optional().nullable(),
    personEmail: z.string().optional().nullable(),
    personPhone: z.string().optional().nullable(),
    invoiceType: z.string().optional().nullable(),
    invoiceTitle: z.string().optional().nullable(),
    invoiceTaxId: z.string().optional().nullable(),
    invoiceRemark: z.string().optional().nullable(),
    invoiceUrl: z.string().optional().nullable(),
    order: z.string().optional().nullable(),
    outOrderId: z.string().optional().nullable(),
    payUrl: z.string().optional().nullable(),
    successUrl: z.string().optional().nullable(),
    state: z.string().optional().nullable(),
    message: z.string().optional().nullable(),
    currency: z.string().optional().nullable(),
    price: z.union([z.string(), z.number()]).optional().nullable(),
    orderObj: z
      .object({
        owner: z.string().optional().nullable(),
        name: z.string().optional().nullable(),
        displayName: z.string().optional().nullable(),
        user: z.string().optional().nullable(),
        state: z.string().optional().nullable(),
        productInfos: z
          .array(
            z
              .object({
                name: z.string().optional().nullable(),
                displayName: z.string().optional().nullable(),
                price: z.union([z.string(), z.number()]).optional().nullable(),
                quantity: z.union([z.string(), z.number()]).optional().nullable(),
              })
              .passthrough(),
          )
          .optional()
          .nullable(),
      })
      .passthrough()
      .optional()
      .nullable(),
  })
  .passthrough();

export const transactionSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string().optional().nullable(),
    createdTime: z.string().optional().nullable(),
    displayName: z.string().optional().nullable(),
    application: z.string().optional().nullable(),
    domain: z.string().optional().nullable(),
    category: z.string().optional().nullable(),
    type: z.string().optional().nullable(),
    subtype: z.string().optional().nullable(),
    provider: z.string().optional().nullable(),
    user: z.string().optional().nullable(),
    tag: z.string().optional().nullable(),
    amount: z.union([z.string(), z.number()]).optional().nullable(),
    currency: z.string().optional().nullable(),
    payment: z.string().optional().nullable(),
    state: z.string().optional().nullable(),
  })
  .passthrough();

export const subscriptionSchema = z
  .object({
    owner: z.string().optional(),
    name: z.string(),
    displayName: z.string().optional().nullable(),
    createdTime: z.string().optional().nullable(),
    description: z.string().optional().nullable(),
    user: z.string().optional().nullable(),
    pricing: z.string().optional().nullable(),
    plan: z.string().optional().nullable(),
    payment: z.string().optional().nullable(),
    startTime: z.string().optional().nullable(),
    endTime: z.string().optional().nullable(),
    period: z.string().optional().nullable(),
    state: z.string().optional().nullable(),
  })
  .passthrough();

export const policySchema = z
  .object({
    Ptype: z.string().optional().nullable(),
    V0: z.string().optional().nullable(),
    V1: z.string().optional().nullable(),
    V2: z.string().optional().nullable(),
    V3: z.string().optional().nullable(),
    V4: z.string().optional().nullable(),
    V5: z.string().optional().nullable(),
  })
  .passthrough();

export const pagedUsersSchema = resourceListSchema(userSchema);
export const pagedApplicationsSchema = resourceListSchema(applicationSchema);
export const pagedProvidersSchema = resourceListSchema(providerSchema);
export const pagedOrganizationsSchema = resourceListSchema(organizationSchema);
export const pagedGroupsSchema = resourceListSchema(groupSchema);
export const pagedInvitationsSchema = resourceListSchema(invitationSchema);
export const pagedResourcesSchema = resourceListSchema(resourceSchema);
export const pagedCertsSchema = resourceListSchema(certSchema);
export const pagedServersSchema = resourceListSchema(serverSchema);
export const pagedRulesSchema = resourceListSchema(ruleSchema);
export const pagedSitesSchema = resourceListSchema(siteSchema);
export const pagedFormsSchema = resourceListSchema(formSchema);
export const pagedWebhooksSchema = resourceListSchema(webhookSchema);
export const pagedTicketsSchema = resourceListSchema(ticketSchema);
export const pagedLdapsSchema = resourceListSchema(ldapSchema);
export const pagedSyncersSchema = resourceListSchema(syncerSchema);
export const pagedRolesSchema = resourceListSchema(roleSchema);
export const pagedPermissionsSchema = resourceListSchema(permissionSchema);
export const pagedModelsSchema = resourceListSchema(modelSchema);
export const pagedAdaptersSchema = resourceListSchema(adapterSchema);
export const pagedEnforcersSchema = resourceListSchema(enforcerSchema);
export const pagedSessionsSchema = resourceListSchema(sessionSchema);
export const pagedTokensSchema = resourceListSchema(tokenSchema);
export const pagedRecordsSchema = resourceListSchema(recordSchema);
export const pagedVerificationsSchema = resourceListSchema(verificationSchema);
export const pagedProductsSchema = resourceListSchema(productSchema);
export const pagedPlansSchema = resourceListSchema(planSchema);
export const pagedPricingsSchema = resourceListSchema(pricingSchema);
export const pagedOrdersSchema = resourceListSchema(orderSchema);
export const pagedPaymentsSchema = resourceListSchema(paymentSchema);
export const pagedTransactionsSchema = resourceListSchema(transactionSchema);
export const pagedSubscriptionsSchema = resourceListSchema(subscriptionSchema);

export type SessionOrganization = z.infer<typeof sessionOrganizationSchema>;
export type SessionAccount = z.infer<typeof sessionAccountSchema>;
export type DashboardData = z.infer<typeof dashboardSchema>;
export type UserItem = z.infer<typeof userSchema>;
export type MfaPropsItem = z.infer<typeof mfaPropsSchema>;
export type MfaAccountItem = z.infer<typeof mfaAccountSchema>;
export type MfaItem = z.infer<typeof mfaItemSchema>;
export type UserDetailItem = z.infer<typeof userDetailSchema>;
export type ApplicationItem = z.infer<typeof applicationSchema>;
export type ProviderItem = z.infer<typeof providerSchema>;
export type OrganizationItem = z.infer<typeof organizationSchema>;
export type GroupItem = z.infer<typeof groupSchema>;
export type InvitationItem = z.infer<typeof invitationSchema>;
export type ResourceItem = z.infer<typeof resourceSchema>;
export type CertItem = z.infer<typeof certSchema>;
export type ServerItem = z.infer<typeof serverSchema>;
export type RuleExpressionItem = z.infer<typeof ruleExpressionSchema>;
export type RuleItem = z.infer<typeof ruleSchema>;
export type SiteNodeItem = z.infer<typeof siteNodeItemSchema>;
export type SiteItem = z.infer<typeof siteSchema>;
export type FormItemField = z.infer<typeof formItemSchema>;
export type FormItem = z.infer<typeof formSchema>;
export type WebhookHeaderItem = z.infer<typeof webhookHeaderSchema>;
export type WebhookItem = z.infer<typeof webhookSchema>;
export type TicketMessageItem = z.infer<typeof ticketMessageSchema>;
export type TicketItem = z.infer<typeof ticketSchema>;
export type LdapItem = z.infer<typeof ldapSchema>;
export type LdapUserItem = z.infer<typeof ldapUserSchema>;
export type LdapUsersData = z.infer<typeof ldapUsersDataSchema>;
export type LdapSyncResult = z.infer<typeof ldapSyncResultSchema>;
export type SyncerTableColumnItem = z.infer<typeof syncerTableColumnSchema>;
export type SyncerItem = z.infer<typeof syncerSchema>;
export type RoleItem = z.infer<typeof roleSchema>;
export type PermissionItem = z.infer<typeof permissionSchema>;
export type ModelItem = z.infer<typeof modelSchema>;
export type AdapterItem = z.infer<typeof adapterSchema>;
export type EnforcerItem = z.infer<typeof enforcerSchema>;
export type SessionItem = z.infer<typeof sessionSchema>;
export type TokenItem = z.infer<typeof tokenSchema>;
export type RecordItem = z.infer<typeof recordSchema>;
export type VerificationItem = z.infer<typeof verificationSchema>;
export type ProductItem = z.infer<typeof productSchema>;
export type PlanItem = z.infer<typeof planSchema>;
export type PricingItem = z.infer<typeof pricingSchema>;
export type OrderItem = z.infer<typeof orderSchema>;
export type PaymentItem = z.infer<typeof paymentSchema>;
export type TransactionItem = z.infer<typeof transactionSchema>;
export type SubscriptionItem = z.infer<typeof subscriptionSchema>;
export type PolicyItem = z.infer<typeof policySchema>;

// ─────────────────────────────────────────────
// GuliangIAM native types — re-exported from iam/* modules
// These use the GuliangIAM naming convention (organizationId, slug, etc.)
// ─────────────────────────────────────────────

export type {
  UserDetail as IamUserDetail,
  CreateUserPayload as IamCreateUserPayload,
  PatchUserPayload as IamPatchUserPayload,
  UserOption as IamUserOption,
} from "@/shared/api/iam/user";

export type {
  OrganizationDetail as IamOrganizationDetail,
  OrganizationOption as IamOrganizationOption,
  CreateOrganizationPayload as IamCreateOrganizationPayload,
  UpdateOrganizationPayload as IamUpdateOrganizationPayload,
} from "@/shared/api/iam/organization";

export type {
  ApplicationDetail as IamApplicationDetail,
  ApplicationOption as IamApplicationOption,
  CreateApplicationPayload as IamCreateApplicationPayload,
  UpdateApplicationPayload as IamUpdateApplicationPayload,
} from "@/shared/api/iam/application";

export type {
  ProviderDetail as IamProviderDetail,
  CreateProviderPayload as IamCreateProviderPayload,
  UpdateProviderPayload as IamUpdateProviderPayload,
} from "@/shared/api/iam/provider-extended";
