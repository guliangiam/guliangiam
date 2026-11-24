import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createTestWrapper } from "@/test-utils/render";
import { AccountPasskeysSection } from "./AccountPasskeysSection";

const { mockList, mockRevoke, mockBegin, mockFinish, mockCreate } = vi.hoisted(() => ({
  mockList: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockRevoke: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockBegin: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockFinish: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockCreate: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("@/shared/lib/webauthn-b64", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/lib/webauthn-b64")>();
  return { ...actual, isWebAuthnSupported: () => true };
});

vi.mock("@/shared/lib/webauthn-error", () => ({
  registrationErrorMessageKey: () => "user.webauthn.registerFailed",
}));

vi.mock("@/shared/api/iam/auth-webauthn", () => ({
  listOwnWebAuthnCredentials: (...args: unknown[]) => mockList(...args),
  revokeOwnWebAuthnCredential: (...args: unknown[]) => mockRevoke(...args),
  webAuthnRegistrationBegin: (...args: unknown[]) => mockBegin(...args),
  finishOwnWebAuthnRegistration: (...args: unknown[]) => mockFinish(...args),
}));

function makeCreatedCredential() {
  return {
    id: "cred-1",
    type: "public-key",
    response: {
      clientDataJSON: Uint8Array.from([1, 2, 3]).buffer,
      attestationObject: Uint8Array.from([4, 5, 6]).buffer,
      getTransports: () => ["internal"],
      getPublicKey: () => Uint8Array.from([7, 8, 9]).buffer,
      getAuthenticatorData: () => Uint8Array.from([10, 11]).buffer,
      getAttestationType: () => "none",
    },
  };
}

const activeCredential = {
  id: "row-1",
  credentialId: "cred-1",
  name: "YubiKey 5C",
  status: "active",
  aaguid: "abcdef123456",
  signCount: 3,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-02T00:00:00Z",
};

beforeEach(() => {
  vi.clearAllMocks();
  mockList.mockResolvedValue([]);
  mockRevoke.mockResolvedValue(undefined);
  mockBegin.mockResolvedValue({
    challenge: "Y2hhbGxlbmdl",
    challengeId: "challenge-1",
    rpId: "rp.example.com",
    rpName: "GuliangIAM",
    userId: "dXNlci0x",
    userDisplayName: "alice",
    timeout: 60_000,
    userVerification: "required",
  });
  mockFinish.mockResolvedValue({ status: "success", credentialId: "cred-1" });
  mockCreate.mockResolvedValue(makeCreatedCredential());
  Object.defineProperty(navigator, "credentials", {
    value: { create: mockCreate },
    configurable: true,
  });
});

function renderSection() {
  return render(<AccountPasskeysSection organizationSlug="built-in" username="alice" />, {
    wrapper: createTestWrapper(),
  });
}

describe("AccountPasskeysSection", () => {
  it("lists the session user's own passkeys", async () => {
    mockList.mockResolvedValue([activeCredential]);
    renderSection();
    await waitFor(() => expect(mockList).toHaveBeenCalledTimes(1));
    expect(await screen.findByText("YubiKey 5C")).toBeTruthy();
  });

  it("shows the empty state when no passkey is registered", async () => {
    renderSection();
    expect(await screen.findByText("user.noWebauthn")).toBeTruthy();
  });

  it("registers a passkey with ES256 and the begin UV policy, bound to the session identity", async () => {
    renderSection();
    await waitFor(() => expect(mockList).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getAllByRole("button", { name: "user.webauthn.add" })[0]);

    await waitFor(() => expect(mockBegin).toHaveBeenCalledWith("built-in", "alice"));
    await waitFor(() => expect(mockCreate).toHaveBeenCalledTimes(1));
    const options = mockCreate.mock.calls[0][0] as {
      publicKey: PublicKeyCredentialCreationOptions;
    };
    expect(options.publicKey.pubKeyCredParams).toEqual([{ type: "public-key", alg: -7 }]);
    expect(options.publicKey.authenticatorSelection?.userVerification).toBe("required");

    await waitFor(() => expect(mockFinish).toHaveBeenCalledTimes(1));
    const body = mockFinish.mock.calls[0][0] as { organizationSlug: string; username: string };
    expect(body.organizationSlug).toBe("built-in");
    expect(body.username).toBe("alice");
  });

  it("revokes own passkey by row id after confirmation", async () => {
    mockList.mockResolvedValue([activeCredential]);
    renderSection();
    await screen.findByText("YubiKey 5C");

    fireEvent.click(screen.getByRole("button", { name: "user.webauthn.revoke" }));
    await screen.findByText("user.webauthn.revokeConfirm");
    const buttons = screen.getAllByRole("button", { name: "user.webauthn.revoke" });
    fireEvent.click(buttons[buttons.length - 1]);

    await waitFor(() => expect(mockRevoke).toHaveBeenCalledWith("row-1"));
  });
});
