import { afterEach, describe, expect, it, vi } from "vitest";
import { runWebAuthnLogin, runWebAuthnSignin } from "./auth-signin-providers";
import type { ResponseEnvelope } from "@/shared/api/http";

// i18next is mocked so thrown messages are the raw keys, matching the other
// auth-* lib tests.
// i18n is consumed through the named `t` export; keep default.t for legacy call sites.
vi.mock("i18next", () => {
  const t = (key: string) => key;
  return { default: { t }, t };
});

// The flow is left un-mocked on purpose: it must reach the network through
// `shared/api/iam/auth-webauthn.ts` and the real `requestDocumentJson`. Stubbing
// `fetch` (http.test.ts style) therefore pins the bytes the console actually
// sends — URL, verb and JSON body — which is exactly what drifted to the legacy
// `/api/webauthn/signin/*` routes before.
const fetchMock = vi.fn<(...args: unknown[]) => unknown>();
vi.stubGlobal("fetch", (...args: unknown[]) => fetchMock(...args));

// Pulled in by the module under test but unused by the WebAuthn path; mocking
// them keeps the import graph off the IAM API layer.
vi.mock("@/shared/lib/provider-auth", () => ({
  getDirectOAuthProviderAuthUrl: vi.fn<(...args: unknown[]) => unknown>(),
  canStartDirectOAuthProvider: vi.fn<(...args: unknown[]) => unknown>().mockReturnValue(true),
}));

function mockJsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: new Headers({ "content-type": "application/json" }),
    json: async () => body,
  } as unknown as Response;
}

function textBytes(text: string) {
  return Uint8Array.from(text, (character) => character.charCodeAt(0));
}

function bytes(text: string) {
  return textBytes(text).buffer;
}

function makeAssertion() {
  return {
    id: "cred-1",
    rawId: bytes("cred-1"),
    type: "public-key",
    response: {
      authenticatorData: bytes("auth-data"),
      clientDataJSON: bytes("client-data"),
      signature: bytes("sig"),
      userHandle: null,
    },
  };
}

function makeCredentialsContainer(credential: unknown) {
  const get = vi.fn<(...args: unknown[]) => Promise<unknown>>().mockResolvedValue(credential);
  return { credentialsContainer: { get } as unknown as CredentialsContainer, get };
}

// Mirrors WebAuthnAuthenticationBeginResult from the v1 contract.
const beginResult = {
  challenge: "Y2hhbGxlbmdl",
  challengeId: "challenge-1",
  rpId: "rp.example.com",
  allowedCredentialIds: ["Y3JlZC0x"],
  timeout: 60_000,
};

afterEach(() => {
  fetchMock.mockReset();
});

