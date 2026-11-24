// oxlint-disable vitest/require-mock-type-parameters -- 测试替身按最小形状 mock，泛型参数会迫使测试引入实现细节
import { describe, it, expect, vi, beforeEach, beforeAll } from "vitest";
import { fireEvent, render, waitFor, within } from "@testing-library/react";
import { FormSection, Web3LoginPanel } from "./Web3LoginPanel";
import { passkeyErrorMessage } from "./passkey-error";
import { HttpError } from "@/shared/api/http";
import type { TFunction } from "i18next";

// Mock antd icons（jsdom 环境下 @ant-design/icons 是 undefined）
vi.mock("@ant-design/icons", () => ({
  VerifiedOutlined: () => <span data-testid="icon-verified" />,
  UserOutlined: () => <span data-testid="icon-user" />,
  ThunderboltOutlined: () => <span data-testid="icon-thunder" />,
  CheckCircleOutlined: () => <span data-testid="icon-check" />,
  SwapOutlined: () => <span data-testid="icon-swap" />,
  InfoCircleOutlined: () => <span data-testid="icon-info" />,
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (_key: string, fallback?: string) => fallback ?? "label" }),
}));

vi.mock("@/shared/api/iam/webauthn", () => ({
  webauthnApi: {
    registrationBegin: vi.fn().mockResolvedValue({
      challenge: "Y2hhbGxlbmdl",
      rpId: "e.com",
      rpName: "E",
      userId: "dXNlci0x",
      challengeId: "id",
      userDisplayName: "U",
      timeout: 60000,
      userVerification: "required",
    }),
    registrationFinish: vi.fn().mockResolvedValue({ status: "ok", credentialId: "abc" }),
    authenticationBegin: vi.fn().mockResolvedValue({
      challenge: "c",
      rpId: "e.com",
      challengeId: "id",
      timeout: 30000,
    }),
    authenticationFinish: vi.fn().mockResolvedValue({ status: "ok" }),
  },
}));

const createCredentialMock = vi.fn();
const getCredentialMock = vi.fn();

beforeAll(() => {
  if (typeof window !== "undefined") {
    Object.defineProperty(window, "PublicKeyCredential", {
      value: {
        isUserVerifyingPlatformAuthenticatorAvailable: async () => true,
      },
      configurable: true,
    });
    createCredentialMock.mockRejectedValue(new DOMException("cancelled", "NotAllowedError"));
    getCredentialMock.mockRejectedValue(new DOMException("cancelled", "NotAllowedError"));
    Object.defineProperty(navigator, "credentials", {
      value: {
        create: createCredentialMock,
        get: getCredentialMock,
      },
      configurable: true,
    });
  }
});

describe("Web3LoginPanel smoke", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders mode selector by default", () => {
    const { container } = render(<Web3LoginPanel organizationSlug="built-in" />);
    expect(container.firstChild).toBeTruthy();
  });

  it("renders form when initialMode=login", () => {
    const { container } = render(
      <Web3LoginPanel organizationSlug="built-in" initialMode="login" />,
    );
    expect(container.firstChild).toBeTruthy();
  });
});

describe("Web3LoginPanel registration UV policy", () => {
  beforeEach(() => vi.clearAllMocks());

  it("passes the userVerification value from registration:begin to navigator.credentials.create", async () => {
    const { container } = render(
      <Web3LoginPanel organizationSlug="built-in" initialMode="register" />,
    );

    fireEvent.change(within(container).getByLabelText("用户名"), {
      target: { value: "alice" },
    });
    fireEvent.click(within(container).getByRole("button", { name: "创建 Passkey" }));

    await waitFor(() => expect(createCredentialMock).toHaveBeenCalledTimes(1));
    const options = createCredentialMock.mock.calls[0][0] as {
      publicKey: PublicKeyCredentialCreationOptions;
    };
    // 若前端仍硬编码 preferred，这条断言会失败。
    expect(options.publicKey.authenticatorSelection?.userVerification).toBe("required");
  });
});

describe("Web3LoginPanel form section accessibility", () => {
  const t = ((_key: string, fallback?: string) =>
    fallback ?? "label") as unknown as TFunction<"auth">;
  const baseProps = {
    form: { username: "", mode: "login" as const },
    submitting: false,
    hideRegister: false,
    t,
    onUsernameChange: () => {},
    onModeChange: () => {},
    onBack: () => {},
    onSubmit: () => {},
  };

  it("names the username input and spins the wrapper while loading", () => {
    const { container: idleContainer } = render(<FormSection {...baseProps} loading={false} />);
    expect(within(idleContainer).getByLabelText("用户名")).toBeTruthy();
    expect(idleContainer.querySelector(".ant-spin-spinning")).toBeNull();

    const { container: loadingContainer } = render(<FormSection {...baseProps} loading={true} />);
    expect(loadingContainer.querySelector(".ant-spin-spinning")).not.toBeNull();
  });
});

describe("passkey error mapping", () => {
  const t = ((key: string) => key) as unknown as TFunction;

  it("maps backend details.reason to a passkey-specific message", () => {
    const err = new HttpError(422, {});
    err.details = { reason: "signature_invalid" };
    expect(passkeyErrorMessage(err, "login", t)).toBe("auth:passkey.reason.signature_invalid");
  });

  it("parses reason from the protocol-face error_description", () => {
    const err = new HttpError(422, {
      error: "invalid_request",
      error_description:
        "WebAuthn assertion verification failed: origin not allowed [reason: origin_not_allowed]",
    });
    expect(passkeyErrorMessage(err, "login", t)).toBe("auth:passkey.reason.origin_not_allowed");
  });

  it("never falls back to the generic form-validation copy on 422", () => {
    const err = new HttpError(422, {});
    expect(passkeyErrorMessage(err, "login", t)).toBe("auth:passkey.authenticationFailed");
  });

  it("maps 410 to the expired-challenge message", () => {
    const err = new HttpError(410, {});
    expect(passkeyErrorMessage(err, "login", t)).toBe("auth:passkey.reason.challenge_expired");
  });
});
