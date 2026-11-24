import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ApplicationItem } from "@/shared/types/domain";
import { useBiometricSigninActions } from "./useBiometricSigninActions";

const { mockRunWebAuthnLogin, mockRunFaceIdLogin } = vi.hoisted(() => ({
  mockRunWebAuthnLogin: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockRunFaceIdLogin: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
}));

vi.mock("@/shared/lib/auth-signin-providers", () => ({
  runWebAuthnLogin: (...args: unknown[]) => mockRunWebAuthnLogin(...args),
  runFaceIdLogin: (...args: unknown[]) => mockRunFaceIdLogin(...args),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

type TestValues = {
  organization: string;
  application: string;
  username: string;
  captchaToken?: string;
};

type TestPayload = {
  organization: string;
  application: string;
  username: string;
  applicationId?: string;
  signinMethod?: string;
  type?: string;
};

function makeApplication(overrides: Record<string, unknown> = {}) {
  return {
    id: "app-1",
    organization: "org-app",
    owner: "admin",
    ...overrides,
  } as unknown as ApplicationItem;
}

function setup(application: ApplicationItem | null = null) {
  const onSuccess = vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined);
  const createProtocolContextPayload = vi.fn<() => { type: string }>(() => ({
    type: "login",
  }));

  const view = renderHook(() =>
    useBiometricSigninActions<TestValues, TestPayload>({
      application,
      oauthContext: null,
      isCasLogin: false,
      isSamlLogin: false,
      hasFaceIdProvider: false,
      requestFaceRecognition: vi.fn<() => Promise<Record<string, unknown>>>(),
      createProtocolContextPayload,
      createCaptchaPayload: () => ({}),
      onSuccess,
    }),
  );

  return { view, onSuccess };
}

type RunWebAuthnLoginArgs = {
  owner: string;
  username: string;
  createPayload: (username: string) => TestPayload;
  onSuccess: unknown;
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe("useBiometricSigninActions WebAuthn", () => {
  it("uses the application organization and maps the built-in admin payload", async () => {
    mockRunWebAuthnLogin.mockResolvedValue({});
    const { view, onSuccess } = setup(makeApplication({ organization: "built-in" }));

    await act(async () => {
      await view.result.current.signInWithWebAuthn({
        organization: "form-org",
        application: "console",
        username: "  admin  ",
      });
    });

    expect(mockRunWebAuthnLogin).toHaveBeenCalledTimes(1);
    const args = mockRunWebAuthnLogin.mock.calls[0][0] as RunWebAuthnLoginArgs;
    expect(args.owner).toBe("built-in");
    expect(args.username).toBe("admin");
    expect(args.onSuccess).toBe(onSuccess);
    expect(args.createPayload("admin")).toEqual({
      type: "login",
      signinMethod: "WebAuthn",
      organization: "form-org",
      username: "admin",
      password: "",
      application: "console",
      applicationId: "app-1",
    });
  });

  it("falls back to the application owner when the organization projection is missing", async () => {
    mockRunWebAuthnLogin.mockResolvedValue({});
    const { view } = setup(makeApplication({ organization: "" }));

    await act(async () => {
      await view.result.current.signInWithWebAuthn({
        organization: "form-org",
        application: "console",
        username: "admin",
      });
    });

    const args = mockRunWebAuthnLogin.mock.calls[0][0] as RunWebAuthnLoginArgs;
    expect(args.owner).toBe("admin");
  });

  it("falls back to the form organization without an application and omits applicationId", async () => {
    mockRunWebAuthnLogin.mockResolvedValue({});
    const { view } = setup(null);

    await act(async () => {
      await view.result.current.signInWithWebAuthn({
        organization: "built-in",
        application: "console",
        username: "admin",
      });
    });

    const args = mockRunWebAuthnLogin.mock.calls[0][0] as RunWebAuthnLoginArgs;
    expect(args.owner).toBe("built-in");
    expect(args.createPayload("admin").applicationId).toBeUndefined();
  });

  it("rejects before touching the network when no owner can be resolved", async () => {
    const { view } = setup(null);

    await act(async () => {
      await expect(
        view.result.current.signInWithWebAuthn({
          organization: "",
          application: "console",
          username: "admin",
        }),
      ).rejects.toThrow("biometric.missingOrganization");
    });

    expect(mockRunWebAuthnLogin).not.toHaveBeenCalled();
  });
});
