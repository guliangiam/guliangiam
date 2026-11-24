import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { App, Form } from "antd";
import { useSessionStore } from "@/features/session/session-store";
import { createUser } from "@/shared/api/iam/user";
import { getRoleOptions } from "@/shared/api/iam/role";
import { getGroupOptions } from "@/shared/api/iam/group";
import { getOrganizationOptions } from "@/shared/api/iam/organization";
import { getApplicationOptions } from "@/shared/api/iam/application";
import { queryKeys } from "@/shared/api/query-keys";
import { checkPasswordComplexity, usePasswordRules } from "@/shared/ui/PasswordChecker";
import { buildUserCreatePayload } from "./user-create-payload";
import {
  getGenderOptions,
  getStatusOptions,
  getUserTypeOptions,
  getIdcardTypeOptions,
  getLanguageOptions,
  getBalanceCurrencyOptions,
  type UserCreateFormValues,
} from "./tabs/types";

export function useUserCreatePageController() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: ta } = useTranslation("auth");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const organization = useSessionStore((state) => state.organization);
  const passwordRules = usePasswordRules();
  const [form] = Form.useForm<UserCreateFormValues>();

  const organizationId = Form.useWatch("organizationId", form) ?? organization?.id ?? "";
  const username = Form.useWatch("username", form) ?? "new";

  // i18n-aware option constants
  const GENDER_OPTIONS = getGenderOptions(tr);
  const STATUS_OPTIONS = getStatusOptions(tc);
  const USER_TYPE_OPTIONS = getUserTypeOptions(tr);
  const IDCARD_TYPE_OPTIONS = getIdcardTypeOptions(tr);
  const LANGUAGE_OPTIONS = getLanguageOptions(tr);
  const BALANCE_CURRENCY_OPTIONS = getBalanceCurrencyOptions(tr);

  // Password modal state
  const [passwordModalOpen, setPasswordModalOpen] = useState(false);
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [passwordError, setPasswordError] = useState("");
  const [confirmPasswordError, setConfirmPasswordError] = useState("");

  // ─── Queries ─────────────────────────────────────────────────
  const organizationOptionsQuery = useQuery({
    queryKey: ["organizations", "auth", "options"],
    queryFn: getOrganizationOptions,
  });

  const roleOptionsQuery = useQuery({
    queryKey: ["roles", "auth", "options", organizationId],
    queryFn: () => getRoleOptions(organizationId),
    enabled: organizationId !== "",
  });

  const groupOptionsQuery = useQuery({
    queryKey: ["groups", "auth", "options", organizationId],
    queryFn: () => getGroupOptions(organizationId),
    enabled: organizationId !== "",
  });

  const applicationOptionsQuery = useQuery({
    queryKey: ["applications", "auth", "options", organizationId],
    queryFn: () => getApplicationOptions(organizationId),
    enabled: organizationId !== "",
  });

  // ─── Mutation ────────────────────────────────────────────────
  const createMutation = useMutation({
    mutationFn: (values: UserCreateFormValues) => createUser(buildUserCreatePayload(values)),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.users.all });
      message.success(tc("createSuccess"));
      navigate("/users");
    },
  });

  // ─── Handlers ────────────────────────────────────────────────
  const handleUserTypeChange = (value: string) => {
    if (value === "manager") {
      form.setFieldValue("isAdmin", true);
    }
  };

  const handleOpenPasswordModal = () => {
    setNewPassword("");
    setConfirmPassword("");
    setPasswordError("");
    setConfirmPasswordError("");
    setPasswordModalOpen(true);
  };

  const handlePasswordConfirm = () => {
    // 空字段必须给出可见必填提示，不能静默返回（零反馈死胡同）。
    let valid = true;
    if (!newPassword) {
      setPasswordError(ta("enterNewPassword"));
      valid = false;
    } else {
      const error = checkPasswordComplexity(newPassword, passwordRules);
      setPasswordError(error ?? "");
      if (error) valid = false;
    }
    if (!confirmPassword) {
      setConfirmPasswordError(ta("enterConfirmPassword"));
      valid = false;
    } else if (confirmPassword !== newPassword) {
      setConfirmPasswordError(ta("passwordMismatch"));
      valid = false;
    } else {
      setConfirmPasswordError("");
    }
    if (!valid) return;
    form.setFieldValue("password", newPassword);
    setPasswordModalOpen(false);
    message.success(tc("setPassword"));
  };

  const handleOrganizationChange = () => {
    form.setFieldsValue({
      signupApplicationId: undefined,
      roleBindingIds: [],
      groupIds: [],
    });
  };

  return {
    // i18n
    tc,
    ta,
    tr,
    navigate,
    // Form
    form,
    organization,
    organizationId,
    username,
    // Options
    GENDER_OPTIONS,
    STATUS_OPTIONS,
    USER_TYPE_OPTIONS,
    IDCARD_TYPE_OPTIONS,
    LANGUAGE_OPTIONS,
    BALANCE_CURRENCY_OPTIONS,
    // Password modal state
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
    // Queries
    organizationOptionsQuery,
    roleOptionsQuery,
    groupOptionsQuery,
    applicationOptionsQuery,
    // Mutation
    createMutation,
    // Handlers
    handleUserTypeChange,
    handleOpenPasswordModal,
    handlePasswordConfirm,
    handleOrganizationChange,
  };
}
