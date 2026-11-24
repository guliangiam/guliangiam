import { t } from "i18next";
import { completeFaceIdLogin } from "@/shared/api/iam/auth-face-id";
import type { FaceIdCompleteParams, FaceIdCompleteResult } from "@/shared/api/iam/auth-face-id";
import { requestDocumentJson } from "@/shared/api/http";
import type { ResponseEnvelope } from "@/shared/api/http";
import {
  webAuthnAuthenticationBegin,
  webAuthnAuthenticationFinish,
  type WebAuthnAuthenticationFinishResult,
} from "@/shared/api/iam/auth-webauthn";
import type { DirectAuthProvider } from "@/shared/lib/provider-auth";
import type { ApplicationItem } from "@/shared/types/domain";

import {
  AUTH_FACEID,
  AUTH_FACEID_VERIFY,
  AUTH_QRCODE,
  AUTH_QRCODE_CONFIRM,
  AUTH_QRCODE_EXCHANGE,
  AUTH_QRCODE_SCAN,
  AUTH_QRCODE_STATUS,
} from "@/shared/api/paths";

/** Console login uses type="login" as a local sentinel, never an OAuth responseType. */
function protocolResponseType(type?: string): string | undefined {
  return type && type !== "login" ? type : undefined;
}

async function faceIdSigninBegin(values: { owner: string; name: string }) {
  // 后端契约：POST /api/v1/auth/faceid（InitiateFaceIDRequest）
  // 入参是 organizationSlug + username（不是 owner/name），
  // 响应 data 为 { sessionId, challenge, expiresIn }。
  return requestDocumentJson<{ sessionId: string; challenge: string; expiresIn: number }>(
    AUTH_FACEID,
    {
      method: "POST",
      body: JSON.stringify({ organizationSlug: values.owner, username: values.name }),
    },
  );
}

// 服务端活体校验：AUTH-059~062 的挑战动作（眨眼/转头/点头/张嘴）必须由后端基于
// 图像判定，前端特征向量无法证明活体，因此只支持 images 模式。
async function faceIdVerify(values: { sessionId: string; imageData: string }) {
  return requestDocumentJson<{ verified: boolean; livenessPassed: boolean }>(AUTH_FACEID_VERIFY, {
    method: "POST",
    body: JSON.stringify({ sessionId: values.sessionId, imageData: values.imageData }),
  });
}

// 会话在此签发：complete 返回登录结果文档（实现见 shared/api/iam/auth-face-id.ts，
// 避免契约字段在多处各写一遍）。
async function faceIdComplete(values: FaceIdCompleteParams): Promise<FaceIdCompleteResult> {
  return completeFaceIdLogin(values);
}

type ApplicationProviderListItem = NonNullable<ApplicationItem["providers"]>[number];
type ApplicationProviderItem = Extract<ApplicationProviderListItem, { provider?: unknown }>;
export type SigninProvider = DirectAuthProvider & {
  owner?: string | null;
  category?: string | null;
  displayName?: string | null;
};

export type FaceRecognitionModuleLoader = () => Promise<unknown>;

export type FaceRecognitionModalModuleLoader = () => Promise<{
  preloadFaceApiScript: () => Promise<unknown>;
}>;

type FaceRecognitionRequest = (mode: "vector" | "images") => Promise<{
  faceId?: number[];
  faceIdImage?: string[];
}>;

type PasswordSigninPayload = {
  username?: string;
  password?: string;
  applicationId?: string;
  organizationSlug?: string;
  [key: string]: unknown;
};

function isProviderConfigItem(
  providerItem: ApplicationProviderListItem,
): providerItem is ApplicationProviderItem {
  return typeof providerItem === "object" && providerItem !== null && "provider" in providerItem;
}

export function getFaceIdProvider(application: ApplicationItem | null) {
  const providerItem = (application?.providers ?? [])
    .filter(isProviderConfigItem)
    .find((item) => item.provider?.category === "FaceId");

  return (providerItem?.provider as SigninProvider | undefined) ?? null;
}

