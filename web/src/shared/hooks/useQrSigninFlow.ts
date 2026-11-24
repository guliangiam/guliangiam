import { useCallback, useEffect, useRef, useState } from "react";
import { t } from "i18next";
import {
  createQrLoginSession,
  exchangeQrLogin,
  waitForQrLoginOutcome,
  type QrLoginResultDocument,
} from "@/shared/lib/auth-signin-providers";
import { localizedError } from "@/shared/lib/server-error";

export type QrSigninStatus =
  | "idle"
  | "loading"
  | "pending"
  | "scanned"
  | "confirmed"
  | "expired"
  | "failed";

export type QrSigninState = {
  status: QrSigninStatus;
  /** 二维码载荷（base64url 随机串），由渲染方自行编码成图形。 */
  code: string | null;
  /** 服务端给定的会话有效期（秒），用于倒计时。 */
  expiresIn: number | null;
  error: string | null;
};

const IDLE_QR_SIGNIN_STATE: QrSigninState = {
  status: "idle",
  code: null,
  expiresIn: null,
  error: null,
};

/**
 * 控制台侧的扫码登录流程：创建会话 → 可取消轮询 → 换取令牌（服务端同时种会话 cookie）。
 *
 * 与微信扫码时代不同，这里只打已声明契约（详见 auth-signin-providers 的 QR 段）：
 * 二维码渲染 `code`，轮询按服务端 `interval`，取消走 AbortSignal；切换登录方式、
 * 刷新二维码或组件卸载都会 abort 在途请求与定时器，过期会话的回包不会覆盖新会话。
 */
export function useQrSigninFlow(values: {
  active: boolean;
  organizationId?: string | null;
  applicationId?: string | null;
  onConfirmed: (loginResult: QrLoginResultDocument) => Promise<void>;
  createImpl?: typeof createQrLoginSession;
  waitImpl?: typeof waitForQrLoginOutcome;
  exchangeImpl?: typeof exchangeQrLogin;
}) {
  const { active, applicationId, onConfirmed, organizationId } = values;
  const createImpl = values.createImpl ?? createQrLoginSession;
  const waitImpl = values.waitImpl ?? waitForQrLoginOutcome;
  const exchangeImpl = values.exchangeImpl ?? exchangeQrLogin;
  const [state, setState] = useState<QrSigninState>(IDLE_QR_SIGNIN_STATE);
  const [runId, setRunId] = useState(0);
  const onConfirmedRef = useRef(onConfirmed);
  useEffect(() => {
    onConfirmedRef.current = onConfirmed;
  }, [onConfirmed]);

  const reloadQrSignin = useCallback(() => {
    setRunId((current) => current + 1);
  }, []);

  useEffect(() => {
    if (!active || !organizationId || !applicationId) {
      // 没有可跑的会话：返回的 qrSigninState 由下方派生表达式给 idle，这里不 setState
      // （effect 内同步 setState 会触发级联渲染）。
      return;
    }

    const controller = new AbortController();

    const run = async () => {
      setState({ status: "loading", code: null, expiresIn: null, error: null });
      try {
        const challenge = await createImpl({
          organizationId,
          applicationId,
          signal: controller.signal,
        });
        if (controller.signal.aborted) {
          return;
        }

        setState({
          status: "pending",
          code: challenge.code,
          expiresIn: challenge.expiresIn,
          error: null,
        });

        const outcome = await waitImpl({
          codeId: challenge.codeId,
          intervalSeconds: challenge.interval,
          timeoutSeconds: challenge.expiresIn,
          signal: controller.signal,
          onStatus: (status) => {
            if (controller.signal.aborted) {
              return;
            }
            setState((current) =>
              current.status === "confirmed"
                ? current
                : { ...current, status: status === "scanned" ? "scanned" : "pending" },
            );
          },
        });
        if (controller.signal.aborted) {
          return;
        }

        if (outcome.kind === "expired") {
          setState((current) => ({ ...current, status: "expired", error: outcome.message }));
          return;
        }

        const loginResult = await exchangeImpl({
          codeId: challenge.codeId,
          signal: controller.signal,
        });
        if (controller.signal.aborted) {
          return;
        }

        setState((current) => ({ ...current, status: "confirmed" }));
        await onConfirmedRef.current(loginResult);
      } catch (error) {
        if (controller.signal.aborted) {
          return;
        }
        if (error instanceof Error && error.name === "AbortError") {
          return;
        }
        setState({
          status: "failed",
          code: null,
          expiresIn: null,
          error: localizedError(error, t, "auth:signin.loginFailed"),
        });
      }
    };

    void run();
    return () => {
      controller.abort();
    };
    // oxlint-disable-next-line react/exhaustive-effect-dependencies -- runId 是「刷新二维码」的重跑键：reloadQrSignin 递增它重新建会话，effect 体内不读取它
  }, [active, applicationId, createImpl, exchangeImpl, organizationId, runId, waitImpl]);

  return {
    qrSigninState: active && organizationId && applicationId ? state : IDLE_QR_SIGNIN_STATE,
    reloadQrSignin,
  };
}
