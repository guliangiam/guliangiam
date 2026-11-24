import {
  App,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  Modal,
  Popconfirm,
  Result,
  Space,
  Tag,
  Typography,
} from "antd";
import { lazy, Suspense, useCallback, useEffect, useMemo } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { DownloadOutlined } from "@ant-design/icons";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage, FORM_ITEM_FULL_WIDTH_CLASS } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { SectionNavShell } from "@/features/section-nav/SectionNavShell";
import type { SectionNavItem } from "@/features/section-nav/types";
import { serverErrorMessage } from "@/shared/lib/server-error";
import { BasicTab } from "./tabs/BasicTab";
import { AuthenticationTab } from "./tabs/AuthenticationTab";
import { OidcOAuthTab } from "./tabs/OidcOAuthTab";
import { APP_SECTION_KEYS, resolveAppSection, type AppSectionKey } from "./application-sections";
import { useApplicationDetailPageController } from "./useApplicationDetailPageController";

const SamlTab = lazy(() => import("./tabs/SamlTab").then((m) => ({ default: m.SamlTab })));
const ProvidersTab = lazy(() =>
  import("./tabs/ProvidersTab").then((m) => ({ default: m.ProvidersTab })),
);
const UiCustomizationTab = lazy(() =>
  import("./tabs/UiCustomizationTab").then((m) => ({ default: m.UiCustomizationTab })),
);
const SecurityTab = lazy(() =>
  import("./tabs/SecurityTab").then((m) => ({ default: m.SecurityTab })),
);
const ReverseProxyTab = lazy(() =>
  import("./tabs/ReverseProxyTab").then((m) => ({ default: m.ReverseProxyTab })),
);
const CustomScopesTab = lazy(() => import("./tabs/CustomScopesTab"));

