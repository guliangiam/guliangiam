import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Form, Input, Popconfirm, Select, Skeleton, Switch } from "antd";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { ResourceFormPage, FORM_ITEM_FULL_WIDTH_CLASS } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { NotFoundResult, QueryErrorResult } from "@/shared/ui/QueryErrorResult";
import { DynamicProviderFields } from "@/shared/ui/ProviderFields";
import { HttpHeaderTable } from "@/shared/ui/HttpHeaderTable";
import { FormSection } from "@/shared/ui/FormSection";
import { TestConnectionButton } from "@/shared/ui/TestConnectionButton";
import { CaptchaTestModal } from "./CaptchaTestModal";
import { UserMappingFields } from "@/shared/ui/UserMappingFields";
import { ProviderCategoryBadge } from "@/shared/ui/ProviderCategoryBadge";
import { ProviderIcon } from "@/shared/ui/ProviderIcon";
import { SectionNavShell } from "@/features/section-nav/SectionNavShell";
import type { SectionNavItem } from "@/features/section-nav/types";
import {
  isAuthSwitchApplicable,
  isManagedProviderKind,
  PROVIDER_CONSOLE_REDIRECTS,
  useProviderCategories,
  type ProviderAuthSwitchName,
} from "@/shared/hooks/useProviderCategories";
import {
  httpHeadersToRows,
  useProviderDetail,
  type ProviderFormValues,
} from "@/shared/hooks/useProviderDetail";

/**
 * Auth-switch values that must survive a save even when their control is
 * hidden for the current category/type. Hidden fields have no Form.Item, so a
 * missing store value is back-filled from the loaded detail instead of being
 * silently reset by the PATCH payload.
 */
const AUTH_SWITCH_PRESERVED_FIELDS = [
  "enablePkce",
  "enableProxy",
  "httpHeaders",
  "emailRegex",
  "customLogoutUrl",
  "customLogo",
  "state",
  "requireMessageAuthenticator",
  "enableSignAuthnRequest",
  "cert",
] as const;

/**
 * 跨类别共享的配置列。切换 category 时这些列对旧类别可能仍有残值，而新类别
 * 不再渲染对应控件（无 Form.Item），残值会随 getFieldsValue(true) 进入 PATCH，
 * 把上一类别的配置错误地写入新类别。切换时统一清空，实现 category 白名单裁剪。
 */
const CATEGORY_CONFIG_FIELDS = [
  "subType",
  "method",
  "clientId",
  "clientSecret",
  "clientId2",
  "clientSecret2",
  "authUrl",
  "tokenUrl",
  "userInfoUrl",
  "callbackUrl",
  "signingAlg",
  "idpEntityId",
  "ssoUrl",
  "sloUrl",
  "metadata",
  "issuerUrl",
  "region",
  "host",
  "databaseType",
  "template",
  "customApi",
  "customHeader",
  "customBody",
  "admin",
  "password",
  "appId",
  "agentId",
  "contentType",
  "sender",
  "receiver",
  "providerUrl",
  "userMapping",
  "idvSceneId",
] as const;

export default function ProviderDetailPage() {
  const { providerId = "new" } = useParams();
  const isNewProvider = providerId === "new";

  // 新建同样是写操作：只读权限不能创建，否则会出现「列表看不到入口、URL 直连可写」的绕过。
  return (
    <PermissionGuard
      action={isNewProvider ? "provider:write" : "provider:read"}
      resource="providers"
    >
      <ProviderDetailContent />
    </PermissionGuard>
  );
}

