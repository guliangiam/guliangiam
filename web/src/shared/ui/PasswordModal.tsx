import { App, Button, Input, Modal, Popover, Space, Typography } from "antd";
import { type FC, useCallback, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { getErrorDetails } from "@/shared/api/http";
import { changeOwnPassword } from "@/shared/api/iam/auth";
import { setUserPassword } from "@/shared/api/iam/user";
import { usePasswordPolicy } from "@/shared/hooks/usePasswordPolicy";
import { PasswordChecker, buildPasswordRules, checkPasswordComplexity } from "./PasswordChecker";

type PasswordModalProps = {
  userId: string;
  /** Tenant of the target user; admins may reset users outside their session org. */
  organizationId?: string;
  /** 'admin' = 管理员重置他人口令（users:password:reset）, 'self' = 用户自助改密（需校验当前口令） */
  mode?: "admin" | "self";
  hasPassword?: boolean;
  disabled?: boolean;
  onSuccess?: () => void;
};

export const PasswordModal: FC<PasswordModalProps> = ({
  userId,
  organizationId,
  mode = "admin",
  disabled,
  onSuccess,
}) => {
  const { t } = useTranslation("common");
  const { message } = App.useApp();
  const policy = usePasswordPolicy();
  const [open, setOpen] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmLoading, setConfirmLoading] = useState(false);
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [rePassword, setRePassword] = useState("");
  const [reason, setReason] = useState("");
  const [newPasswordError, setNewPasswordError] = useState("");
  const [rePasswordError, setRePasswordError] = useState("");
  const [submitError, setSubmitError] = useState("");
  const [submitFailures, setSubmitFailures] = useState<string[]>([]);
  const [popoverOpen, setPopoverOpen] = useState(false);

  // 只有自助改密才校验当前口令；管理员重置不需要目标用户的口令。
  const requireOldPassword = mode === "self";

  // 规则完全由会话中的组织策略生成，不再读取页面传入的硬编码选项。
  const rules = useMemo(() => buildPasswordRules(policy, t), [policy, t]);

  const showModal = useCallback(() => setOpen(true), []);

  const handleCancel = useCallback(() => {
    setOpen(false);
    setConfirmOpen(false);
    setOldPassword("");
    setNewPassword("");
    setRePassword("");
    setReason("");
    setNewPasswordError("");
    setRePasswordError("");
    setSubmitError("");
    setSubmitFailures([]);
  }, []);

  const handleNewPasswordChange = useCallback(
    (value: string) => {
      setNewPassword(value);
      const error = checkPasswordComplexity(value, rules);
      setNewPasswordError(error ?? "");
    },
    [rules],
  );

  const handleRePasswordChange = useCallback(
    (value: string) => {
      setRePassword(value);
      if (value !== newPassword) {
        setRePasswordError(t("auth:passwordMismatch", "Passwords do not match"));
      } else {
        setRePasswordError("");
      }
    },
    [newPassword, t],
  );

  // 服务端拒绝时原样展示错误；若带最小长度明细则用本地化文案，避免出现
  // 与表单规则不同的第二套门槛提示。
  const resolveSubmitError = useCallback(
    (error: unknown): { message: string; failures: string[] } => {
      const details = getErrorDetails(error);
      const rawFailures = details?.failures;
      const failures = Array.isArray(rawFailures)
        ? rawFailures
            .map((item) =>
              item &&
              typeof item === "object" &&
              typeof (item as { message?: unknown }).message === "string"
                ? (item as { message: string }).message
                : "",
            )
            .filter((failure) => failure !== "")
        : [];
      if (details?.field === "password" && typeof details.min === "number") {
        return { message: t("form.minLength", { min: details.min }), failures };
      }
      if (details?.field === "password" && typeof details.max === "number") {
        return { message: t("form.maxLength", { max: details.max }), failures };
      }
      return { message: error instanceof Error && error.message ? error.message : "", failures };
    },
    [t],
  );

  const handleConfirm = useCallback(async () => {
    setConfirmLoading(true);
    setSubmitError("");
    setSubmitFailures([]);
    try {
      if (mode === "self") {
        // 自助改密：后端吊销其他会话并轮换当前会话。
        await changeOwnPassword({ oldPassword, newPassword });
      } else {
        // 管理员重置：口令走独立权限点；reason 仅写入审计，不回显。
        await setUserPassword(userId, newPassword, reason.trim() || undefined, organizationId);
        message.success(t("resetPasswordSuccess", "Password reset"));
      }
      setConfirmOpen(false);
      setOpen(false);
      setOldPassword("");
      setNewPassword("");
      setRePassword("");
      setReason("");
      onSuccess?.();
    } catch (error) {
      setConfirmOpen(false);
      const resolved = resolveSubmitError(error);
      setSubmitError(resolved.message);
      setSubmitFailures(resolved.failures);
    } finally {
      setConfirmLoading(false);
    }
  }, [
    mode,
    userId,
    organizationId,
    newPassword,
    oldPassword,
    reason,
    onSuccess,
    resolveSubmitError,
    message,
    t,
  ]);

  const handleOk = useCallback(() => {
    if (newPassword === "" || rePassword === "") return;
    if (newPassword !== rePassword) return;
    if (newPasswordError) return;
    if (requireOldPassword && oldPassword === "") return;

    setSubmitError("");
    // 自助改密只有主弹窗一个确认步骤；二次确认弹窗仅管理员重置使用
    // （它会吊销目标用户的全部会话与设备）。此前自助模式也走 setConfirmOpen，
    // 但确认弹窗不渲染，导致 /account 的改密按钮永远不发请求。
    if (requireOldPassword) {
      void handleConfirm();
      return;
    }
    setConfirmOpen(true);
  }, [newPassword, rePassword, newPasswordError, requireOldPassword, oldPassword, handleConfirm]);

  const title =
    mode === "self"
      ? t("changePassword", "Change Password")
      : t("auth:resetPassword", "Reset Password");

  return (
    <>
      <Button type="primary" disabled={disabled} onClick={showModal}>
        {title}
      </Button>
      <Modal
        title={title}
        open={open}
        okText={t("confirm", "Confirm")}
        cancelText={t("cancel", "Cancel")}
        confirmLoading={confirmLoading}
        onCancel={handleCancel}
        onOk={handleOk}
        width={500}
      >
        <Space orientation="vertical" style={{ width: "100%" }} size="middle">
          {requireOldPassword && (
            <div>
              <Typography.Text strong style={{ display: "block", marginBottom: 4 }}>
                {t("oldPassword", "Old Password")}
              </Typography.Text>
              <Input.Password
                placeholder={t("placeholder.oldPassword", "Enter old password")}
                value={oldPassword}
                onChange={(e) => setOldPassword(e.target.value)}
              />
            </div>
          )}
          {mode === "admin" && (
            <div>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {t("passwordPolicyRequirements", "Organization policy requirements")}
              </Typography.Text>
              <ul style={{ margin: "4px 0 0", paddingInlineStart: 20, fontSize: 12 }}>
                {rules.map((rule) => (
                  <li key={rule.key}>{rule.label}</li>
                ))}
              </ul>
            </div>
          )}
          <div>
            <Typography.Text strong style={{ display: "block", marginBottom: 4 }}>
              {t("auth:newPassword", "New Password")}
            </Typography.Text>
            <Popover
              placement="right"
              open={popoverOpen && newPassword.length > 0}
              content={<PasswordChecker password={newPassword} rules={rules} />}
            >
              <Input.Password
                placeholder={t("placeholder.newPassword", "Enter new password")}
                value={newPassword}
                onChange={(e) => {
                  handleNewPasswordChange(e.target.value);
                  setPopoverOpen(true);
                }}
                onFocus={() => setPopoverOpen(true)}
                onBlur={() => setPopoverOpen(false)}
                status={newPasswordError ? "error" : undefined}
              />
            </Popover>
          </div>
          {newPasswordError && (
            <Typography.Text type="danger" style={{ fontSize: 12 }}>
              {newPasswordError}
            </Typography.Text>
          )}
          <div>
            <Typography.Text strong style={{ display: "block", marginBottom: 4 }}>
              {t("auth:confirmPassword", "Confirm Password")}
            </Typography.Text>
            <Input.Password
              placeholder={t("placeholder.confirmPassword", "Re-enter new password")}
              value={rePassword}
              onChange={(e) => handleRePasswordChange(e.target.value)}
              status={rePasswordError ? "error" : undefined}
            />
          </div>
          {rePasswordError && (
            <Typography.Text type="danger" style={{ fontSize: 12 }}>
              {rePasswordError}
            </Typography.Text>
          )}
          {mode === "admin" && (
            <div>
              <Typography.Text strong style={{ display: "block", marginBottom: 4 }}>
                {t("resetPasswordReason", "Reason (optional)")}
              </Typography.Text>
              <Input
                value={reason}
                maxLength={128}
                placeholder={t("resetPasswordReasonPlaceholder", "Recorded in the audit log only")}
                onChange={(e) => setReason(e.target.value)}
              />
            </div>
          )}
          {submitFailures.length > 0 ? (
            <ul style={{ margin: 0, paddingInlineStart: 18 }}>
              {submitFailures.map((failure, index) => (
                <li key={`${index}-${failure}`}>
                  <Typography.Text type="danger" style={{ fontSize: 12 }}>
                    {failure}
                  </Typography.Text>
                </li>
              ))}
            </ul>
          ) : (
            submitError && (
              <Typography.Text type="danger" style={{ fontSize: 12 }}>
                {submitError}
              </Typography.Text>
            )
          )}
        </Space>
      </Modal>
      {mode === "admin" && (
        <Modal
          title={t("resetPasswordConfirmTitle", "Confirm password reset")}
          open={confirmOpen}
          okText={t("confirm", "Confirm")}
          cancelText={t("cancel", "Cancel")}
          confirmLoading={confirmLoading}
          onCancel={() => setConfirmOpen(false)}
          onOk={handleConfirm}
          width={460}
        >
          <Typography.Text>
            {t("resetPasswordConfirmContent", "All sessions and devices will be revoked.")}
          </Typography.Text>
        </Modal>
      )}
    </>
  );
};