export default function ApplicationDetailPage() {
  const { t: tr } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const {
    applicationId,
    isNew,
    sessionOrganizationId,
    configForm,
    setOpenDeleteConfirm,
    detailQuery,
    organizationsQuery,
    existingApplicationsQuery,
    detail,
    createApplicationMutation,
    applicationConfigMutation,
    deleteApplicationMutation,
    exportMutation,
    orgNameMap,
    providerOptions,
    existingApplicationSlugs,
    clientSecretExtra,
    clientSecretSet,
    existingClientCertificates,
    clientJwks,
    groups,
    clientSecretReveal,
    isRotatingClientSecret,
    boundProviders,
    deleteDescription,
    canConfirmDelete,
    handleSave,
    handleConfigFormChange,
    initializedEntityKey,
    resetUiConfigToSaved,
    resetUiConfigToDefaults,
    handleRotateClientSecret,
    dismissClientSecretReveal,
    formatTokenEndpointAuthMethod,
  } = useApplicationDetailPageController();

  // All hooks must run before the loading/error early returns below, otherwise
  // the hook order changes between the loading and loaded renders.
  const [searchParams, setSearchParams] = useSearchParams();
  // 8 个业务分区对创建态/编辑态都可见；差异只体现在分区内的卡片（登录日志、运维工具）。
  const availableSections = useMemo(() => new Set<AppSectionKey>(APP_SECTION_KEYS), []);
  const activeSection: AppSectionKey = useMemo(() => {
    const resolved = resolveAppSection(searchParams.get("tab"));
    return resolved && availableSections.has(resolved) ? resolved : "basics";
  }, [searchParams, availableSections]);

  const handleSectionChange = useCallback(
    (key: AppSectionKey) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          if (key === "basics") next.delete("tab");
          else next.set("tab", key);
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  // Legacy or unknown `?tab=` values are rewritten to the section key so the URL
  // always describes a section the page can render.
  useEffect(() => {
    const raw = searchParams.get("tab");
    if (!raw) return;
    const resolved = resolveAppSection(raw);
    if (resolved && resolved === raw && availableSections.has(resolved)) return;
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (!resolved || resolved === "basics") {
          next.delete("tab");
        } else {
          next.set("tab", resolved);
        }
        return next;
      },
      { replace: true },
    );
  }, [searchParams, availableSections, setSearchParams]);

  // 预览头部展示真实组织 slug（编辑态取详情组织，创建态取当前选择），
  // 避免把 organizationId 直接当成 slug 渲染。
  const watchedOrganizationId = Form.useWatch("organizationId", configForm) ?? "";
  const previewOrganizationSlug = useMemo(() => {
    const orgId = isNew ? watchedOrganizationId : (detail?.organizationId ?? "");
    if (!orgId) return "";
    const slug = (organizationsQuery.data ?? []).find((item) => item.id === orgId)?.slug;
    return slug || orgNameMap.get(orgId) || "";
  }, [isNew, watchedOrganizationId, detail?.organizationId, organizationsQuery.data, orgNameMap]);

  // 提供商登记清单只读派生自 providerItems：编辑区是唯一事实源，这里实时展示
  // 保存后将同步到 application_provider_bindings 的提供商集合。
  const watchedProviderItems = Form.useWatch("providerItems", configForm);
  const derivedProviderLabels = useMemo(() => {
    const labelByValue = new Map(providerOptions.map((option) => [option.value, option.label]));
    const seen = new Set<string>();
    const labels: string[] = [];
    for (const item of watchedProviderItems ?? []) {
      const value = (item.provider ?? "").trim();
      if (!value || seen.has(value)) continue;
      seen.add(value);
      labels.push(labelByValue.get(value) ?? value);
    }
    return labels;
  }, [watchedProviderItems, providerOptions]);

  const handleCopySecret = useCallback(() => {
    if (!clientSecretReveal) return;
    void navigator.clipboard?.writeText(clientSecretReveal.plain).then(
      () => message.success(tr("application.copied")),
      () => message.error(tr("application.copyFailed")),
    );
  }, [clientSecretReveal, message, tr]);

  if (!isNew && detailQuery.isError) {
    return (
      <Result
        status="error"
        title={tc("loadFailed")}
        subTitle={serverErrorMessage(detailQuery.error, tc, "common:loadFailed")}
        extra={<Button onClick={() => detailQuery.refetch()}>{tc("retry")}</Button>}
      />
    );
  }

  if (!isNew && !detailQuery.isLoading && !detail) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Space orientation="vertical" size="middle">
          <Typography.Title level={2}>{tr("application.appNotFoundTitle")}</Typography.Title>
          <Typography.Paragraph>{tr("application.appNotFoundDesc")}</Typography.Paragraph>
          <Button onClick={() => navigate("/applications")}>{tc("backToList")}</Button>
        </Space>
      </Card>
    );
  }

  const tabFallback = (
    <Card loading className="gl-panel" variant="borderless" style={{ minHeight: 200 }} />
  );

  // 分区导航：8 个业务分区各为一个锚点，全部同时挂载，靠 SectionNavShell
  // 的副菜单在右侧/左侧/顶部之间切换落位。`?tab=<section>` 深链保持可用。
  const sectionNavItems: SectionNavItem[] = [
    { key: "basics", anchor: "app-section-basics", label: tr("application.tabs.basic") },
    {
      key: "authentication",
      anchor: "app-section-authentication",
      label: tr("application.tabs.authentication"),
    },
    { key: "oidc", anchor: "app-section-oidc", label: tr("application.tabs.oidc") },
    { key: "saml", anchor: "app-section-saml", label: tr("application.tabs.saml") },
    {
      key: "providers",
      anchor: "app-section-providers",
      label: tr("application.tabs.providers"),
    },
    {
      key: "ui",
      anchor: "app-section-ui",
      label: tr("application.tabs.uiCustomization"),
    },
    { key: "security", anchor: "app-section-security", label: tr("application.tabs.security") },
    {
      key: "reverseProxy",
      anchor: "app-section-reverseProxy",
      label: tr("application.tabs.reverseProxy"),
    },
  ];

  const summaryCard =
    !isNew && detail ? (
      <FormSection title={tr("application.overviewSummary")}>
        <div className="gl-app-summary">
          <Descriptions size="small" column={{ xs: 1, sm: 2, lg: 3 }}>
            <Descriptions.Item label={tr("application.id")}>{detail.id}</Descriptions.Item>
            <Descriptions.Item label={tr("application.slug")}>{detail.slug}</Descriptions.Item>
            <Descriptions.Item label={tr("application.displayName")}>
              {detail.displayName || "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.organizationId")}>
              {orgNameMap.get(detail.organizationId) || detail.organizationId}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.category")}>
              {detail.category || "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.appType")}>
              {detail.appType || "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.status")}>
              {detail.status ? <Tag color="blue">{detail.status}</Tag> : "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.domain")}>
              {detail.domain || "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.homepageUrl")}>
              {detail.homepageUrl ? (
                <Typography.Link href={detail.homepageUrl} target="_blank" rel="noreferrer">
                  {detail.homepageUrl}
                </Typography.Link>
              ) : (
                "-"
              )}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.description")}>
              {detail.description || "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.clientId")}>
              {detail.clientId || "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.tokenEndpointAuthMethod")}>
              <Tag color={detail.tokenEndpointAuthMethod === "none" ? "default" : "geekblue"}>
                {formatTokenEndpointAuthMethod(detail.tokenEndpointAuthMethod, tr)}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.redirectUris")}>
              {Array.isArray(detail.redirectUris) && detail.redirectUris.length > 0 ? (
                <Space size={[8, 8]} wrap>
                  {detail.redirectUris.map((u) => (
                    <Tag key={u}>{u}</Tag>
                  ))}
                </Space>
              ) : (
                "-"
              )}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.grantTypes")}>
              {Array.isArray(detail.grantTypes) && detail.grantTypes.length > 0 ? (
                <Space size={[8, 8]} wrap>
                  {detail.grantTypes.map((g) => (
                    <Tag key={g}>{g}</Tag>
                  ))}
                </Space>
              ) : (
                "-"
              )}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.customScopes")}>
              {detail.customScopes && detail.customScopes.length > 0 ? (
                <Space size={[8, 8]} wrap>
                  {detail.customScopes.map((s) => (
                    <Tag key={s.scope}>{s.scope}</Tag>
                  ))}
                </Space>
              ) : (
                "-"
              )}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.providerBindingsDerivedTitle")}>
              {boundProviders.length > 0 ? (
                <Space size={[8, 8]} wrap>
                  {boundProviders.map((n) => (
                    <Tag key={n}>{n}</Tag>
                  ))}
                </Space>
              ) : (
                "-"
              )}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.tags")}>
              {detail.tags && detail.tags.length > 0 ? (
                <Space size={[8, 8]} wrap>
                  {detail.tags.map((t) => (
                    <Tag key={t}>{t}</Tag>
                  ))}
                </Space>
              ) : (
                "-"
              )}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.disableSignin")}>
              {detail.disableSignin ? (
                <Tag color="red">{tr("application.state.disabled")}</Tag>
              ) : (
                <Tag color="green">{tr("application.state.enabled")}</Tag>
              )}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.enableSigninSession")}>
              {detail.enableSigninSession ? (
                <Tag color="green">{tr("application.state.enabled")}</Tag>
              ) : (
                <Tag color="default">{tr("application.state.disabled")}</Tag>
              )}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.createdAt")}>
              {detail.createdAt}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.updatedAt")}>
              {detail.updatedAt}
            </Descriptions.Item>
            <Descriptions.Item label={tr("application.revision")}>
              {detail.revision}
            </Descriptions.Item>
          </Descriptions>
          <Space>
            <Link to="/providers">{tr("application.viewProviderList")}</Link>
          </Space>
        </div>
      </FormSection>
    ) : null;

  return (
    <div className="gl-resource-page">
      <ResourceFormPage
        title={
          isNew
            ? tr("application.newApplication")
            : detail?.displayName || detail?.slug || applicationId
        }
        description={isNew ? tr("application.newCreateDesc") : tr("application.editConfigDesc")}
        loading={!isNew && detailQuery.isLoading}
        saving={createApplicationMutation.isPending || applicationConfigMutation.isPending}
        form={configForm}
        fieldLabels={{ organizationId: tr("application.organizationId") }}
        onFinish={(values) => handleSave(values)}
        onValuesChange={() => handleConfigFormChange()}
        onCancel={() => navigate("/applications")}
        submitText={isNew ? tr("application.createApplication") : tr("application.saveConfig")}
        extra={
          !isNew ? (
            <PermissionGuard action="app:delete" resource="applications" fallback={null}>
              <Popconfirm
                title={tr("application.deleteAppTitle")}
                description={deleteDescription}
                okText={tc("delete")}
                cancelText={tc("cancel")}
                okButtonProps={{
                  danger: true,
                  loading: deleteApplicationMutation.isPending,
                  disabled: !canConfirmDelete,
                }}
                onOpenChange={setOpenDeleteConfirm}
                onConfirm={() => {
                  deleteApplicationMutation.mutate();
                }}
              >
                <Button danger disabled={!detail}>
                  {tc("delete")}
                </Button>
              </Popconfirm>
            </PermissionGuard>
          ) : null
        }
        enableDraft
        draftRoute="applications"
        draftEntityId={isNew ? "new" : applicationId}
        draftIgnoreKeys={["revision", "clientSecret", "jwks", "mtlsCertificates"]}
      >
        <div className="gl-app-summary-slot">
          {summaryCard}
          <SectionNavShell
            items={sectionNavItems}
            initialActiveKey={activeSection}
            onNavigate={(key) => handleSectionChange(key as AppSectionKey)}
          >
            <div className="gl-section-anchors">
              <div id="app-section-basics" className="gl-section-anchor">
                <div id="section-organization" className="gl-app-ia__section">
                  {isNew ? (
                    <FormSection title={tr("application.organizationSection")}>
                      <Form.Item
                        name="organizationId"
                        className={FORM_ITEM_FULL_WIDTH_CLASS}
                        rules={[
                          {
                            required: true,
                            whitespace: true,
                            message: tr("application.validation.orgIdRequired"),
                          },
                        ]}
                      >
                        <GlFormRow label={tr("application.organizationId")} required>
                          <OrganizationSelect
                            placeholder={tr("application.placeholder.selectOrganization")}
                          />
                        </GlFormRow>
                      </Form.Item>
                    </FormSection>
                  ) : (
                    <FormSection title={tr("application.organizationSection")}>
                      {/* GlFormRow 内部用 Form.Item.useStatus()，必须位于 Form.Item 之内 */}
                      <Form.Item noStyle>
                        <GlFormRow label={tr("application.organizationId")}>
                          <Typography.Text strong>
                            {orgNameMap.get(detail?.organizationId ?? "") || detail?.organizationId}
                          </Typography.Text>
                        </GlFormRow>
                      </Form.Item>
                    </FormSection>
                  )}
                </div>
                <div id="section-basic" className="gl-app-ia__section">
                  <BasicTab
                    form={configForm}
                    isNew={isNew}
                    existingApplicationSlugs={existingApplicationSlugs}
                    isSlugValidationLoading={existingApplicationsQuery.isLoading}
                    owner={detail?.organizationId ?? sessionOrganizationId}
                    appName={detail?.slug ?? configForm.getFieldValue("slug") ?? ""}
                    organizationSlug={previewOrganizationSlug}
                  />
                </div>
                {!isNew ? (
                  <div id="section-tools" className="gl-app-ia__section">
                    <FormSection title={tr("application.toolsSection")}>
                      <Typography.Paragraph type="secondary">
                        {tr("application.toolsDesc")}
                      </Typography.Paragraph>
                      <Space wrap>
                        <Button
                          icon={<DownloadOutlined />}
                          loading={exportMutation.isPending}
                          onClick={() => exportMutation.mutate()}
                        >
                          {tr("application.export")}
                        </Button>
                      </Space>
                    </FormSection>
                  </div>
                ) : null}
              </div>

              <div id="app-section-authentication" className="gl-section-anchor">
                <div id="section-authentication" className="gl-app-ia__section">
                  <AuthenticationTab form={configForm} groups={groups} />
                </div>
              </div>

              <div id="app-section-oidc" className="gl-section-anchor">
                <div id="section-oidc" className="gl-app-ia__section">
                  <OidcOAuthTab
                    form={configForm}
                    isNew={isNew}
                    clientSecretExtra={clientSecretExtra}
                    clientSecretSet={clientSecretSet}
                    onRotateSecret={handleRotateClientSecret}
                    isRotatingSecret={isRotatingClientSecret}
                    applicationName={detail?.displayName || detail?.slug || applicationId}
                    existingClientCertificates={existingClientCertificates}
                    existingJwksKids={clientJwks.kids}
                    persistedAuthMethod={detail?.tokenEndpointAuthMethod}
                    persistedGrantTypes={detail?.grantTypes}
                  />
                </div>
                <div id="section-custom-scopes" className="gl-app-ia__section">
                  <FormSection title={tr("application.customScopesSectionTitle")}>
                    <Suspense fallback={tabFallback}>
                      <CustomScopesTab form={configForm} />
                    </Suspense>
                  </FormSection>
                </div>
              </div>

              <div id="app-section-saml" className="gl-section-anchor">
                <div id="section-saml" className="gl-app-ia__section">
                  <Suspense fallback={tabFallback}>
                    <SamlTab form={configForm} />
                  </Suspense>
                </div>
              </div>

              <div id="app-section-providers" className="gl-section-anchor">
                <div id="section-provider-items" className="gl-app-ia__section">
                  <FormSection title={tr("application.providerItemsSection")}>
                    <Typography.Paragraph type="secondary">
                      {tr("application.providerItemsDesc")}
                    </Typography.Paragraph>
                    <Suspense fallback={tabFallback}>
                      <ProvidersTab
                        form={configForm}
                        providerOptions={providerOptions}
                        organizationId={
                          isNew ? watchedOrganizationId : (detail?.organizationId ?? "")
                        }
                      />
                    </Suspense>
                  </FormSection>
                </div>
                {!isNew ? (
                  <div id="section-provider-bindings" className="gl-app-ia__section">
                    <FormSection title={tr("application.providerBindingsDerivedTitle")}>
                      <Typography.Paragraph type="secondary">
                        {tr("application.providerBindingsDerivedDesc")}
                      </Typography.Paragraph>
                      <Form.Item noStyle>
                        <GlFormRow label={tr("application.boundProviders")}>
                          {derivedProviderLabels.length > 0 ? (
                            <Space size={[8, 8]} wrap>
                              {derivedProviderLabels.map((label) => (
                                <Tag key={label}>{label}</Tag>
                              ))}
                            </Space>
                          ) : (
                            "-"
                          )}
                        </GlFormRow>
                      </Form.Item>
                    </FormSection>
                  </div>
                ) : null}
              </div>

              <div id="app-section-ui" className="gl-section-anchor">
                <div id="section-ui" className="gl-app-ia__section">
                  <Suspense fallback={tabFallback}>
                    <UiCustomizationTab
                      form={configForm}
                      organizationSlug={previewOrganizationSlug}
                      entityKey={initializedEntityKey}
                      onResetToSaved={resetUiConfigToSaved}
                      onResetToDefaults={resetUiConfigToDefaults}
                      onProgrammaticChange={handleConfigFormChange}
                    />
                  </Suspense>
                </div>
              </div>

              <div id="app-section-security" className="gl-section-anchor">
                <div id="section-security" className="gl-app-ia__section">
                  <Suspense fallback={tabFallback}>
                    <SecurityTab form={configForm} />
                  </Suspense>
                </div>
                {!isNew ? (
                  <div id="section-signin-log" className="gl-app-ia__section">
                    <FormSection title={tr("application.tabs.signinLog")}>
                      <Typography.Paragraph type="secondary">
                        {tr("application.signinLogMovedHint")}
                      </Typography.Paragraph>
                      <Link
                        to={`/audit/login-logs?applicationId=${encodeURIComponent(applicationId)}`}
                      >
                        {tr("application.signinLogOpen")}
                      </Link>
                    </FormSection>
                  </div>
                ) : null}
              </div>

              <div id="app-section-reverseProxy" className="gl-section-anchor">
                <div id="section-reverse-proxy" className="gl-app-ia__section">
                  <Suspense fallback={tabFallback}>
                    <ReverseProxyTab form={configForm} />
                  </Suspense>
                </div>
              </div>
            </div>
          </SectionNavShell>
        </div>
      </ResourceFormPage>

      {/* One-time plaintext. The value only exists in the rotation/creation
          response; closing this dialog discards it for good. */}
      <Modal
        open={clientSecretReveal !== null}
        title={tr("application.oneTimeSecretTitle")}
        closable={false}
        mask={{ closable: false }}
        keyboard={false}
        okText={tr("application.oneTimeSecretConfirm")}
        cancelButtonProps={{ style: { display: "none" } }}
        onOk={dismissClientSecretReveal}
      >
        {clientSecretReveal?.prefix ? (
          <Typography.Paragraph type="secondary">
            {tr("application.oneTimeSecretPrefix")}: {clientSecretReveal.prefix}
          </Typography.Paragraph>
        ) : null}
        <Space.Compact style={{ width: "100%" }}>
          <Input readOnly value={clientSecretReveal?.plain ?? ""} />
          <Button onClick={handleCopySecret}>{tr("application.copySecret")}</Button>
        </Space.Compact>
        <Typography.Paragraph type="warning" style={{ marginTop: 12 }}>
          {tr("application.oneTimeSecretWarning")}
        </Typography.Paragraph>
      </Modal>
    </div>
  );
}