// ─────────────────────────────────────────────
// QR Code 登录（/api/v1/auth/qrcode 四步 + 可取消轮询）
// ─────────────────────────────────────────────
//
// 只打本后端已声明的自建扫码登录契约。v1 面返回裸文档，解析必须走
// requestDocumentJson；legacy 时代的 /api/get-qrcode 与
// /api/get-webhook-event 已随微信扫码 tab 一并删除。
//
// 字段名唯一事实源（逐字取自服务端结构体，勿凭记忆改写）：
//   internal/transport/http/api/v1/auth/request.go
//     CreateQRCodeRequest   {organizationId, applicationId}
//     ScanQRCodeRequest     {code, username?, displayName?, ipAddress?, userAgent?}
//     ConfirmQRCodeRequest  {code, approved}
//     ExchangeQRCodeRequest {codeId}
//   internal/app/auth/qrcode.go
//     CreateQRCodeResult    {codeId, code, expiresIn, interval}（TTL 5 分钟、轮询 3 秒）
//     PollQRCodeResult      {status, codeId, expiresIn?}；status ∈ pending|scanned|confirmed|expired
//     ExchangeAuthorizationCodeResponse {status, accessToken, refreshToken?, idToken?, tokenType, expiresIn, scope?}
//
// 载荷字段 code 是 base64url 随机串，服务端 **不返回 qrUrl**——渲染方需自行把它编码成
// 二维码图形，不能当图片地址或跳转链接使用。
//
// 可达性前置（internal/transport/http/api/v1/auth/authorization.go）：
//   create / scan / 轮询 / exchange 属登录握手，可匿名调用；只有 confirm 需要会话。
//   会话在 exchange 签发（服务端 Set-Cookie），匿名 PC 因此能完成整条登录。
//   scan/confirm 按二维码载荷 `code` 寻址，轮询/exchange 按 codeId；扫码者身份只来自
//   会话（共享白名单内的公开端点也会先经 SessionContextMiddleware）。

export type QrLoginChallenge = {
  codeId: string;
  /** 二维码载荷（base64url 随机串，不是 URL）。 */
  code: string;
  /** 会话剩余有效期（秒）。 */
  expiresIn: number;
  /** 服务端要求的轮询间隔（秒）。 */
  interval: number;
};

/** 服务端会话状态；exchanged 不会出现在轮询结果里（会话行在 exchange 时被消费）。 */
export type QrLoginStatus = "pending" | "scanned" | "confirmed" | "expired";

export type QrLoginPollResult = {
  status: QrLoginStatus;
  codeId: string;
  expiresIn?: number;
};

/**
 * exchange 的响应文档：与 /faceid/complete 同形的登录结果（status 为 success），
 * 协议上下文（OAuth2 授权页等）下还会带授权码 / 回调地址字段。会话本身由服务端
 * Set-Cookie 签发，这里拿到的是「已登录 + 协议产物」。
 */
export type QrLoginResultDocument = {
  status: string;
  accessToken?: string;
  idToken?: string;
  redirectTo?: string;
} & Record<string, unknown>;

export type QrLoginOutcome =
  | { kind: "confirmed"; codeId: string }
  | { kind: "expired"; codeId: string; message: string };

/** 第 1 步：创建扫码会话（公开端点）。返回需要渲染的二维码载荷与轮询参数。 */
export async function createQrLoginSession(values: {
  organizationId: string;
  applicationId?: string;
  signal?: AbortSignal;
}): Promise<QrLoginChallenge> {
  const challenge = await requestDocumentJson<QrLoginChallenge>(AUTH_QRCODE, {
    method: "POST",
    body: JSON.stringify({
      organizationId: values.organizationId,
      applicationId: values.applicationId ?? "",
    }),
    signal: values.signal,
  });

  if (!challenge?.codeId || !challenge.code) {
    throw new Error(t("auth:signin.loginFailed"));
  }

  return challenge;
}

/**
 * 第 2 步：扫码端登记（公开端点）——按二维码载荷 `code` 寻址，不按 codeId：
 * codeId 是发起端（PC）的交换句柄，让它随二维码外泄等于把会话交给任何看到二维码的人。
 *
 * 身份规则（服务端强制）：调用方已有会话（cookie 或 Bearer）时身份只取自会话，
 * body 里的 username 一律不读；匿名调用方只能自动注册**尚不存在**的账号，
 * 命中既有账号返回 403（OPEN-API-15）。
 * 匿名注册成功时服务端同时签发扫码端会话（Set-Cookie），返回 isNewUser=true。
 */
export type QrLoginScanResult = {
  status: string;
  isNewUser: boolean;
};

