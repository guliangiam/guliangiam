/**
 * UserDetailPage — 纯 View 组件。
 *
 * 重写自 903 行单体：数据获取抽到 useUserDetailPageQueries，
 * 表单 state / mutation / 处理器抽到 useUserDetailPageActions。
 * 本文件只负责消费 controller 返回值 + 渲染分区导航 / Modals / ResourceFormPage。
 */
import { Button, Form, Input, Modal, Popconfirm, Result, Select } from "antd";
import { useEffect, type ReactNode } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { ProviderSelect } from "@/shared/ui/ProviderSelect";
import { GlFormRow } from "@/shared/ui/GlForm";
import ConsentTable from "@/shared/ui/ConsentTable";
import { SectionNavShell } from "@/features/section-nav/SectionNavShell";
import type { SectionNavItem } from "@/features/section-nav/types";
import { QueryErrorResult } from "@/shared/ui/QueryErrorResult";
import { usePermissions } from "@/shared/hooks/usePermissions";
import {
  useAccountItems,
  getUniqueTabs,
  mapTabNameToKey,
} from "@/features/account-items/useAccountItems";

import UserBasicTab from "./tabs/UserBasicTab";
import AddressTable from "@/shared/ui/AddressTable";
import UserIdentityTab from "./tabs/UserIdentityTab";
import UserProfileTab from "./tabs/UserProfileTab";
import UserRealNameTab from "./tabs/UserRealNameTab";
import UserCredentialsTab from "./tabs/UserCredentialsTab";
import UserAccountTab from "./tabs/UserAccountTab";
import ConsentsTab from "./tabs/ConsentsTab";
import UserSessionsTab from "./tabs/UserSessionsTab";
import ManagedAccountsTab from "./tabs/ManagedAccountsTab";
import { UserMfaCollapse, UserFinanceCollapse } from "./UserDetailCollapses";

import { useUserDetailPageQueries } from "./useUserDetailPageQueries";
import { useUserDetailPageActions } from "./useUserDetailPageActions";
import IdvVerifyQrModal from "./IdvVerifyQrModal";
import { USER_DETAIL_TABS, resolveUserDetailTab } from "./user-detail-tabs";
import type { UserDetailTabEntry, UserDetailTabKey } from "./user-detail-tabs";
import { getBool, getStr } from "./tabs/types";
import type { UserDetailFormValues } from "./tabs/types";

