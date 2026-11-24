import { useEffect, useState } from "react";
import { Button, Card, Input, Space, Alert, Steps, Typography, Badge, Tooltip, Spin } from "antd";
import {
  VerifiedOutlined,
  UserOutlined,
  ThunderboltOutlined,
  CheckCircleOutlined,
  SwapOutlined,
  InfoCircleOutlined,
} from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { webauthnApi } from "@/shared/api/iam/webauthn";
import { passkeyErrorMessage } from "./passkey-error";
import type { PasskeyMode } from "./passkey-error";
import {
  base64UrlToArrayBuffer,
  credentialToAuthenticationFinishRequest,
  credentialToRegistrationFinishRequest,
  hasPlatformAuthenticator,
  isWebAuthnSupported,
} from "@/shared/lib/webauthn-b64";
import { toast } from "@/shared/ui/GlForm";

/**
 * Web3LoginPanel — Passkey 登录/注册面板。
 *
 * 替代设计：旧 @web3-onboard/* wallet connect 登录面板（已删除）。
 *
 * 核心特性：
 *   1. 浏览器原生 WebAuthn API（navigator.credentials.create/get）
 *   2. 渐进揭示 — 先选模式（登录/注册）→ 再填表 → 再点按钮 → 浏览器弹出安全密钥
 *   3. 后端对接：/webauthn/registration/begin|finish + /webauthn/authentication/begin|finish
 *   4. 渐进增强：不支持 WebAuthn 的浏览器显示降级提示
 *
 * 为什么不是 Casdoor 的 Web3Login？
 *   - Casdoor 的 Web3LoginPanel 走的是 wallet connect → 需额外依赖 + 钱包安装
 *   - 本项目走的是 Passkey（WebAuthn）→ 浏览器原生，零依赖，更好的用户体验
 *   - 后端已完整实现 WebAuthn 4 个 endpoint + Layer 2 地址自动推导
 *
 * 使用方式：LoginSigninItems.tsx 里作为一个 option（key="passkey"）渲染。
 *
 * @param organizationSlug 当前组织 slug（从路由上下文取）
 * @param mode initial: 先展示模式选择 | 直接进入指定模式
 * @param onAuthenticated 认证成功回调（可选）
 */

export type { PasskeyMode } from "./passkey-error";

export interface Web3LoginPanelProps {
  organizationSlug: string;
  initialMode?: PasskeyMode;
  onAuthenticated?: () => void;
  /** 自定义标题 — 默认 "Passkey 登录" */
  title?: string;
  /** 是否隐藏注册入口（只允许已有 Passkey 用户登录） */
  hideRegister?: boolean;
  /**
   * 交给登录表单统一提交时使用：只校验用户名，然后把值写回表单并触发
   * 主提交链路（协议上下文 / applicationId / 会话落地都在表单侧完成）。
   * 未提供时保留面板原有的 begin→assertion→finish 自循环。
   */
  onSubmit?: (username: string) => void;
  /** 表单提交中（与 onSubmit 配合时用于禁用按钮） */
  submitting?: boolean;
}

// 状态机
type Step = "select-mode" | "fill-form" | "await-webauthn" | "complete";

interface FormState {
  username: string;
  mode: PasskeyMode;
}