export async function scanQrLogin(values: {
  code: string;
  username?: string;
  displayName?: string;
  ipAddress?: string;
  userAgent?: string;
  signal?: AbortSignal;
}): Promise<QrLoginScanResult> {
  const result = await requestDocumentJson<QrLoginScanResult>(AUTH_QRCODE_SCAN, {
    method: "POST",
    body: JSON.stringify({
      code: values.code,
      ...(values.username ? { username: values.username } : {}),
      ...(values.displayName ? { displayName: values.displayName } : {}),
      ...(values.ipAddress ? { ipAddress: values.ipAddress } : {}),
      ...(values.userAgent ? { userAgent: values.userAgent } : {}),
    }),
    signal: values.signal,
  });

  return { status: result?.status ?? "ok", isNewUser: result?.isNewUser === true };
}

/**
 * 第 3 步：扫码端确认/拒绝（需要扫码端既有会话）。同样按二维码载荷 `code` 寻址。
 * 服务端只接受扫码绑定的那个人确认；拒绝会删除会话，PC 侧轮询随即得到 expired。
 */
export async function confirmQrLogin(values: {
  code: string;
  approved: boolean;
  signal?: AbortSignal;
}): Promise<void> {
  await requestDocumentJson(AUTH_QRCODE_CONFIRM, {
    method: "POST",
    body: JSON.stringify({ code: values.code, approved: values.approved }),
    signal: values.signal,
  });
}

/**
 * 第 4 步：以已确认的会话换取登录结果（匿名可用；会话一次性，重放即失效）。
 *
 * 服务端在成功响应上签发发起端会话 cookie 并返回登录结果（对齐 /faceid/complete）；
 * 协议上下文（responseType/redirectUri/state/nonce/scope）可选，授权页上的扫码登录
 * 靠它拿回授权码/回调地址。
 */
export async function exchangeQrLogin(values: {
  codeId: string;
  signal?: AbortSignal;
  responseType?: string;
  redirectUri?: string;
  state?: string;
  nonce?: string;
  scope?: string[];
}): Promise<QrLoginResultDocument> {
  const result = await requestDocumentJson<QrLoginResultDocument>(AUTH_QRCODE_EXCHANGE, {
    method: "POST",
    body: JSON.stringify({
      codeId: values.codeId,
      ...(values.responseType ? { responseType: values.responseType } : {}),
      ...(values.redirectUri ? { redirectUri: values.redirectUri } : {}),
      ...(values.state ? { state: values.state } : {}),
      ...(values.nonce ? { nonce: values.nonce } : {}),
      ...(values.scope?.length ? { scope: values.scope } : {}),
    }),
    signal: values.signal,
  });

  if (!result || typeof result.status !== "string") {
    throw new Error(t("auth:signin.loginFailed"));
  }

  return result;
}

/** 轮询单次状态（公开端点）。状态取值未知时按 pending 处理，避免渠道新增状态卡死登录。 */
export async function pollQrLoginStatus(values: {
  codeId: string;
  signal?: AbortSignal;
}): Promise<QrLoginPollResult> {
  const result = await requestDocumentJson<QrLoginPollResult>(
    `${AUTH_QRCODE_STATUS(values.codeId)}`,
    { signal: values.signal },
  );

  const status = result?.status;
  return {
    codeId: values.codeId,
    status:
      status === "scanned" || status === "confirmed" || status === "expired" ? status : "pending",
    ...(result?.expiresIn !== undefined ? { expiresIn: result.expiresIn } : {}),
  };
}

function createAbortError() {
  if (typeof DOMException !== "undefined") {
    return new DOMException("QR login polling aborted", "AbortError");
  }

  const error = new Error("QR login polling aborted");
  error.name = "AbortError";
  return error;
}

/** 可取消的等待：abort 时立即拒绝，且不留下悬挂的定时器。 */
function abortableDelay(ms: number, signal?: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    if (signal?.aborted) {
      reject(createAbortError());
      return;
    }

    function onAbort() {
      clearTimeout(timer);
      reject(createAbortError());
    }

    const timer = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, ms);

    signal?.addEventListener("abort", onAbort, { once: true });
  });
}

/**
 * 轮询直到会话被确认、过期或被取消。
 *
 * 取消走 AbortSignal：组件卸载 / 用户刷新二维码时 abort，循环立即停止并抛出 AbortError，
 * 不残留定时器。超时上限优先取创建时服务端给出的 expiresIn，缺省 5 分钟（与服务端
 * qrCodeTTL 一致）；到点按 expired 结束，本地兜底只覆盖服务端不再应答的场景。
 */