export default function UserDetailPage() {
  // 路由是 users/:organizationName/:userName —— 详情页必须用组织 slug + 用户名
  // 解析用户，再拿解析出的 user.id 驱动所有子资源请求与 mutation。
  const { organizationName, userName } = useParams<{
    organizationName: string;
    userName: string;
  }>();
  const navigate = useNavigate();
  const { t } = useTranslation();

  // 页面唯一的表单实例：所有可编辑分区只渲染 Form.Item，由 ResourceFormPage
  // 渲染唯一 <form>，因此不存在 <form> 嵌套；分区改为锚点滚动后所有分区同时
  // 挂载，保存时从整份 store 汇总所有分区的改动，跨分区编辑不会丢。
  // 校验范围随之覆盖全部分区（与页面级「保存」语义一致）。
  const [form] = Form.useForm<UserDetailFormValues>();

  // 深链：`?tab=<原 Tab key>` 是唯一导航参数；旧别名在 resolveUserDetailTab
  // 中折叠到真实 Tab，保证老链接仍能定位。
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab = resolveUserDetailTab(searchParams.get("tab"));

  const handleTabChange = (key: string) => {
    const tab = resolveUserDetailTab(key);
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (tab === "basic") next.delete("tab");
        else next.set("tab", tab);
        return next;
      },
      { replace: true },
    );
  };

  // 分区导航为锚点滚动后，`?tab=<tab key>` 仍是唯一深链参数：地址栏变化（含
  // 首次深链进入、浏览器前进后退）由 SectionNavShell 依据 initialActiveKey
  // 统一滚动到分区锚点；点击分区由 onNavigate 写回 `?tab=`。

  // 把旧别名/未知值写回规范 Tab，避免地址栏长期停留在一个无法直接解析的值。
  useEffect(() => {
    const raw = searchParams.get("tab");
    if (!raw) return;
    const canonical = resolveUserDetailTab(raw);
    if (raw === canonical) return;
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (canonical === "basic") next.delete("tab");
        else next.set("tab", canonical);
        return next;
      },
      { replace: true },
    );
  }, [searchParams, setSearchParams]);

  // ─── Controller hooks ────────────────────────────────────

  const queries = useUserDetailPageQueries(organizationName, userName);
  // 托管实名认证回跳落点：必须是详情页自身的纯路径（路由为
  // users/:organizationName/:userName，不存在 /users/:id），厂商不接受 query/hash。
  const idvReturnPath = `/users/${encodeURIComponent(organizationName ?? "")}/${encodeURIComponent(userName ?? "")}`;
  const actions = useUserDetailPageActions({
    id: queries.user?.id,
    user: queries.user,
    roleOptions: queries.roleOptions,
    groupList: queries.groupList,
    returnPath: idvReturnPath,
    form,
  });

  const fieldLabels: Record<string, string> = {
    name: t("username"),
    displayName: t("displayName"),
    email: t("email"),
    phone: t("resource:user.phone"),
    firstName: t("resource:user.firstName"),
    lastName: t("resource:user.lastName"),
    affiliation: t("resource:user.affiliation"),
    title: t("resource:user.title"),
    signupApplication: t("resource:user.signupApplication"),
    userType: t("userType"),
    status: t("resource:user.status"),
    tag: t("resource:user.tag"),
    score: t("resource:user.score"),
    karma: t("resource:user.karma"),
    ranking: t("resource:user.ranking"),
    externalId: t("resource:user.externalId"),
    ipWhitelist: t("resource:user.ipWhitelist"),
    homepage: t("resource:user.homepage"),
    education: t("resource:user.education"),
    language: t("language"),
    countryCode: t("resource:user.countryCode"),
    region: t("region"),
    location: t("resource:user.location"),
    gender: t("resource:user.gender"),
    birthday: t("resource:user.birthday"),
    bio: t("resource:user.bio"),
    realName: t("resource:user.realName"),
    idCardType: t("resource:user.idCardType"),
    idCard: t("resource:user.idCard"),
  };
  const fieldAliases = { username: "name" };

  const { user, isLoading, isError, error, refetchUser, providers, accountItems } = queries;
  const organizationDisplayName = queries.organization?.displayName;
  const customAttributeDefinitions = queries.customAttributeDefinitions;

  const {
    addresses,
    setAddresses,
    managedAccounts,
    setManagedAccounts,
    applicationScopes,
    setApplicationScopes,
    cart,
    setCart,
    faceIds,
    setFaceIds,
    identities,
    customProperties,
    setCustomProperties,
    // modals
    addIdentityVisible,
    newIdentity,
    setNewIdentity,
    openAddIdentity,
    closeAddIdentity,
    addRoleVisible,
    selectedRoleId,
    setSelectedRoleId,
    openAddRole,
    closeAddRole,
    addGroupVisible,
    selectedGroupId,
    setSelectedGroupId,
    openAddGroup,
    closeAddGroup,
    // mutations
    saveAllMutation,
    verifyIdMutation,
    // hosted IDV challenge
    idvQrVisible,
    idvQrUrl,
    idvCollecting,
    closeIdvQr,
    deleteMutation,
    mfaMutation,
    preferredMfaMutation,
    recognizeDocumentMutation,
    rotateSecretMutation,
    rotatedSecret,
    dismissRotatedSecret,
    // handlers
    handleReset,
    handleMainSave,
    handleSectionSave,
    handleCredentialsSave,
    handleAddIdentity,
    handleDeleteIdentity,
    handleLinkIdentity,
    handleAddRole,
    handleRemoveRole,
    roleSavePending,
    handleAddGroup,
    handleRemoveGroup,
    groupSavePending,
  } = actions;

  const fieldPermissions = useAccountItems({ accountItems, viewerRole: "admin" });
  const uniqueTabs = getUniqueTabs(accountItems);
  // 写权限是保存的唯一前置条件：无 users:write 时隐藏保存入口并把属性编辑器只读化，
  // 不再依赖后端 403 才让用户发现「能点但存不了」。
  const { hasAction } = usePermissions();
  const canWrite = hasAction("users:write");

  // ─── Error state ─────────────────────────────────────────

  if (isError) {
    // 404 → 友好 NotFound、403 → 权限拒绝、其余 → 可重试错误态（对照工单详情）。
    return (
      <QueryErrorResult
        error={error}
        onRetry={() => void refetchUser()}
        notFoundDescription={`${organizationName ?? "-"} / ${userName ?? "-"}`}
      />
    );
  }

  // 组织或用户名不存在：显式 404，而不是渲染一页空表单。
  if (!isLoading && !user) {
    return (
      <Result
        status="404"
        title={t("common:notFound", "Not found")}
        subTitle={`${organizationName ?? "-"} / ${userName ?? "-"}`}
        extra={<Button onClick={() => navigate("/users")}>{t("common:backToList", "Back")}</Button>}
      />
    );
  }

  // ─── Sections ────────────────────────────────────────────

  // 8 个业务分区：合并后的分区内部仍以独立卡片堆叠，保持统一的卡片外边距与
  // 视觉区隔（FormSection 是全局卡片壳，内部标题不吸顶）。
  const tabChildren: Record<UserDetailTabKey, ReactNode> = {
    basic: (
      <div className="gl-section-stack">
        <UserBasicTab
          form={form}
          owner={user?.organizationId}
          organizationName={organizationDisplayName}
          name={user?.username}
          userId={user?.id}
          organizationId={user?.organizationId}
          currentCountryCode={user?.profile?.countryCode as string | undefined}
          fieldPermissions={fieldPermissions}
          onContactUpdated={() => void refetchUser()}
        />
        <FormSection title={t("resource:user.tabs.profile")}>
          <UserProfileTab
            customProperties={customProperties}
            onCustomPropertiesChange={setCustomProperties}
            definitions={customAttributeDefinitions}
            readOnly={!canWrite}
          />
        </FormSection>
        {/* 地址子模块自「实名与属性」迁入「资料」：主表单只保留常用地址字段，
            多地址集合仍走 PATCH collections 持久化。 */}
        <FormSection
          title={t("resource:user.addresses")}
          description={t("resource:user.address.hint", {
            defaultValue: "Country → province → city → district → street",
          })}
        >
          <AddressTable value={addresses} onChange={setAddresses} readOnly={!canWrite} />
        </FormSection>
      </div>
    ),
    realname: (
      <div className="gl-section-stack">
        <UserRealNameTab
          form={form}
          owner={user?.organizationId}
          name={user?.username}
          isVerified={user?.isVerified}
          onVerifyIdentification={() => verifyIdMutation.mutate()}
          isVerifying={verifyIdMutation.isPending}
          verifyingLabel={idvCollecting ? t("verifyIdCollecting") : undefined}
          canWrite={canWrite}
          onRecognizeDocument={(documentType, file, side) =>
            recognizeDocumentMutation.mutate({ documentType, file, side })
          }
          recognizing={recognizeDocumentMutation.isPending}
          idCardSet={getBool(user?.profile, "idCardSet")}
          idCardMasked={getStr(user?.profile, "idCardMasked")}
          passportNoSet={getBool(user?.profile, "passportNoSet")}
          passportNoMasked={getStr(user?.profile, "passportNoMasked")}
          driverLicenseNoSet={getBool(user?.profile, "driverLicenseNoSet")}
          driverLicenseNoMasked={getStr(user?.profile, "driverLicenseNoMasked")}
        />
      </div>
    ),
    identity: (
      <UserIdentityTab
        user={user}
        identities={identities}
        providers={providers}
        onDeleteIdentity={handleDeleteIdentity}
        onAddIdentity={openAddIdentity}
        onLinkIdentity={handleLinkIdentity}
      />
    ),
    account: user ? (
      <UserAccountTab
        user={user}
        signupApplicationName={queries.signupApplicationName}
        effectivePermissions={queries.effectivePermissions}
        onRemoveRole={handleRemoveRole}
        onAddRole={openAddRole}
        onRemoveGroup={handleRemoveGroup}
        onAddGroup={openAddGroup}
      />
    ) : null,
    credentials: (
      <UserCredentialsTab
        user={user}
        canWrite={canWrite}
        onRotateSecret={() => rotateSecretMutation.mutate()}
        rotatePending={rotateSecretMutation.isPending}
        rotatedSecret={rotatedSecret}
        onDismissRotatedSecret={dismissRotatedSecret}
        faceIds={faceIds}
        onRemoveFaceId={(index) => setFaceIds((prev) => prev.filter((_, i) => i !== index))}
        onAppendFaceId={(faceId) => setFaceIds((prev) => [...prev, faceId])}
        onGoToRealName={() => handleTabChange("realname")}
      />
    ),
    mfa: user ? (
      <UserMfaCollapse
        user={user}
        onUpdateMfaMethods={(methods) => mfaMutation.mutate(methods)}
        onSetPreferredMfa={(mfaType) => preferredMfaMutation.mutate(mfaType)}
        canWrite={canWrite}
        mfaPending={mfaMutation.isPending || preferredMfaMutation.isPending}
      />
    ) : null,
    sessions: user ? (
      <div className="gl-section-stack">
        <FormSection title={t("resource:user.tabs.sessions")}>
          <UserSessionsTab userId={user.id} organizationId={user.organizationId} />
        </FormSection>
        <FormSection title={t("resource:user.tabs.consents")}>
          <ConsentsTab userId={user.id} organizationId={user.organizationId} />
        </FormSection>
        <FormSection title={t("resource:user.applicationScopes", "Application scopes")}>
          <ConsentTable
            value={applicationScopes}
            onChange={setApplicationScopes}
            readOnly={!canWrite}
          />
        </FormSection>
      </div>
    ) : null,
    assets: user ? (
      <div className="gl-section-stack">
        <FormSection title={t("resource:user.tabs.finance")}>
          <UserFinanceCollapse user={user} cart={cart} onCartChange={setCart} canWrite={canWrite} />
        </FormSection>
        <FormSection title={t("resource:user.tabs.managedAccounts")}>
          <ManagedAccountsTab
            managedAccounts={managedAccounts}
            onChange={setManagedAccounts}
            canWrite={canWrite}
          />
        </FormSection>
      </div>
    ) : null,
  };

  // accountItems 若声明了分区顺序则优先；未声明时按分组顺序展开。
  const tabRank = (key: UserDetailTabKey) => {
    const configured = uniqueTabs.findIndex((ut) => mapTabNameToKey(ut) === key);
    if (configured >= 0) return configured;
    const fallback = USER_DETAIL_TABS.findIndex((tab) => tab.key === key);
    return USER_DETAIL_TABS.length + fallback;
  };

  const orderedTabs: UserDetailTabEntry[] = [...USER_DETAIL_TABS].sort(
    (a, b) => tabRank(a.key) - tabRank(b.key),
  );

  // 分区清单是导航条目与分区容器的唯一事实源：两处都由 orderedTabs 派生，
  // 顺序与标签天然一致，不会出现「导航有、分区没有」的错位。
  const sectionNavItems: SectionNavItem[] = orderedTabs.map((entry) => ({
    key: entry.key,
    anchor: `user-section-${entry.key}`,
    label: uniqueTabs.find((ut) => mapTabNameToKey(ut) === entry.key) || t(entry.labelKey),
  }));

  // ─── Active-tab save wiring ──────────────────────────────
  // 每个分区都显示同一组「重置 / 保存」按钮。主阶段按分区决定：basic/account →
  // PATCH /users + PUT /profile（资料分区同时承载地址集合 PATCH /collections，
  // 地址编辑不再被实名分区阻塞），credentials → PUT /credentials；其余分区只提交
  // 确实有改动的阶段（含 collections），保证按钮存在且不会产生空写入。
  const savePending = saveAllMutation.isPending;
  const activeSave: {
    save: (values: UserDetailFormValues) => void | boolean | Promise<void | boolean>;
    pending: boolean;
  } =
    activeTab === "credentials"
      ? { save: handleCredentialsSave, pending: savePending }
      : activeTab === "basic" || activeTab === "account"
        ? { save: handleMainSave, pending: savePending }
        : { save: handleSectionSave, pending: savePending };

  // ─── Render ───────────────────────────────────────────────

  return (
    <ResourceFormPage
      title={user?.username ?? "User Detail"}
      description={user?.displayName ?? ""}
      loading={isLoading}
      form={form}
      fieldLabels={fieldLabels}
      fieldAliases={fieldAliases}
      onFinish={activeSave.save}
      saving={activeSave.pending}
      submitPermission={{ permission: "users:write", resource: "users" }}
      onCancel={() => navigate("/users")}
      onReset={handleReset}
      extra={
        <PermissionGuard action="users:write" resource="users" fallback={null}>
          <Popconfirm
            title={t("deleteConfirm", "Are you sure you want to delete this user?")}
            onConfirm={() => deleteMutation.mutate()}
          >
            <Button danger loading={deleteMutation.isPending}>
              {t("common:delete")}
            </Button>
          </Popconfirm>
        </PermissionGuard>
      }
      enableDraft
      draftRoute="user"
      draftEntityId={user?.id}
    >
      {/* 分区导航：所有分区同时挂载、单列堆叠，由 SectionNavShell 在右锚点 /
          左二级 / 顶部横条之间切换落位；`?tab=<tab key>` 深链保持可用。 */}
      <SectionNavShell
        items={sectionNavItems}
        initialActiveKey={activeTab}
        onNavigate={handleTabChange}
      >
        <div className="gl-section-anchors">
          {orderedTabs.map((entry) => (
            <div key={entry.key} id={`user-section-${entry.key}`} className="gl-section-anchor">
              {tabChildren[entry.key]}
            </div>
          ))}
        </div>
      </SectionNavShell>

      {/* 托管实名认证扫码弹窗（结果由 useIdvPendingPolling 收尾） */}
      <IdvVerifyQrModal open={idvQrVisible} verifyUrl={idvQrUrl} onClose={closeIdvQr} />

      {/* Add Identity Modal */}
      <Modal
        title={t("resource:user.addIdentity", "Add Identity")}
        open={addIdentityVisible}
        onOk={handleAddIdentity}
        onCancel={() => {
          closeAddIdentity();
          setNewIdentity({ providerId: "", identityType: "", subject: "", providerAccount: "" });
        }}
        okText={t("common:save")}
        destroyOnHidden
      >
        <Form layout="vertical">
          <Form.Item>
            <GlFormRow label={t("providerType", "Provider")}>
              <ProviderSelect
                value={newIdentity.providerId || undefined}
                onChange={(v) => setNewIdentity((prev) => ({ ...prev, providerId: v as string }))}
                placeholder={t("resource:user.selectProvider", "Select provider")}
                providers={providers}
              />
            </GlFormRow>
          </Form.Item>
          <Form.Item>
            <GlFormRow label={t("resource:user.identityType", "Identity Type")}>
              <Select
                value={newIdentity.identityType || undefined}
                onChange={(v) => setNewIdentity((prev) => ({ ...prev, identityType: v }))}
                placeholder={t("resource:user.selectIdentityType", "Select identity type")}
                options={[
                  { label: "OAuth2", value: "oauth2" },
                  { label: "OIDC", value: "oidc" },
                  { label: "SAML", value: "saml" },
                  { label: "LDAP", value: "ldap" },
                  { label: "CAS", value: "cas" },
                  { label: "Custom", value: "custom" },
                ]}
              />
            </GlFormRow>
          </Form.Item>
          <Form.Item>
            <GlFormRow label={t("resource:user.subject", "Subject")}>
              <Input
                value={newIdentity.subject}
                onChange={(e) => setNewIdentity((prev) => ({ ...prev, subject: e.target.value }))}
                placeholder={t(
                  "resource:user.subjectPlaceholder",
                  "Unique identifier from provider",
                )}
              />
            </GlFormRow>
          </Form.Item>
          <Form.Item>
            <GlFormRow label={t("resource:user.providerAccount", "Provider Account")}>
              <Input
                value={newIdentity.providerAccount}
                onChange={(e) =>
                  setNewIdentity((prev) => ({ ...prev, providerAccount: e.target.value }))
                }
                placeholder={t(
                  "resource:user.providerAccountPlaceholder",
                  "Account name at provider (optional)",
                )}
              />
            </GlFormRow>
          </Form.Item>
        </Form>
      </Modal>

      {/* Add Role Modal */}
      <Modal
        title={t("resource:user.addRole", "Add Role")}
        open={addRoleVisible}
        onOk={handleAddRole}
        onCancel={() => {
          closeAddRole();
          setSelectedRoleId("");
        }}
        okText={t("common:save")}
        okButtonProps={{ loading: roleSavePending }}
        destroyOnHidden
      >
        <Select
          value={selectedRoleId || undefined}
          onChange={setSelectedRoleId}
          placeholder={t("resource:user.selectRole", "Select role")}
          id="add-role-select"
          aria-label={t("resource:user.selectRole", "Select role")}
          style={{ width: "100%" }}
          options={
            queries.roleOptions?.map((r: { id: string; displayName: string; name: string }) => ({
              label: `${r.displayName} (${r.name})`,
              value: r.id,
            })) ?? []
          }
          showSearch
          optionFilterProp="label"
        />
      </Modal>

      {/* Add Group Modal */}
      <Modal
        title={t("resource:user.addGroup", "Add Group")}
        open={addGroupVisible}
        onOk={handleAddGroup}
        onCancel={() => {
          closeAddGroup();
          setSelectedGroupId("");
        }}
        okText={t("common:save")}
        okButtonProps={{ loading: groupSavePending }}
        destroyOnHidden
      >
        <Select
          value={selectedGroupId || undefined}
          onChange={setSelectedGroupId}
          placeholder={t("resource:user.selectGroup", "Select group")}
          id="add-group-select"
          aria-label={t("resource:user.selectGroup", "Select group")}
          style={{ width: "100%" }}
          options={
            queries.groupList?.map((g: { id: string; displayName: string }) => ({
              label: g.displayName,
              value: g.id,
            })) ?? []
          }
          showSearch
          optionFilterProp="label"
        />
      </Modal>
    </ResourceFormPage>
  );
}