export const Web3LoginPanel: React.FC<Web3LoginPanelProps> = ({
  organizationSlug,
  initialMode,
  onAuthenticated,
  hideRegister = false,
  onSubmit,
  submitting = false,
}) => {
  const { t } = useTranslation("auth");
  const [step, setStep] = useState<Step>(initialMode ? "fill-form" : "select-mode");
  const [form, setForm] = useState<FormState>({
    username: "",
    mode: initialMode ?? "login",
  });
  const [loading, setLoading] = useState(false);
  const [supported, setSupported] = useState(true);
  const [hasPlatformAuth, setHasPlatformAuth] = useState(false);

  useEffect(() => {
    // oxlint-disable-next-line react/set-state-in-effect -- 浏览器能力探测是外部系统同步点，非渲染派生
    setSupported(isWebAuthnSupported());
    hasPlatformAuthenticator().then(setHasPlatformAuth);
  }, []);

  // ────────────────────────────────────────────────────────────────────
  // 主流程
  // ────────────────────────────────────────────────────────────────────

  const handleSubmit = async () => {
    if (!form.username.trim()) {
      toast.error(t("passkey.enterUsername", "请输入用户名"));
      return;
    }
    if (!supported) {
      toast.error(t("passkey.notSupported", "当前浏览器不支持 Passkey"));
      return;
    }

    // 登录页统一提交：把用户名交给表单，由表单的 loginMethod==="webauthn" 分支
    // 走 begin→assertion→finish 与协议重定向，避免面板自循环丢失 OAuth/SAML 上下文。
    if (onSubmit) {
      onSubmit(form.username.trim());
      return;
    }

    setLoading(true);
    try {
      if (form.mode === "register") {
        await runRegistration();
      } else {
        await runAuthentication();
      }
    } catch (err) {
      toast.error(passkeyErrorMessage(err, form.mode, t));
    } finally {
      setLoading(false);
    }
  };

  // ────────────────────────────────────────────────────────────────────
  // 注册流程
  // ────────────────────────────────────────────────────────────────────

  async function runRegistration() {
    setStep("await-webauthn");

    const begin = await webauthnApi.registrationBegin({
      organizationSlug,
      username: form.username,
    });

    if (!begin) {
      throw new Error(t("passkey.unknownError", "未知错误"));
    }

    // 转浏览器 PublicKeyCredentialCreationOptions
    const publicKey: PublicKeyCredentialCreationOptions = {
      rp: { id: begin.rpId, name: begin.rpName },
      user: {
        id: base64UrlToArrayBuffer(begin.userId),
        name: form.username,
        displayName: begin.userDisplayName || form.username,
      },
      challenge: base64UrlToArrayBuffer(begin.challenge),
      pubKeyCredParams: [
        // 后端 COSE 解析只支持 EC2/P-256/ES256；广告 -257(RS256) 会让
        // 只支持 RS256 的认证器注册成功一半后被 finish 拒绝，故不广告。
        { type: "public-key", alg: -7 }, // ES256
      ],
      timeout: begin.timeout || 60_000,
      attestation: "none",
      authenticatorSelection: {
        residentKey: "preferred",
        requireResidentKey: false,
        // 由 begin 下发：组织/部署策略要求 UV 时必须是 "required"，不能再硬编码。
        userVerification: begin.userVerification,
      },
      // 由 begin 下发：该用户已有凭据。浏览器在客户端直接拒绝同一认证器重复注册。
      ...(begin.excludeCredentials?.length
        ? {
            excludeCredentials: begin.excludeCredentials.map((id) => ({
              type: "public-key" as const,
              id: base64UrlToArrayBuffer(id),
            })),
          }
        : {}),
    };

    // 浏览器原生 API
    const credential = (await navigator.credentials.create({
      publicKey,
    })) as PublicKeyCredential | null;
    if (!credential) {
      throw new Error(t("passkey.registrationCancelled", "注册已取消"));
    }

    // 提交 finish
    const finishPayload = credentialToRegistrationFinishRequest(
      credential,
      begin.challengeId,
      begin.challenge,
      organizationSlug,
      form.username,
    );
    const finish = await webauthnApi.registrationFinish(finishPayload);
    if (!finish) {
      throw new Error(t("passkey.registrationFailed", "Passkey 注册失败"));
    }

    // 注册 finish 的成功标记是 "registered"；兼容旧契约的 "ok"。
    if (finish.status === "registered" || finish.status === "ok") {
      toast.success(t("passkey.registrationSuccess", "Passkey 注册成功！"));
      setStep("complete");
      onAuthenticated?.();
    } else {
      throw new Error(t("passkey.registrationFailed", "Passkey 注册失败"));
    }
  }

  // ────────────────────────────────────────────────────────────────────
  // 登录流程
  // ────────────────────────────────────────────────────────────────────

  async function runAuthentication() {
    setStep("await-webauthn");

    const begin = await webauthnApi.authenticationBegin({
      organizationSlug,
      username: form.username,
    });

    if (!begin) {
      throw new Error(t("passkey.unknownError", "未知错误"));
    }

    const allowCredentials = begin.allowedCredentialIds?.length
      ? begin.allowedCredentialIds.map((id) => ({
          type: "public-key" as const,
          id: base64UrlToArrayBuffer(id),
        }))
      : undefined;

    const publicKey: PublicKeyCredentialRequestOptions = {
      rpId: begin.rpId,
      challenge: base64UrlToArrayBuffer(begin.challenge),
      timeout: begin.timeout || 60_000,
      // 由 begin 下发：组织策略要求 UV 时必须是 "required"，不能再硬编码 "preferred"。
      userVerification: begin.userVerification ?? "preferred",
      ...(allowCredentials ? { allowCredentials } : {}),
    };

    const credential = (await navigator.credentials.get({
      publicKey,
    })) as PublicKeyCredential | null;
    if (!credential) {
      throw new Error(t("passkey.authenticationCancelled", "登录已取消"));
    }

    const finishPayload = credentialToAuthenticationFinishRequest(
      credential,
      begin.challengeId,
      begin.challenge,
      organizationSlug,
      form.username,
    );
    const finish = await webauthnApi.authenticationFinish(finishPayload);
    if (!finish) {
      throw new Error(t("passkey.authenticationFailed", "登录失败"));
    }

    if (finish.status) {
      toast.success(t("passkey.authenticationSuccess", "登录成功！"));
      setStep("complete");
      onAuthenticated?.();
    } else {
      throw new Error(t("passkey.authenticationFailed", "登录失败"));
    }
  }

  // ────────────────────────────────────────────────────────────────────
  // 渲染
  // ────────────────────────────────────────────────────────────────────

  return (
    <Card className="gl-passkey-panel" size="small" variant="borderless">
      {/* 标题 + 状态 badge */}
      <div className="gl-passkey-panel__header">
        <Space align="center">
          <Badge
            status={supported ? "success" : "error"}
            text={
              supported
                ? t("passkey.supported", "Passkey 可用")
                : t("passkey.unsupported", "浏览器不支持")
            }
          />
          {hasPlatformAuth && (
            <Tooltip
              title={t("passkey.platformAuthHint", "Windows Hello / Face ID / Touch ID 可用")}
            >
              <ThunderboltOutlined style={{ color: "var(--gl-color-warning)" }} />
            </Tooltip>
          )}
        </Space>
      </div>

      {!supported && (
        <Alert
          type="error"
          title={t("passkey.notSupportedTitle", "浏览器不支持 Passkey")}
          description={t(
            "passkey.notSupportedDesc",
            "请使用最新版 Chrome / Edge / Firefox / Safari，或改用密码登录。",
          )}
          showIcon
          icon={<InfoCircleOutlined />}
          style={{ marginBottom: 12 }}
        />
      )}

      {step === "select-mode" && !hideRegister && (
        <ModeSelector
          onSelect={(mode) => {
            setForm({ ...form, mode });
            setStep("fill-form");
          }}
        />
      )}

      {step === "fill-form" && (
        <FormSection
          form={form}
          loading={loading}
          submitting={submitting}
          hideRegister={hideRegister}
          t={t}
          onUsernameChange={(username) => setForm({ ...form, username })}
          onModeChange={(mode) => setForm({ ...form, mode })}
          onBack={() => setStep("select-mode")}
          onSubmit={handleSubmit}
        />
      )}

      {step === "await-webauthn" && <AwaitWebAuthn mode={form.mode} t={t} />}

      {step === "complete" && <CompleteSuccess mode={form.mode} t={t} />}

      {/* 渐进揭示提示 */}
      <Typography.Text type="secondary" style={{ fontSize: 12, display: "block", marginTop: 12 }}>
        {t("passkey.hint", "Passkey 使用 WebAuthn 标准，私钥永远不会离开你的设备。")}
      </Typography.Text>
    </Card>
  );
};