function ProviderDetailContent() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const { providerId = "new" } = useParams();
  const [searchParams] = useSearchParams();
  const [form] = Form.useForm<ProviderFormValues>();
  const [captchaTestOpen, setCaptchaTestOpen] = useState(false);

  const watchedCategory = Form.useWatch("category", form) ?? "OAuth";
  const watchedProviderType = Form.useWatch("providerType", form);

  const { categoryOptions, providerTypeOptions, credentialFields, showUserMapping } =
    useProviderCategories({ category: watchedCategory });

  const { isNew, detailQuery, orgOptionsQuery, saveMutation, enableMutation, deleteMutation } =
    useProviderDetail({
      providerId,
      form,
    });

  // `/providers/new?category=Captcha` 预置类别（验证码专用控制台的「新建」入口
  // 统一到这里）。声明在 useProviderDetail 之后，effect 同批后置执行，不会被
  // hook 的默认值填充覆盖。
  useEffect(() => {
    if (!isNew) return;
    const preset = searchParams.get("category");
    if (!preset) return;
    const match = categoryOptions.find((option) => option.value === preset);
    if (!match) return;
    // 专用控制台类别（FaceId）：预置即跳转独立创建页，不在统一表单落地。
    const redirect = PROVIDER_CONSOLE_REDIRECTS[match.value];
    if (redirect) {
      navigate(redirect);
      return;
    }
    form.setFieldsValue({ category: match.value, providerType: undefined });
  }, [isNew, searchParams, categoryOptions, form, navigate]);

  // 分区清单固定为 5 项，与详情页的分区定义一一对应。凭据 / 用户映射随
  // category 条件渲染，不适用时导航项保留但禁用，避免清单随类型抖动。
  const showCredentials = credentialFields.clientId || credentialFields.clientSecret;
  // 只在可见性变化时重建：SectionNavShell 用该清单建立 IntersectionObserver，
  // 每次 re-render 换新数组会让观察器反复重建。
  const sectionNavItems: SectionNavItem[] = useMemo(
    () => [
      { key: "basic", anchor: "provider-section-basic", label: tr("provider.basicInfo") },
      {
        key: "credentials",
        anchor: "provider-section-credentials",
        label: tr("provider.credentials"),
        disabled: !showCredentials,
      },
      { key: "dynamic", anchor: "provider-section-dynamic", label: tr("provider.dynamicFields") },
      {
        key: "user-mapping",
        anchor: "provider-section-user-mapping",
        label: tr("provider.userMapping"),
        disabled: !showUserMapping,
      },
      {
        key: "auth-switches",
        anchor: "provider-section-auth-switches",
        label: tr("provider.authSwitches"),
      },
    ],
    [tr, showCredentials, showUserMapping],
  );

  if (!isNew && detailQuery.isLoading) {
    return (
      <div style={{ padding: 48, textAlign: "center" }}>
        <Skeleton active paragraph={{ rows: 8 }} />
      </div>
    );
  }

  if (!isNew && detailQuery.isError) {
    return (
      <QueryErrorResult
        error={detailQuery.error}
        onRetry={() => detailQuery.refetch()}
        notFoundTitle={tr("provider.notFound")}
        notFoundDescription={tr("provider.notFoundDesc", { id: providerId })}
      />
    );
  }

  const data = detailQuery.data;

  if (!isNew && !data) {
    return (
      <NotFoundResult
        title={tr("provider.notFound")}
        description={tr("provider.notFoundDesc", { id: providerId })}
      />
    );
  }

  const effectiveCategory = String(watchedCategory ?? data?.category ?? "");
  const effectiveProviderType = String(watchedProviderType ?? data?.providerType ?? "");
  const showAuthSwitch = (field: ProviderAuthSwitchName) =>
    isAuthSwitchApplicable(field, effectiveCategory, effectiveProviderType);

  const isSMSProvider = effectiveCategory.toUpperCase() === "SMS";
  const smsProviderType = effectiveProviderType;
  const isAlibabaSMSProvider = smsProviderType.toLowerCase() === "alibabasms";

  // Captcha delegated kinds 调 /providers/{id}/test-connection 返回 422，
  // 因此统一详情页需要检测这些 kind 并替换为专门的验证码测试按钮。
  const isCaptchaProvider =
    effectiveCategory === "Captcha" || isManagedProviderKind(effectiveProviderType);

  // 历史统一 FaceId 行（kind=faceid/aliyun_face_body）由专用 face_id_providers
  // 子系统托管：统一端点拒绝 kind/providerType 变更与密钥轮换。编辑态锁定类型与
  // 类别，避免选中下拉里仅剩的 Custom 触发 managed-kind 422（表现为「类型 自定义」）。
  const isFaceIdProvider = effectiveCategory === "FaceId";
  const isNotificationProvider = effectiveCategory === "Notification";
  const lockManagedCategory = !isNew && (effectiveCategory === "Captcha" || isFaceIdProvider);

  // 动态区凭据（clientSecret/clientSecret2/password/appSecret）由受控组件渲染、
  // 拿不到详情，已保存指示统一在这里回显：否则用户看不到密钥已配置，会误以为
  // 留空即未设置。凭据分区自己渲染 clientSecret 提示时不再重复列入。
  const savedSecretEntries =
    !isNew && data
      ? [
          !credentialFields.clientSecret && data.clientSecretSet
            ? { label: t("clientSecret"), masked: data.clientSecretMasked }
            : null,
          data.clientSecret2Set
            ? { label: t("clientSecret2"), masked: data.clientSecret2Masked }
            : null,
          data.passwordSet ? { label: t("password"), masked: data.passwordMasked } : null,
          data.appSecretSet ? { label: t("appSecret"), masked: data.appSecretMasked } : null,
        ].filter((entry) => entry !== null)
      : [];

  return (
    <>
      <ResourceFormPage
        title={isNew ? tr("provider.newProvider") : (data?.displayName ?? providerId)}
        description={tr("provider.providerDescription")}
        loading={detailQuery.isLoading || orgOptionsQuery.isLoading}
        saving={saveMutation.isPending}
        form={form}
        enableDraft
        draftRoute="providers"
        draftEntityId={isNew ? "new" : providerId}
        fieldAliases={{ kind: "providerType" }}
        fieldLabels={{ kind: t("providerType") }}
        onCancel={() => navigate("/providers")}
        onFinish={async (values) => {
          // 动态 Provider 字段（Client ID/Secret/Scope/回调等）没有对应的 Form.Item，
          // 由 ProviderFieldBridge 用 setFieldValue 写入 store；validateFields() 只返回
          // 已注册字段，因此这里合并整份 store，否则这些值不会进入提交 payload。
          const all = form.getFieldsValue(true) as Partial<ProviderFormValues>;
          // 隐藏的认证开关没有 Form.Item，若 store 中缺值则回填详情里的既有值，
          // 避免 PATCH 省略该键或把它重置成默认值。
          if (data) {
            for (const field of AUTH_SWITCH_PRESERVED_FIELDS) {
              if (all[field] !== undefined) continue;
              if (field === "httpHeaders") {
                all.httpHeaders = httpHeadersToRows(data.httpHeaders);
              } else {
                (all as Record<string, unknown>)[field] = data[field];
              }
            }
          }
          const category = String(all.category ?? values.category ?? "");
          if (["OAuth", "OIDC"].includes(category) && !String(all.clientId ?? "").trim()) {
            // Client ID 在动态区输入，不属于 Form.Item，AntD 规则覆盖不到，这里补一道。
            throw new Error(`${t("clientId")}: ${t("required")}`);
          }
          // 返回 Promise：ResourceFormPage 会保持提交锁直到请求结束，失败保留输入。
          await saveMutation.mutateAsync({ ...all, ...values } as ProviderFormValues);
        }}
        extra={
          <>
            {!isNew && data ? <ProviderCategoryBadge category={data.category} /> : null}
            {!isNew && data && data.status !== "active" ? (
              <PermissionGuard action="provider:write" resource="providers" fallback={null}>
                <Popconfirm
                  title={tr("provider.enableConfirm")}
                  okText={t("enable")}
                  cancelText={t("action.cancel")}
                  okButtonProps={{ loading: enableMutation.isPending }}
                  onConfirm={() => {
                    enableMutation.mutate();
                  }}
                >
                  <Button type="primary" loading={enableMutation.isPending}>
                    {t("enable")}
                  </Button>
                </Popconfirm>
              </PermissionGuard>
            ) : null}
            {!isNew && isCaptchaProvider ? (
              <PermissionGuard action="provider:read" resource="providers" fallback={null}>
                <Button
                  onClick={() => setCaptchaTestOpen(true)}
                  title={tr("provider.captchaTestNote")}
                >
                  {t("captchaTest")}
                </Button>
              </PermissionGuard>
            ) : null}
            {!isNew && !isCaptchaProvider ? (
              <TestConnectionButton
                providerId={providerId}
                organizationId={detailQuery.data?.organizationId}
              />
            ) : null}
            {!isNew ? (
              <PermissionGuard action="provider:write" resource="providers" fallback={null}>
                <Popconfirm
                  title={t("deleteConfirm")}
                  description={providerId}
                  okText={t("delete")}
                  cancelText={t("action.cancel")}
                  okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
                  onConfirm={() => {
                    deleteMutation.mutate();
                  }}
                >
                  <Button danger>{t("delete")}</Button>
                </Popconfirm>
              </PermissionGuard>
            ) : null}
          </>
        }
      >
        {!isNew && isFaceIdProvider && (
          <Alert
            showIcon
            style={{ marginBottom: 16 }}
            type="info"
            title={tr(
              "provider.faceIdManagedNote",
              "This legacy Face ID provider is managed by the dedicated Face ID console. Type and category are locked here; configure AccessKey credentials on the Face ID Providers page.",
            )}
          />
        )}
        {isSMSProvider && (
          <Alert
            showIcon
            style={{ marginBottom: 16 }}
            type={isAlibabaSMSProvider ? "info" : "warning"}
            title={
              isAlibabaSMSProvider
                ? tr(
                    "provider.alibabaSmsProbeNote",
                    "Alibaba Cloud SMS test connection runs a read-only authentication probe (GetSmsSign/GetSmsTemplate). No SMS is sent and no fee is charged; actual SMS delivery is NOT verified.",
                  )
                : tr(
                    "provider.smsStructuralOnlyNote",
                    "This SMS provider has no real connection probe. Test connection performs structural validation only; connectivity is NOT verified.",
                  )
            }
          />
        )}

        {isNotificationProvider && (
          <Alert
            showIcon
            style={{ marginBottom: 16 }}
            type="info"
            title={tr(
              "provider.notificationPushNote",
              "Notification providers in this catalog back Push MFA: users bind them in security settings. Outbound notification channels (webhooks, bots) are managed in the dedicated notification console.",
            )}
            description={
              <Link to="/notification-providers">
                {tr("provider.notificationConsoleLink", "Open notification console")}
              </Link>
            }
          />
        )}

        <div style={{ gridColumn: "1 / -1", minWidth: 0 }}>
          <SectionNavShell items={sectionNavItems}>
            <div className="gl-section-anchors">
              <div id="provider-section-basic" className="gl-section-anchor">
                <FormSection title={tr("provider.basicInfo")} columns={2}>
                  <Form.Item
                    label={t("organization")}
                    name="organizationId"
                    rules={[{ required: true, message: t("required") }]}
                  >
                    <Select
                      disabled={!isNew}
                      options={orgOptionsQuery.data?.map((item) => ({
                        label: item.displayName || item.slug,
                        value: item.id,
                      }))}
                    />
                  </Form.Item>
                  <Form.Item
                    label={t("name")}
                    name="name"
                    rules={[{ required: true, message: t("required") }]}
                  >
                    <Input disabled={!isNew} />
                  </Form.Item>
                  {/* 专用 captcha_providers 表没有 displayName 列：Captcha 类别
                      不渲染无落库列的字段（统一表单创建时分流到专用端点）。 */}
                  {effectiveCategory !== "Captcha" && (
                    <Form.Item
                      label={t("displayName")}
                      name="displayName"
                      rules={[{ required: true, message: t("required") }]}
                    >
                      <Input />
                    </Form.Item>
                  )}
                  <Form.Item
                    label={t("category")}
                    name="category"
                    rules={[{ required: true, message: t("required") }]}
                  >
                    <Select
                      options={categoryOptions}
                      disabled={lockManagedCategory}
                      onChange={(value) => {
                        // 专用控制台类别（FaceId）：选中即跳转独立创建页，统一端点
                        // 拒绝 managed kind，不在统一表单提交。
                        const redirect = PROVIDER_CONSOLE_REDIRECTS[value];
                        if (isNew && redirect) {
                          navigate(redirect);
                          return;
                        }
                        // 切类别即重置类型与全部跨类别配置列，避免旧类别残值随
                        // 整份 store 提交（FE-B-06）。名称/组织等身份字段保留。
                        const reset: Record<string, unknown> = { providerType: undefined };
                        for (const field of CATEGORY_CONFIG_FIELDS) reset[field] = "";
                        reset.scopes = [];
                        reset.port = 0;
                        form.setFieldsValue(reset as Partial<ProviderFormValues>);
                      }}
                    />
                  </Form.Item>
                  <Form.Item
                    label={t("type")}
                    name="providerType"
                    rules={[{ required: true, message: t("required") }]}
                  >
                    {/* 遗留统一 Captcha / FaceId 行：统一端点禁止变更 managed kind
                        （P1 422），编辑态锁定类型。 */}
                    <Select
                      showSearch
                      optionFilterProp="value"
                      disabled={lockManagedCategory}
                      options={providerTypeOptions.map((option) => ({
                        value: option.value,
                        label: (
                          <span className="gl-provider-option">
                            <ProviderIcon type={option.value} size={20} />
                            <span>{option.label}</span>
                          </span>
                        ),
                      }))}
                    />
                  </Form.Item>
                </FormSection>
              </div>

              {showCredentials && (
                <div id="provider-section-credentials" className="gl-section-anchor">
                  <FormSection title={tr("provider.credentials")} columns={2}>
                    {credentialFields.clientId && (
                      <Form.Item label={t("clientId")} name="clientId">
                        <Input placeholder={t("clientIdOrSiteKey")} />
                      </Form.Item>
                    )}
                    {credentialFields.clientSecret && (
                      <Form.Item
                        label={t("clientSecret")}
                        name="clientSecret"
                        extra={
                          !isNew && data?.clientSecretSet
                            ? tr("provider.clientSecretMaskedHint", {
                                masked: data.clientSecretMasked || "******",
                              })
                            : undefined
                        }
                      >
                        <Input.Password placeholder={isNew ? "" : t("leaveEmptyToKeepCurrent")} />
                      </Form.Item>
                    )}
                  </FormSection>
                </div>
              )}

              <div id="provider-section-dynamic" className="gl-section-anchor">
                <FormSection title={tr("provider.dynamicFields")}>
                  {savedSecretEntries.length > 0 && (
                    <Alert
                      showIcon
                      type="info"
                      style={{ marginBottom: 16 }}
                      title={tr("provider.savedCredentialsTitle")}
                      description={savedSecretEntries
                        .map((entry) => `${entry.label}: ${entry.masked || "******"}`)
                        .join(" · ")}
                    />
                  )}
                  <DynamicProviderFields category={watchedCategory} form={form} />
                </FormSection>
              </div>

              {showUserMapping && (
                <div id="provider-section-user-mapping" className="gl-section-anchor">
                  <FormSection title={tr("provider.userMapping")} columns={2}>
                    <Form.Item name="userMapping" className={FORM_ITEM_FULL_WIDTH_CLASS}>
                      <UserMappingFields />
                    </Form.Item>
                  </FormSection>
                </div>
              )}

              <div id="provider-section-auth-switches" className="gl-section-anchor">
                <FormSection title={tr("provider.authSwitches")} columns={2}>
                  {/* ProviderDetail/PatchProviderRequest has no enableSignUp column: the
            switch never persisted, so it was removed instead of faking a setting.
            Re-adding it requires a schema column + DTO field first. */}
                  {showAuthSwitch("enablePkce") && (
                    <Form.Item
                      label={tr("provider.enablePkce")}
                      name="enablePkce"
                      valuePropName="checked"
                      extra={tr("provider.enablePkceExtra")}
                    >
                      <Switch />
                    </Form.Item>
                  )}
                  {showAuthSwitch("emailRegex") && (
                    <Form.Item
                      label={tr("provider.emailRegex")}
                      name="emailRegex"
                      extra={tr("provider.emailRegexExtra")}
                    >
                      <Input placeholder=".*@example\\.com$" />
                    </Form.Item>
                  )}
                  {showAuthSwitch("enableProxy") && (
                    <Form.Item
                      label={tr("provider.enableProxy")}
                      name="enableProxy"
                      valuePropName="checked"
                      extra={tr("provider.enableProxyExtra")}
                    >
                      <Switch />
                    </Form.Item>
                  )}
                  {showAuthSwitch("requireMessageAuthenticator") && (
                    <Form.Item
                      label={tr("provider.requireMessageAuthenticator")}
                      name="requireMessageAuthenticator"
                      valuePropName="checked"
                      extra={tr("provider.requireMessageAuthenticatorExtra")}
                    >
                      <Switch />
                    </Form.Item>
                  )}
                  {showAuthSwitch("enableSignAuthnRequest") && (
                    <Form.Item
                      label={tr("provider.enableSignAuthnRequest")}
                      name="enableSignAuthnRequest"
                      valuePropName="checked"
                      extra={tr("provider.enableSignAuthnRequestExtra")}
                    >
                      <Switch />
                    </Form.Item>
                  )}
                  {showAuthSwitch("customLogoutUrl") && (
                    <Form.Item label={tr("provider.customLogoutUrl")} name="customLogoutUrl">
                      <Input placeholder="https://idp.example.com/logout" />
                    </Form.Item>
                  )}
                  {showAuthSwitch("customLogo") && (
                    <Form.Item label={tr("provider.customLogo")} name="customLogo">
                      <Input placeholder="https://cdn.example.com/provider-logo.svg" />
                    </Form.Item>
                  )}
                  {showAuthSwitch("cert") && (
                    <Form.Item
                      label={tr("provider.cert")}
                      name="cert"
                      extra={tr("provider.certExtra")}
                    >
                      <Input.TextArea
                        rows={3}
                        placeholder={tr("provider.certificatePlaceholder")}
                      />
                    </Form.Item>
                  )}
                  {showAuthSwitch("state") && (
                    <Form.Item label={tr("provider.state")} name="state">
                      <Input placeholder={tr("provider.statePlaceholder")} />
                    </Form.Item>
                  )}
                  {showAuthSwitch("httpHeaders") && (
                    <Form.Item
                      label={tr("provider.httpHeaders")}
                      name="httpHeaders"
                      extra={tr("provider.httpHeadersExtra")}
                    >
                      <HttpHeaderTable />
                    </Form.Item>
                  )}
                </FormSection>
              </div>
            </div>
          </SectionNavShell>
        </div>
      </ResourceFormPage>
      <CaptchaTestModal
        providerId={providerId}
        providerType={effectiveProviderType}
        organizationId={detailQuery.data?.organizationId}
        open={captchaTestOpen}
        onClose={() => setCaptchaTestOpen(false)}
      />
    </>
  );
}