export async function waitForQrLoginOutcome(values: {
  codeId: string;
  intervalSeconds?: number;
  timeoutSeconds?: number;
  signal?: AbortSignal;
  pollImpl?: typeof pollQrLoginStatus;
  onStatus?: (status: QrLoginStatus) => void;
}): Promise<QrLoginOutcome> {
  const pollImpl = values.pollImpl ?? pollQrLoginStatus;
  const intervalMs = Math.max(1, values.intervalSeconds ?? 3) * 1000;
  const deadline = Date.now() + Math.max(1, values.timeoutSeconds ?? 300) * 1000;
  let lastStatus: QrLoginStatus | null = null;

  while (Date.now() < deadline) {
    const result = await pollImpl({ codeId: values.codeId, signal: values.signal });

    if (result.status === "confirmed") {
      return { kind: "confirmed", codeId: values.codeId };
    }

    if (result.status === "expired") {
      return {
        kind: "expired",
        codeId: values.codeId,
        message: t("auth:login.qrcode.expired"),
      };
    }

    if (result.status !== lastStatus) {
      lastStatus = result.status;
      values.onStatus?.(result.status);
    }

    await abortableDelay(intervalMs, values.signal);
  }

  return {
    kind: "expired",
    codeId: values.codeId,
    message: t("auth:login.qrcode.expired"),
  };
}

/**
 * 控制台侧的最小闭环：创建（含展示回调）→ 轮询 → 换取令牌。
 *
 * 扫码与确认由已登录的扫码端完成，不在控制台内；本函数覆盖 PC 侧四步中的
 * create / 展示 / 轮询 / exchange。exchange 需要调用方既有会话，因此调用方必须处理
 * 401（HttpError），不能假设匿名也能换到令牌。
 */
export async function runQrLogin(values: {
  organizationId: string;
  applicationId?: string;
  signal?: AbortSignal;
  onChallenge?: (challenge: QrLoginChallenge) => void;
  onStatus?: (status: QrLoginStatus) => void;
  pollImpl?: typeof pollQrLoginStatus;
}): Promise<
  | { status: "confirmed"; challenge: QrLoginChallenge; loginResult: QrLoginResultDocument }
  | { status: "expired"; challenge: QrLoginChallenge; message: string }
> {
  const challenge = await createQrLoginSession({
    organizationId: values.organizationId,
    applicationId: values.applicationId,
    signal: values.signal,
  });
  values.onChallenge?.(challenge);

  const outcome = await waitForQrLoginOutcome({
    codeId: challenge.codeId,
    intervalSeconds: challenge.interval,
    timeoutSeconds: challenge.expiresIn,
    signal: values.signal,
    pollImpl: values.pollImpl,
    onStatus: values.onStatus,
  });

  if (outcome.kind === "expired") {
    return { status: "expired", challenge, message: outcome.message };
  }

  const loginResult = await exchangeQrLogin({ codeId: challenge.codeId, signal: values.signal });
  return { status: "confirmed", challenge, loginResult };
}

function webAuthnBufferDecode(value: string) {
  let normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  while (normalized.length % 4) {
    normalized += "=";
  }
  return Uint8Array.from(atob(normalized), (character) => character.charCodeAt(0));
}

function webAuthnBufferEncode(value: ArrayBuffer | null) {
  if (!value) {
    return "";
  }

  return btoa(String.fromCharCode(...new Uint8Array(value)))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=/g, "");
}

async function requestWebAuthnAssertion(values: {
  options: Record<string, unknown>;
  username: string;
  credentialsContainer?: CredentialsContainer;
}) {
  if (
    !("publicKey" in values.options) ||
    !values.options.publicKey ||
    typeof values.options.publicKey !== "object"
  ) {
    throw new Error(t("auth:signin.webauthn.challengeInvalid"));
  }

  const credentialsContainer = values.credentialsContainer ?? navigator.credentials;
  if (!credentialsContainer?.get) {
    throw new Error(t("auth:signin.webauthn.browserNotSupported"));
  }

  const publicKey = values.options.publicKey as {
    challenge?: string;
    allowCredentials?: Array<{
      id: string;
      type?: PublicKeyCredentialType;
      transports?: AuthenticatorTransport[];
    }>;
  };

  if (!publicKey.challenge) {
    throw new Error(t("auth:signin.webauthn.challengeMissing"));
  }

  const requestOptions: PublicKeyCredentialRequestOptions = {
    ...publicKey,
    challenge: webAuthnBufferDecode(publicKey.challenge),
    allowCredentials: values.username
      ? (publicKey.allowCredentials ?? []).map((item) => ({
          id: webAuthnBufferDecode(item.id),
          type: item.type ?? "public-key",
          transports: item.transports,
        }))
      : publicKey.allowCredentials?.map((item) => ({
          id: webAuthnBufferDecode(item.id),
          type: item.type ?? "public-key",
          transports: item.transports,
        })),
  };

  return credentialsContainer.get({ publicKey: requestOptions });
}