// ──────────────────────────────────────────────────────────────────────────
// 子组件
// ──────────────────────────────────────────────────────────────────────────

interface ModeSelectorProps {
  onSelect: (mode: PasskeyMode) => void;
}

function ModeSelector({ onSelect }: ModeSelectorProps) {
  const { t } = useTranslation("auth");
  return (
    <Space orientation="vertical" size="middle" style={{ width: "100%" }}>
      <Typography.Text strong>{t("passkey.chooseMode", "选择 Passkey 模式")}</Typography.Text>

      <Button
        block
        size="large"
        icon={<VerifiedOutlined />}
        type="primary"
        onClick={() => onSelect("login")}
      >
        {t("passkey.loginExisting", "用 Passkey 登录")}
        <div style={{ fontSize: 12, opacity: 0.75 }}>
          {t("passkey.loginExistingHint", "已有 Passkey 设备，快速登录")}
        </div>
      </Button>

      <Button block size="large" icon={<UserOutlined />} onClick={() => onSelect("register")}>
        {t("passkey.registerNew", "注册新 Passkey")}
        <div style={{ fontSize: 12, opacity: 0.75 }}>
          {t("passkey.registerNewHint", "首次使用，为你当前设备创建 Passkey")}
        </div>
      </Button>
    </Space>
  );
}