describe("runWebAuthnSignin (v1 authentication contract)", () => {
  it("posts begin and finish to /api/v1/auth/webauthn/authentication/* with a JSON finish body", async () => {
    fetchMock
      .mockResolvedValueOnce(mockJsonResponse(beginResult))
      .mockResolvedValueOnce(mockJsonResponse({ status: "success", accessToken: "access-1" }));
    const { credentialsContainer } = makeCredentialsContainer(makeAssertion());

    const loginResult = await runWebAuthnSignin({
      owner: "org-1",
      username: " alice ",
      createPayload: () => ({}),
      credentialsContainer,
    });

    expect(fetchMock).toHaveBeenCalledTimes(2);

    const [beginPath, beginInit] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(beginPath).toBe("/api/v1/auth/webauthn/authentication/begin");
    expect(beginInit.method).toBe("POST");
    expect(JSON.parse(String(beginInit.body))).toEqual({
      organizationSlug: "org-1",
      username: "alice",
    });

    const [finishPath, finishInit] = fetchMock.mock.calls[1] as [string, RequestInit];
    expect(finishPath).toBe("/api/v1/auth/webauthn/authentication/finish");
    expect(finishInit.method).toBe("POST");
    expect(new Headers(finishInit.headers).get("Content-Type")).toBe("application/json");
    // base64url (no padding) for the assertion material; credentialId is the
    // already-base64url PublicKeyCredential.id.
    expect(JSON.parse(String(finishInit.body))).toEqual({
      organizationSlug: "org-1",
      username: "alice",
      challengeId: "challenge-1",
      challenge: "Y2hhbGxlbmdl",
      credentialId: "cred-1",
      signature: "c2ln",
      authenticatorData: "YXV0aC1kYXRh",
      clientDataJSON: "Y2xpZW50LWRhdGE",
    });

    expect(loginResult).toEqual({ status: "success", accessToken: "access-1" });
  });

  it("decodes the begin challenge and allowedCredentialIds for credentials.get", async () => {
    fetchMock
      .mockResolvedValueOnce(mockJsonResponse(beginResult))
      .mockResolvedValueOnce(mockJsonResponse({ status: "success" }));
    const { credentialsContainer, get } = makeCredentialsContainer(makeAssertion());

    await runWebAuthnSignin({
      owner: "org-1",
      username: "alice",
      createPayload: () => ({}),
      credentialsContainer,
    });

    const [request] = get.mock.calls[0] as [{ publicKey: PublicKeyCredentialRequestOptions }];
    const publicKey = request.publicKey;
    expect(Array.from(publicKey.challenge as unknown as Uint8Array)).toEqual(
      Array.from(textBytes("challenge")),
    );
    expect(publicKey.rpId).toBe("rp.example.com");
    expect(publicKey.timeout).toBe(60_000);
    const [allowed] = publicKey.allowCredentials ?? [];
    expect(Array.from(allowed.id as unknown as Uint8Array)).toEqual(
      Array.from(textBytes("cred-1")),
    );
  });

  it("reads the protocol context off the payload instead of a legacy query string", async () => {
    fetchMock
      .mockResolvedValueOnce(mockJsonResponse(beginResult))
      .mockResolvedValueOnce(mockJsonResponse({ status: "success" }));
    const { credentialsContainer } = makeCredentialsContainer(makeAssertion());

    await runWebAuthnSignin({
      owner: "org-1",
      username: "alice",
      createPayload: () => ({
        type: "code",
        redirectUri: "https://rp.example.com/cb",
        state: "state-1",
        nonce: "nonce-1",
        scope: "openid profile",
        applicationId: "app-1",
      }),
      credentialsContainer,
    });

    const [finishPath, finishInit] = fetchMock.mock.calls[1] as [string, RequestInit];
    expect(finishPath).not.toContain("?");
    expect(JSON.parse(String(finishInit.body))).toMatchObject({
      applicationId: "app-1",
      responseType: "code",
      redirectUri: "https://rp.example.com/cb",
      state: "state-1",
      nonce: "nonce-1",
      scope: ["openid profile"],
    });
  });

  it("does not send the console login sentinel as an OAuth responseType", async () => {
    fetchMock
      .mockResolvedValueOnce(mockJsonResponse(beginResult))
      .mockResolvedValueOnce(mockJsonResponse({ status: "success" }));
    const { credentialsContainer } = makeCredentialsContainer(makeAssertion());

    await runWebAuthnSignin({
      owner: "org-1",
      username: "alice",
      createPayload: () => ({ type: "login" }),
      credentialsContainer,
    });

    const [, finishInit] = fetchMock.mock.calls[1] as [string, RequestInit];
    const body = JSON.parse(String(finishInit.body)) as Record<string, unknown>;
    expect(body).not.toHaveProperty("responseType");
  });

  it("forwards an array scope unchanged", async () => {
    fetchMock
      .mockResolvedValueOnce(mockJsonResponse(beginResult))
      .mockResolvedValueOnce(mockJsonResponse({ status: "success" }));
    const { credentialsContainer } = makeCredentialsContainer(makeAssertion());

    await runWebAuthnSignin({
      owner: "org-1",
      username: "alice",
      createPayload: () => ({ scope: ["openid", "profile"] }),
      credentialsContainer,
    });

    const [, finishInit] = fetchMock.mock.calls[1] as [string, RequestInit];
    expect(JSON.parse(String(finishInit.body)).scope).toEqual(["openid", "profile"]);
  });

  it("throws signinCancelled when the user dismisses the system prompt", async () => {
    fetchMock.mockResolvedValueOnce(mockJsonResponse(beginResult));
    const { credentialsContainer } = makeCredentialsContainer(null);

    await expect(
      runWebAuthnSignin({
        owner: "org-1",
        username: "alice",
        createPayload: () => ({}),
        credentialsContainer,
      }),
    ).rejects.toThrow("auth:signin.webauthn.signinCancelled");

    // The dismissed assertion never reaches finish.
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("propagates the server rejection from finish without inventing an envelope", async () => {
    fetchMock
      .mockResolvedValueOnce(mockJsonResponse(beginResult))
      .mockResolvedValueOnce(
        mockJsonResponse(
          { error: { code: "invalid_argument", message: "invalid credential" } },
          400,
        ),
      );
    const { credentialsContainer } = makeCredentialsContainer(makeAssertion());

    await expect(
      runWebAuthnSignin({
        owner: "org-1",
        username: "alice",
        createPayload: () => ({}),
        credentialsContainer,
      }),
    ).rejects.toThrow("invalid credential");
  });
});

describe("runWebAuthnLogin", () => {
  it("hands the login result to onSuccess as an ok envelope", async () => {
    const loginResult = { status: "success", accessToken: "access-1", idToken: "id-1" };
    fetchMock
      .mockResolvedValueOnce(mockJsonResponse(beginResult))
      .mockResolvedValueOnce(mockJsonResponse(loginResult));
    const { credentialsContainer } = makeCredentialsContainer(makeAssertion());
    const payload = { signinMethod: "WebAuthn", applicationId: "app-1" };
    const onSuccess = vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined);

    await runWebAuthnLogin({
      owner: "org-1",
      username: "alice",
      createPayload: () => payload,
      onSuccess,
      credentialsContainer,
    });

    expect(onSuccess).toHaveBeenCalledTimes(1);
    const handed = onSuccess.mock.calls[0][0] as {
      response: ResponseEnvelope;
      payload: unknown;
    };
    expect(handed.response).toEqual({ status: "ok", data: loginResult });
    expect(handed.payload).toEqual(payload);
  });
});