/**
 * 控制台 WebAuthn（通行密钥 / 安全密钥）登录的断言闭环：begin → 系统弹窗 → finish。
 *
 * 打的是本后端 v1 面（POST /api/v1/auth/webauthn/authentication/{begin,finish}），
 * （responseType/redirectUri/state/nonce/scope/applicationId）与 Face ID 同一条路径：
 * 从 createPayload 产出的 payload 上读，而不是用 isCasLogin/isSamlLogin 拼查询串。
 * finish 的响应就是登录结果文档（会话由服务端在该响应上 Set-Cookie 签发）。
 */
export async function runWebAuthnSignin<TPayload>(values: {
  owner: string;
  username: string;
  createPayload: (username: string) => TPayload;
  credentialsContainer?: CredentialsContainer;
}): Promise<WebAuthnAuthenticationFinishResult> {
  const username = values.username.trim();
  const begin = await webAuthnAuthenticationBegin(values.owner, username);

  const credential = await requestWebAuthnAssertion({
    options: {
      publicKey: {
        challenge: begin.challenge,
        rpId: begin.rpId,
        timeout: begin.timeout,
        // 透传 begin 的 UV 策略（required/preferred），requestWebAuthnAssertion
        // 会整体 spread 这份 publicKey 交给 navigator.credentials.get。
        userVerification: begin.userVerification,
        allowCredentials: begin.allowedCredentialIds?.map((id) => ({
          id,
          type: "public-key" as const,
        })),
      },
    },
    username,
    credentialsContainer: values.credentialsContainer,
  });

  if (!credential) {
    throw new Error(t("auth:signin.webauthn.signinCancelled"));
  }

  const assertion = credential as PublicKeyCredential & {
    response: AuthenticatorAssertionResponse;
  };
  const payload = values.createPayload(username);
  const context = payload as unknown as {
    type?: string;
    redirectUri?: string;
    state?: string;
    nonce?: string;
    scope?: string | string[];
    applicationId?: string;
  };
  // 控制台登录的 payload 用 type="login" 标记登录方式，它不是 OAuth 协议
  // responseType；直接透传会被 finish 按 RFC 6749 拒绝（unsupported_response_type）。
  const responseType = protocolResponseType(context.type);

  const loginResult = await webAuthnAuthenticationFinish({
    organizationSlug: values.owner,
    username,
    applicationId: context.applicationId,
    challengeId: begin.challengeId,
    challenge: begin.challenge,
    // PublicKeyCredential.id 本身就是 rawId 的 base64url（服务端按 RawURLEncoding 解码）。
    credentialId: assertion.id,
    signature: webAuthnBufferEncode(assertion.response.signature),
    authenticatorData: webAuthnBufferEncode(assertion.response.authenticatorData),
    clientDataJSON: webAuthnBufferEncode(assertion.response.clientDataJSON),
    responseType,
    redirectUri: context.redirectUri,
    state: context.state,
    nonce: context.nonce,
    scope: Array.isArray(context.scope)
      ? context.scope
      : context.scope
        ? [context.scope]
        : undefined,
  });

  if (!loginResult || typeof loginResult.status !== "string") {
    throw new Error(t("auth:signin.webauthn.signinFailed"));
  }

  return loginResult;
}

export async function runWebAuthnLogin<TPayload>(values: {
  owner: string;
  username: string;
  createPayload: (username: string) => TPayload;
  onSuccess: (values: { response: ResponseEnvelope; payload: TPayload }) => Promise<void>;
  credentialsContainer?: CredentialsContainer;
}) {
  const username = values.username.trim();
  const loginResult = await runWebAuthnSignin({
    owner: values.owner,
    username,
    createPayload: values.createPayload,
    credentialsContainer: values.credentialsContainer,
  });

  // v1 面返回裸文档；各登录入口的后续协议处理消费统一信封，
  // 因此在此显式包成 {status:"ok", data}，而不是改动 onSuccess 契约。
  const response: ResponseEnvelope = { status: "ok", data: loginResult };
  // runWebAuthnSignin 已从 createPayload 的产物上读过协议上下文；payload 是表单
  // 字段的纯投影，这里再取一份同形状的对象交给 onSuccess。
  const payload = values.createPayload(username);

  await values.onSuccess({ response, payload });
}

