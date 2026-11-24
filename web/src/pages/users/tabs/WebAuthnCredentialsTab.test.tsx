import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { createTestWrapper } from "@/test-utils/render";
import WebAuthnCredentialsTab from "./WebAuthnCredentialsTab";

const { mockList, mockBegin, mockFinish, mockCreate } = vi.hoisted(() => ({
  mockList: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockBegin: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockFinish: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockCreate: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("@/shared/ui/PermissionGuard", () => ({
  PermissionGuard: ({ children }: { children: ReactNode }) => children,
}));

// The ceremony body is what this test pins; keep the real base64url helpers.
vi.mock("@/shared/lib/webauthn-b64", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/lib/webauthn-b64")>();
  return { ...actual, isWebAuthnSupported: () => true };
});

vi.mock("@/shared/api/iam/user", () => ({
  listWebAuthnCredentials: (...args: unknown[]) => mockList(...args),
  beginWebAuthnCredentialRegistration: (...args: unknown[]) => mockBegin(...args),
  finishWebAuthnCredentialRegistration: (...args: unknown[]) => mockFinish(...args),
  deleteWebAuthnCredential: vi.fn<() => Promise<void>>(),
  deactivateWebAuthnCredential: vi.fn<() => Promise<void>>(),
  revokeWebAuthnCredential: vi.fn<() => Promise<void>>(),
  restoreWebAuthnCredential: vi.fn<() => Promise<void>>(),
  renameWebAuthnCredential: vi.fn<() => Promise<void>>(),
  getWebAuthnCredential: vi.fn<() => Promise<unknown>>(),
}));

function makeCreatedCredential() {
  return {
    id: "cred-1",
    type: "public-key",
    response: {
      clientDataJSON: Uint8Array.from([1, 2, 3]).buffer,
      attestationObject: Uint8Array.from([4, 5, 6]).buffer,
      getTransports: () => ["internal"],
    },
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockList.mockResolvedValue([]);
  mockBegin.mockResolvedValue({
    challenge: "Y2hhbGxlbmdl",
    challengeId: "challenge-1",
    rpId: "rp.example.com",
    rpName: "GuliangIAM",
    userId: "dXNlci0x",
    userDisplayName: "admin",
    timeout: 60_000,
    userVerification: "required",
  });
  mockFinish.mockResolvedValue({ credentialId: "cred-1" });
  mockCreate.mockResolvedValue(makeCreatedCredential());
  Object.defineProperty(navigator, "credentials", {
    value: { create: mockCreate },
    configurable: true,
  });
});

describe("WebAuthnCredentialsTab registration", () => {
  it("advertises only ES256 in pubKeyCredParams", async () => {
    render(<WebAuthnCredentialsTab userId="user-1" />, { wrapper: createTestWrapper() });
    await waitFor(() => expect(mockList).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getAllByRole("button", { name: "user.webauthn.add" })[0]);
    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByPlaceholderText("user.webauthn.namePlaceholder"), {
      target: { value: "Laptop" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "user.webauthn.add" }));

    await waitFor(() => expect(mockCreate).toHaveBeenCalledTimes(1));
    const options = mockCreate.mock.calls[0][0] as {
      publicKey: PublicKeyCredentialCreationOptions;
    };
    expect(options.publicKey.pubKeyCredParams).toEqual([{ type: "public-key", alg: -7 }]);
    // UV 策略来自 registration:begin，前端不得写死 preferred。
    expect(options.publicKey.authenticatorSelection?.userVerification).toBe("required");

    await waitFor(() => expect(mockFinish).toHaveBeenCalledTimes(1));
  });

  it("forwards organizationId to every credential API call (cross-tenant admin)", async () => {
    render(<WebAuthnCredentialsTab userId="user-1" organizationId="org-2" />, {
      wrapper: createTestWrapper(),
    });
    // 列表读取必须携带目标组织，否则后端 trustedOrgID 回退会话租户 → 404。
    await waitFor(() => expect(mockList).toHaveBeenCalledWith("user-1", "org-2"));

    fireEvent.click(screen.getAllByRole("button", { name: "user.webauthn.add" })[0]);
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "user.webauthn.add" }));

    await waitFor(() => expect(mockBegin).toHaveBeenCalledWith("user-1", undefined, "org-2"));
    await waitFor(() => expect(mockFinish).toHaveBeenCalledTimes(1));
    expect(mockFinish.mock.calls[0][2]).toBe("org-2");
  });
});
