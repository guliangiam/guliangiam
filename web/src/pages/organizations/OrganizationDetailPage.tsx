/**
 * Organization 详情页 · View（纯渲染）
 *
 * 信息架构：左侧 5 个分区锚点，右侧单列表单；所有分区共享同一个
 * `<Form>` 与全局保存事务，避免分区保存把其他子资源按空值覆盖。
 */
import { useMemo } from "react";
import { Alert, Button, Card, Popconfirm, Result } from "antd";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { SectionNavShell } from "@/features/section-nav/SectionNavShell";
import type { SectionNavItem } from "@/features/section-nav/types";
import { serverErrorMessage } from "@/shared/lib/server-error";
import { useOrganizationDetailPageController } from "./useOrganizationDetailPageController";
import { BasicInfoTab } from "./tabs/BasicInfoTab";
import { BrandingTab } from "./tabs/BrandingTab";
import { PasswordPolicyTab } from "./tabs/PasswordPolicyTab";
import { SecurityPolicyTab } from "./tabs/SecurityPolicyTab";
import { FinanceTab } from "./tabs/FinanceTab";
import { NavigationTab } from "./tabs/NavigationTab";
import { ThemeTab } from "./tabs/ThemeTab";
import { DirectoryTab } from "./tabs/DirectoryTab";
import { DomainsTab } from "./tabs/DomainsTab";

export default function OrganizationDetailPage() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const controller = useOrganizationDetailPageController();

  const {
    form,
    identityQuery,
    detailQuery,
    applicationsQuery,
    regionOptionsQuery,
    saveMutation,
    deleteMutation,
    organizationName,
    orgId,
  } = controller;

  // 内置组织与列表页同一保护口径：不可删除（后端同样拒绝）。
  const isBuiltIn = detailQuery.data?.slug === "built-in";

  const navItems = useMemo<SectionNavItem[]>(
    () => [
      { key: "profile", anchor: "org-section-profile", label: tr("organization.sections.profile") },
      {
        key: "branding",
        anchor: "org-section-branding",
        label: tr("organization.sections.branding"),
      },
      {
        key: "signin-password",
        anchor: "org-section-signin-password",
        label: tr("organization.sections.signinPassword"),
      },
      { key: "mfa", anchor: "org-section-mfa", label: tr("organization.sections.mfa") },
      {
        key: "accounts",
        anchor: "org-section-accounts",
        label: tr("organization.sections.accounts"),
      },
      {
        key: "directory",
        anchor: "org-section-directory",
        label: tr("organization.sections.directory"),
      },
      { key: "finance", anchor: "org-section-finance", label: tr("organization.sections.finance") },
    ],
    [tr],
  );

  // ── Error ──
  if (detailQuery.isError) {
    return (
      <Result
        status="error"
        title={t("loadFailed")}
        subTitle={serverErrorMessage(detailQuery.error, t, "loadFailed")}
        extra={<Button onClick={() => detailQuery.refetch()}>{t("retry")}</Button>}
      />
    );
  }

  // ── Not Found ──
  if (!identityQuery.isLoading && identityQuery.data === null) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Alert
          type="error"
          showIcon
          title={tr("organization.orgNotFoundTitle")}
          description={tr("organization.orgNotFoundDesc", { name: organizationName })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/organizations")}>{t("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <ResourceFormPage
      title={detailQuery.data?.displayName || organizationName}
      description={tr("organization.settingsSectionDesc")}
      loading={identityQuery.isLoading || detailQuery.isLoading}
      saving={saveMutation.isPending}
      form={form}
      onFinish={(values) => saveMutation.mutateAsync(values).then(() => undefined)}
      onCancel={() => navigate("/organizations")}
      submitPermission={{ permission: "organizations:write", resource: "organizations" }}
      extra={
        <PermissionGuard action="organizations:delete" resource="organizations" fallback={null}>
          <Popconfirm
            title={isBuiltIn ? tr("organization.builtInProtected") : t("deleteConfirm")}
            description={isBuiltIn ? undefined : tr("organization.deleteDesc")}
            onConfirm={() => deleteMutation.mutate()}
            okText={isBuiltIn ? undefined : t("delete")}
            cancelText={t("cancel")}
            okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
            disabled={isBuiltIn}
          >
            <Button danger disabled={isBuiltIn} data-testid="organization-delete">
              {t("delete")}
            </Button>
          </Popconfirm>
        </PermissionGuard>
      }
      enableDraft
      draftRoute="organizations"
      draftEntityId={orgId}
      draftIgnoreKeys={[
        "revision",
        "passwordSalt",
        "passwordObfuscatorKey",
        "defaultPassword",
        "masterPassword",
        "masterVerificationCode",
        "kerberosKeytab",
      ]}
    >
      <div style={{ gridColumn: "1 / -1", minWidth: 0 }}>
        <SectionNavShell items={navItems}>
          <div className="gl-section-anchors">
            <div id="org-section-profile" className="gl-section-anchor">
              <FormSection title={tr("organization.sections.profile")}>
                <BasicInfoTab
                  currentOrganizationId={detailQuery.data?.id ?? ""}
                  regionOptions={regionOptionsQuery.data ?? []}
                />
              </FormSection>
            </div>

            <div id="org-section-branding" className="gl-section-anchor">
              <FormSection title={tr("organization.sections.branding")}>
                <BrandingTab
                  organizationName={organizationName}
                  applicationOptions={applicationsQuery.data ?? []}
                  applicationsLoading={applicationsQuery.isLoading}
                />
                <ThemeTab />
              </FormSection>
            </div>

            <div id="org-section-signin-password" className="gl-section-anchor">
              <FormSection title={tr("organization.sections.signinPassword")}>
                <PasswordPolicyTab />
                <SecurityPolicyTab section="access" />
                <SecurityPolicyTab section="session" />
              </FormSection>
            </div>

            <div id="org-section-mfa" className="gl-section-anchor">
              <FormSection title={tr("organization.sections.mfa")}>
                <SecurityPolicyTab section="mfa" />
              </FormSection>
            </div>

            <div id="org-section-accounts" className="gl-section-anchor">
              <FormSection title={tr("organization.sections.accounts")}>
                <NavigationTab form={form} />
              </FormSection>
            </div>

            <div id="org-section-directory" className="gl-section-anchor">
              <FormSection title={tr("organization.sections.directory")}>
                <DirectoryTab organizationId={orgId} />
                <PermissionGuard action="b2b:read" fallback={null}>
                  <DomainsTab organizationId={orgId} />
                </PermissionGuard>
              </FormSection>
            </div>

            <div id="org-section-finance" className="gl-section-anchor">
              <FormSection title={tr("organization.sections.finance")}>
                <FinanceTab organizationId={orgId} />
              </FormSection>
            </div>
          </div>
        </SectionNavShell>
      </div>
    </ResourceFormPage>
  );
}