export async function prepareFaceIdSignin(values: {
  owner: string;
  username: string;
  useImageModal: boolean;
  loadFaceRecognitionCommonModalModule: FaceRecognitionModuleLoader;
  loadFaceRecognitionModalModule: FaceRecognitionModalModuleLoader;
  faceIdSigninBeginImpl?: typeof faceIdSigninBegin;
}) {
  const faceIdSigninBeginImpl = values.faceIdSigninBeginImpl ?? faceIdSigninBegin;
  const beginResponse = await faceIdSigninBeginImpl({
    owner: values.owner,
    name: values.username,
  });

  if (!beginResponse?.sessionId) {
    throw new Error(t("auth:signin.faceId.initFailed"));
  }

  const beginData = beginResponse;

  if (values.useImageModal) {
    await values.loadFaceRecognitionCommonModalModule();
    return beginData.sessionId;
  }

  const module = await values.loadFaceRecognitionModalModule();
  await module.preloadFaceApiScript();

  return beginData.sessionId;
}

export async function runFaceIdLogin<TPayload extends PasswordSigninPayload>(values: {
  owner: string;
  username: string;
  useImageModal: boolean;
  loadFaceRecognitionCommonModalModule: FaceRecognitionModuleLoader;
  loadFaceRecognitionModalModule: FaceRecognitionModalModuleLoader;
  requestFaceRecognition: FaceRecognitionRequest;
  createPayload: (values: {
    username: string;
    faceId?: number[];
    faceIdImage?: string[];
  }) => TPayload;
  onSuccess: (values: { response: ResponseEnvelope; payload: TPayload }) => Promise<void>;
  faceIdSigninBeginImpl?: typeof faceIdSigninBegin;
  loginWithPasswordImpl?: (payload: PasswordSigninPayload) => Promise<unknown>;
}) {
  const username = values.username.trim();
  if (!username) {
    throw new Error(t("auth:signin.faceId.enterUsername"));
  }

  const sessionId = await prepareFaceIdSignin({
    owner: values.owner,
    username,
    useImageModal: values.useImageModal,
    loadFaceRecognitionCommonModalModule: values.loadFaceRecognitionCommonModalModule,
    loadFaceRecognitionModalModule: values.loadFaceRecognitionModalModule,
    faceIdSigninBeginImpl: values.faceIdSigninBeginImpl,
  });

  // 安全性最优：服务端活体校验要求图像，前端特征向量无法证明活体，
  // 因此固定使用 images 模式。原始图像仅用于本次校验，不落库。
  const recognitionResult = await values.requestFaceRecognition("images");
  const imageData = recognitionResult.faceIdImage?.[0];
  if (!imageData) {
    throw new Error(t("auth:signin.faceId.signinFailed"));
  }

  const verification = await faceIdVerify({ sessionId, imageData });
  if (!verification?.verified || !verification?.livenessPassed) {
    throw new Error(t("auth:signin.faceId.signinFailed"));
  }

  const payload = values.createPayload({
    username,
    faceId: recognitionResult.faceId,
    faceIdImage: recognitionResult.faceIdImage,
  });
  const context = payload as unknown as {
    type?: string;
    redirectUri?: string;
    state?: string;
    nonce?: string;
    scope?: string | string[];
    applicationId?: string;
  };

  // 控制台登录的 payload 用 type="login" 标记登录方式，它不是 OAuth 协议
  // responseType；直接透传会被 complete 按 RFC 6749 拒绝。
  const responseType = protocolResponseType(context.type);

  // 会话在 complete 签发；其 data 为 LoginResult，仍以统一信封交给 onSuccess，
  // 使各登录入口的后续处理（协议重定向/令牌落地）无需分叉。
  const loginResult = await faceIdComplete({
    sessionId,
    applicationId: context.applicationId,
    responseType,
    redirectUri: context.redirectUri,
    state: context.state,
    nonce: context.nonce,
    scope: Array.isArray(context.scope)
      ? context.scope
      : context.scope
        ? [context.scope]
        : undefined,
  });
  // v1 面返回裸文档；各登录入口的后续协议处理消费统一信封，
  // 因此在此显式包成 {status:"ok", data}，而不是改动 onSuccess 契约。
  const response: ResponseEnvelope = { status: "ok", data: loginResult };

  await values.onSuccess({ response, payload });
}
