import {
  Button,
  Col,
  DatePicker,
  Form,
  Input,
  InputNumber,
  Modal,
  Row,
  Select,
  Switch,
} from "antd";
import type { ZodIssue } from "zod";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { CountryCodeSelect } from "@/shared/ui/CountryCodeSelect";
import { GroupTreeSelect } from "@/shared/ui/GroupTreeSelect";
import { ImageCropUpload } from "@/shared/ui/ImageCropUpload";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { RegionSelect } from "@/shared/ui/RegionSelect";
import {
  PasswordChecker,
  checkPasswordComplexity,
  usePasswordRules,
} from "@/shared/ui/PasswordChecker";
import { PasswordGenerateButton } from "@/shared/ui/PasswordGenerator";
import { usePasswordPolicy } from "@/shared/hooks/usePasswordPolicy";
import { userCreateSchema } from "@/shared/api/iam/schemas";
import { applyZodErrorsToForm } from "@/shared/utils/zod-form";
import { useUserCreatePageController } from "./useUserCreatePageController";
import type { UserCreateFormValues } from "./tabs/types";

export default function UserCreatePage() {
  const {
    tc,
    ta,
    tr,
    navigate,
    form,
    organization,
    organizationId,
    username,
    GENDER_OPTIONS,
    STATUS_OPTIONS,
    USER_TYPE_OPTIONS,
    IDCARD_TYPE_OPTIONS,
    LANGUAGE_OPTIONS,
    BALANCE_CURRENCY_OPTIONS,
    passwordModalOpen,
    setPasswordModalOpen,
    newPassword,
    setNewPassword,
    confirmPassword,
    setConfirmPassword,
    passwordError,
    setPasswordError,
    confirmPasswordError,
    setConfirmPasswordError,
    organizationOptionsQuery,
    roleOptionsQuery,
    groupOptionsQuery,
    applicationOptionsQuery,
    createMutation,
    handleUserTypeChange,
    handleOpenPasswordModal,
    handlePasswordConfirm,
    handleOrganizationChange,
  } = useUserCreatePageController();

  const passwordPolicy = usePasswordPolicy();
  const passwordRules = usePasswordRules();
  const confirmMismatch = Boolean(
    newPassword && confirmPassword && newPassword !== confirmPassword,
  );
  const confirmError = confirmPasswordError || (confirmMismatch ? ta("passwordMismatch") : "");

  const fieldLabels: Record<string, string> = {
    organizationId: tc("organization"),
    username: tc("username"),
    displayName: tc("displayName"),
    email: ta("email"),
    phone: ta("phone"),
    password: tc("password"),
    userType: tr("user.userType"),
    status: tc("statusLabel"),
    homepage: tr("user.homepage"),
    signupApplicationId: tr("user.signupApplication"),
  };

  const fieldAliases: Record<string, string> = {
    name: "username",
    owner: "organizationId",
    signupApplication: "signupApplicationId",
  };

  const localizedZodMessage = (issue: ZodIssue) => {
    const field = issue.path[0];
    if (typeof field !== "string") return undefined;
    switch (field) {
      case "organizationId":
        return tr("user.validation.organizationRequired");
      case "username":
        return issue.code === "too_big"
          ? tc("form.maxLength", { max: 100 })
          : tc("form.minLength", { min: 2 });
      case "displayName":
        return issue.code === "too_big"
          ? tc("form.maxLength", { max: 200 })
          : tr("user.validation.displayNameRequired");
      case "email":
        return tc("form.invalidEmail");
      case "homepage":
        return tc("form.invalidUrl");
      case "password":
        return issue.code === "too_big"
          ? tc("form.maxLength", { max: passwordPolicy.maxLength })
          : tr("user.validation.passwordMinLength", { min: passwordPolicy.minLength });
      case "userType":
        return tr("user.validation.userTypeRequired");
      case "status":
        return tr("user.validation.statusRequired");
      default:
        return undefined;
    }
  };

  return (
    <ResourceFormPage
      title={tr("user.createUser")}
      description={tr("user.createUserDesc", {})}
      loading={organizationOptionsQuery.isLoading}
      saving={createMutation.isPending}
      form={form}
      error={organizationOptionsQuery.isError ? tc("error") : null}
      fieldLabels={fieldLabels}
      fieldAliases={fieldAliases}
      enableDraft
      draftRoute="users"
      draftEntityId={"new"}
      onCancel={() => navigate("/users")}
      onFinish={async (values) => {
        if (organizationOptionsQuery.isError) return false;
        // password 没有对应的 Form.Item，validateFields() 不会带上它，这里显式从 store 补齐。
        const payload = {
          ...values,
          password: form.getFieldValue("password"),
        } as UserCreateFormValues;
        const result = userCreateSchema.safeParse(payload);
        if (!result.success) {
          applyZodErrorsToForm(form, result.error, localizedZodMessage);
          return false;
        }
        await createMutation.mutateAsync(payload);
        return true;
      }}
    >
      {/* ── 账号 ── */}
      <FormSection
        title={tr("user.basicSection")}
        description={tr("user.basicSectionDesc")}
        required
      >
        <Row gutter={16}>
          <Col xs={24} md={12}>
            <Form.Item
              name="organizationId"
              initialValue={organization?.id ?? ""}
              rules={[
                {
                  required: true,
                  message: tr("user.validation.organizationRequired", {}),
                },
              ]}
            >
              <GlFormRow label={tc("organization")} required>
                <OrganizationSelect onChange={handleOrganizationChange} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item
              name="username"
              rules={[
                {
                  required: true,
                  message: tr("user.validation.nameRequired"),
                },
              ]}
            >
              <GlFormRow label={tc("username")} required helpKey="users.username">
                <Input placeholder={ta("enterUsername")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item
              name="displayName"
              rules={[
                {
                  required: true,
                  message: tr("user.validation.displayNameRequired"),
                },
              ]}
            >
              <GlFormRow label={tc("displayName")} required helpKey="users.displayName">
                <Input placeholder={tc("displayName")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item
              name="userType"
              initialValue="normal"
              rules={[
                {
                  required: true,
                  message: tr("user.validation.userTypeRequired", {}),
                },
              ]}
            >
              <GlFormRow label={tr("user.userType")} required helpKey="users.userType">
                <Select
                  options={USER_TYPE_OPTIONS}
                  showSearch
                  optionFilterProp="label"
                  onChange={handleUserTypeChange}
                />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item
              name="status"
              initialValue="active"
              rules={[
                {
                  required: true,
                  message: tr("user.validation.statusRequired"),
                },
              ]}
            >
              <GlFormRow label={tc("statusLabel")} required helpKey="users.status">
                <Select options={STATUS_OPTIONS} showSearch optionFilterProp="label" />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item name="signupApplicationId">
              <GlFormRow label={tr("user.signupApplication")}>
                <Select
                  allowClear
                  showSearch
                  optionFilterProp="label"
                  disabled={organizationId === "" || applicationOptionsQuery.isError}
                  placeholder={
                    applicationOptionsQuery.isError
                      ? tc("loadFailed")
                      : organizationId === ""
                        ? tr("user.selectOrgFirst")
                        : tr("user.selectApplication")
                  }
                  loading={applicationOptionsQuery.isLoading}
                  options={(applicationOptionsQuery.data ?? []).map((item) => ({
                    label: item.displayName ? `${item.displayName} (${item.slug})` : item.slug,
                    value: item.id,
                  }))}
                />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item name="externalId">
              <GlFormRow label={tr("user.externalId")}>
                <Input placeholder={tr("user.placeholder.externalId")} />
              </GlFormRow>
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      {/* ── 资料 ── */}
      <FormSection
        title={tr("user.tabs.profile")}
        description={tr("user.profileSectionDesc")}
        collapsible
        defaultCollapsed={false}
      >
        <Row gutter={16}>
          <Col xs={24} md={8}>
            <Form.Item name="avatarUrl">
              <GlFormRow label={tr("user.avatar")}>
                <ImageCropUpload
                  tag="avatar"
                  owner={organizationId}
                  name={username}
                  shape="circle"
                />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="firstName">
              <GlFormRow label={tr("user.firstName")}>
                <Input placeholder={tr("user.firstName")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="lastName">
              <GlFormRow label={tr("user.lastName")}>
                <Input placeholder={tr("user.lastName")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item
              name="email"
              rules={[
                {
                  type: "email",
                  message: tc("form.invalidEmail"),
                },
              ]}
            >
              <GlFormRow label={ta("email")} helpKey="users.email">
                <Input type="email" placeholder={ta("enterEmail")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item name="phone">
              <GlFormRow label={ta("phone")} helpKey="users.phone">
                <Input placeholder={ta("enterPhone")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="countryCode">
              <GlFormRow label={ta("countryCode")}>
                <CountryCodeSelect ariaLabel={ta("countryCode")} style={{ width: "100%" }} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="region">
              <GlFormRow label={tc("region")}>
                <RegionSelect ariaLabel={tc("region")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="location">
              <GlFormRow label={tr("user.location")}>
                <Input placeholder={tr("user.placeholder.location")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="gender">
              <GlFormRow label={tr("user.gender")}>
                <Select
                  options={GENDER_OPTIONS}
                  allowClear
                  placeholder={tr("user.placeholder.gender")}
                />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="birthday">
              <GlFormRow label={tr("user.birthday")}>
                <DatePicker style={{ width: "100%" }} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="language">
              <GlFormRow label={tr("user.language")}>
                <Select
                  options={LANGUAGE_OPTIONS}
                  allowClear
                  showSearch
                  optionFilterProp="label"
                  placeholder={tr("user.placeholder.language")}
                />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item name="homepage" rules={[{ type: "url", message: tc("form.invalidUrl") }]}>
              <GlFormRow label={tr("user.homepage")}>
                <Input placeholder="https://example.com" />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item name="bio">
              <GlFormRow label={tr("user.bio")}>
                <Input.TextArea rows={2} placeholder={tr("user.placeholder.bio")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item name="education">
              <GlFormRow label={tr("user.education")}>
                <Input placeholder={tr("user.placeholder.education")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item name="tag">
              <GlFormRow label={tr("user.tag")}>
                <Input placeholder={tr("user.placeholder.tag")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="realName">
              <GlFormRow label={tr("user.realName")}>
                <Input placeholder={tr("user.placeholder.realName")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="idCardType">
              <GlFormRow label={tr("user.idCardType")}>
                <Select
                  options={IDCARD_TYPE_OPTIONS}
                  allowClear
                  placeholder={tc("placeholder.selectType")}
                />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="idCard">
              <GlFormRow label={tr("user.idCard")}>
                <Input placeholder={tr("user.placeholder.idCard")} />
              </GlFormRow>
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      {/* ── 组织与角色 ── */}
      <FormSection
        title={tr("user.roleSection")}
        description={tr("user.roleSectionDesc")}
        collapsible
        defaultCollapsed
      >
        <Row gutter={16}>
          <Col xs={24} md={8}>
            <Form.Item name="isAdmin" valuePropName="checked" initialValue={false}>
              <GlFormRow label={tr("user.isAdmin")} helpKey="users.isAdmin" inline>
                <Switch />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={16}>
            <Form.Item name="roleBindingIds">
              <GlFormRow label={tr("user.initialRoles")}>
                <Select
                  mode="multiple"
                  allowClear
                  showSearch
                  optionFilterProp="label"
                  loading={roleOptionsQuery.isLoading}
                  disabled={organizationId === "" || roleOptionsQuery.isError}
                  placeholder={
                    roleOptionsQuery.isError
                      ? tc("loadFailed")
                      : organizationId === ""
                        ? tr("user.selectOrgFirst")
                        : tr("user.placeholder.selectRoles")
                  }
                  options={(roleOptionsQuery.data ?? []).map((item) => ({
                    label: item.displayName ? `${item.displayName} (${item.name})` : item.name,
                    value: item.id,
                  }))}
                />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item name="groupIds">
              <GlFormRow label={tr("user.groupSection")}>
                <GroupTreeSelect
                  multiple
                  groups={groupOptionsQuery.data ?? []}
                  disabled={organizationId === "" || groupOptionsQuery.isError}
                  placeholder={
                    groupOptionsQuery.isError
                      ? tc("loadFailed")
                      : organizationId === ""
                        ? tr("user.selectOrgFirst")
                        : tr("user.groupSection")
                  }
                />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="affiliation">
              <GlFormRow label={tr("user.affiliation")}>
                <Input placeholder={tr("user.placeholder.affiliation")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="title">
              <GlFormRow label={tr("user.jobTitle")}>
                <Input placeholder={tr("user.placeholder.title")} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="score" initialValue={0}>
              <GlFormRow label={tr("user.score")}>
                <InputNumber min={0} style={{ width: "100%" }} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="karma" initialValue={0}>
              <GlFormRow label={tr("user.karma")}>
                <InputNumber min={0} style={{ width: "100%" }} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="ranking" initialValue={0}>
              <GlFormRow label={tr("user.ranking")}>
                <InputNumber min={0} style={{ width: "100%" }} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="balance" initialValue={0}>
              <GlFormRow label={tr("user.balance")}>
                <InputNumber min={0} style={{ width: "100%" }} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="balanceCredit" initialValue={0}>
              <GlFormRow label={tr("user.balanceCredit")}>
                <InputNumber min={0} style={{ width: "100%" }} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="balanceCurrency" initialValue="USD">
              <GlFormRow label={tr("user.balanceCurrency")}>
                <Select options={BALANCE_CURRENCY_OPTIONS} showSearch optionFilterProp="label" />
              </GlFormRow>
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      {/* ── 安全 ── */}
      <FormSection title={tr("user.groups.security")} collapsible defaultCollapsed>
        <Row gutter={16}>
          <Col xs={24} md={12}>
            <Form.Item shouldUpdate={(prev, cur) => prev.password !== cur.password}>
              {({ getFieldValue }) => {
                const passwordSet = Boolean(getFieldValue("password"));
                return (
                  <GlFormRow label={tc("password")}>
                    <Button onClick={handleOpenPasswordModal}>
                      {passwordSet ? tc("changePassword") : tc("setPassword")}
                    </Button>
                    {passwordSet && (
                      <span style={{ marginLeft: 8, color: "var(--gl-color-success)" }}>✓</span>
                    )}
                  </GlFormRow>
                );
              }}
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="isForbidden" valuePropName="checked" initialValue={false}>
              <GlFormRow label={tr("user.isForbidden")} inline>
                <Switch />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="isDeleted" valuePropName="checked" initialValue={false}>
              <GlFormRow label={tr("user.isDeleted")} inline>
                <Switch />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item name="needUpdatePassword" valuePropName="checked" initialValue={false}>
              <GlFormRow label={tr("user.needUpdatePassword")} inline>
                <Switch />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item name="ipWhitelist">
              <GlFormRow label={tr("user.ipWhitelist")}>
                <Input placeholder={tr("user.placeholder.ipWhitelist")} />
              </GlFormRow>
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      {/* ── Password Modal ── */}
      <Modal
        title={tc("setPassword")}
        open={passwordModalOpen}
        okText={tc("setPassword")}
        cancelText={tc("cancel")}
        onOk={handlePasswordConfirm}
        onCancel={() => setPasswordModalOpen(false)}
        width={500}
      >
        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          <div>
            <div style={{ marginBottom: 4, fontWeight: 500 }}>{ta("newPassword")}</div>
            <Input.Password
              placeholder={ta("enterNewPassword")}
              value={newPassword}
              onChange={(e) => {
                setNewPassword(e.target.value);
                if (e.target.value) {
                  const error = checkPasswordComplexity(e.target.value, passwordRules);
                  setPasswordError(error ?? "");
                } else {
                  setPasswordError("");
                }
              }}
              suffix={
                <PasswordGenerateButton
                  onGenerated={(generated) => {
                    setNewPassword(generated);
                    setPasswordError(checkPasswordComplexity(generated, passwordRules) ?? "");
                  }}
                />
              }
              status={passwordError ? "error" : undefined}
            />
          </div>
          {newPassword && <PasswordChecker password={newPassword} />}
          {passwordError && (
            <span style={{ color: "var(--gl-color-error)", fontSize: 12 }}>{passwordError}</span>
          )}
          <div>
            <div style={{ marginBottom: 4, fontWeight: 500 }}>{ta("confirmPassword")}</div>
            <Input.Password
              placeholder={ta("enterConfirmPassword")}
              value={confirmPassword}
              onChange={(e) => {
                setConfirmPassword(e.target.value);
                setConfirmPasswordError(
                  e.target.value && newPassword && e.target.value !== newPassword
                    ? ta("passwordMismatch")
                    : "",
                );
              }}
              status={confirmError ? "error" : undefined}
            />
          </div>
          {confirmError && (
            <span style={{ color: "var(--gl-color-error)", fontSize: 12 }}>{confirmError}</span>
          )}
        </div>
      </Modal>
    </ResourceFormPage>
  );
}