interface FormSectionProps {
  form: FormState;
  loading: boolean;
  submitting: boolean;
  hideRegister: boolean;
  t: TFunction<"auth">;
  onUsernameChange: (v: string) => void;
  onModeChange: (m: PasskeyMode) => void;
  onBack: () => void;
  onSubmit: () => void;
}

export function FormSection({
  form,
  loading,
  submitting,
  hideRegister,
  t,
  onUsernameChange,
  onModeChange,
  onBack,
  onSubmit,
}: FormSectionProps) {
  const busy = loading || submitting;
  return (
    <Spin spinning={loading}>
      <Space orientation="vertical" size="middle" style={{ width: "100%" }}>
        <Input
          size="large"
          prefix={<UserOutlined />}
          aria-label={t("username", "用户名")}
          placeholder={t("passkey.usernamePlaceholder", "输入用户名或邮箱")}
          value={form.username}
          onChange={(e) => onUsernameChange(e.target.value)}
          onPressEnter={onSubmit}
          disabled={busy}
        />

        {!hideRegister && (
          <Button
            type="text"
            size="small"
            icon={<SwapOutlined />}
            onClick={() => onModeChange(form.mode === "login" ? "register" : "login")}
            style={{ padding: 0 }}
          >
            {form.mode === "login"
              ? t("passkey.switchToRegister", "没有 Passkey？注册一个")
              : t("passkey.switchToLogin", "已有 Passkey？去登录")}
          </Button>
        )}

        <Space style={{ width: "100%" }}>
          {!hideRegister && (
            <Button onClick={onBack} disabled={busy}>
              {t("passkey.back", "返回")}
            </Button>
          )}
          <Button
            type="primary"
            size="large"
            icon={<VerifiedOutlined />}
            onClick={onSubmit}
            loading={busy}
            disabled={busy}
            style={{ flex: 1 }}
          >
            {form.mode === "login"
              ? t("passkey.login", "使用 Passkey 登录")
              : t("passkey.register", "创建 Passkey")}
          </Button>
        </Space>
      </Space>
    </Spin>
  );
}

interface AwaitWebAuthnProps {
  mode: PasskeyMode;
  t: TFunction<"auth">;
}

function AwaitWebAuthn({ mode, t }: AwaitWebAuthnProps) {
  const steps = [
    { title: t("passkey.stepChallenge", "获取挑战") },
    { title: t("passkey.stepBrowserPrompt", "浏览器弹出安全密钥") },
    { title: t("passkey.stepDeviceConfirm", "在设备上确认") },
    { title: t("passkey.stepVerify", "后端验证") },
  ];

  return (
    <div style={{ padding: "20px 0", textAlign: "center" }}>
      <Spin size="large">
        <Steps
          orientation="vertical"
          size="small"
          current={1}
          items={steps.map((s) => ({ title: s.title }))}
        />
      </Spin>

      <Alert
        type="info"
        showIcon
        title={
          mode === "login"
            ? t("passkey.waitingLogin", "请在设备上确认 Passkey 登录请求")
            : t("passkey.waitingRegister", "请在设备上确认创建新 Passkey")
        }
        style={{ marginTop: 16, textAlign: "left" }}
      />
    </div>
  );
}

interface CompleteSuccessProps {
  mode: PasskeyMode;
  t: TFunction<"auth">;
}

function CompleteSuccess({ mode, t }: CompleteSuccessProps) {
  return (
    <div style={{ padding: "20px 0", textAlign: "center" }}>
      <CheckCircleOutlined style={{ fontSize: 48, color: "var(--gl-color-success)" }} />
      <Typography.Title level={5} style={{ marginTop: 12 }}>
        {mode === "login"
          ? t("passkey.loginComplete", "登录成功")
          : t("passkey.registerComplete", "Passkey 注册完成")}
      </Typography.Title>
      <Typography.Text type="secondary">
        {mode === "login"
          ? t("passkey.loginCompleteDesc", "你已通过 Passkey 认证")
          : t("passkey.registerCompleteDesc", "下次登录直接用这个 Passkey")}
      </Typography.Text>
    </div>
  );
}
