import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  getOwnProfile,
  registerOwnFaceId,
  revokeOwnFaceId,
  updateOwnProfile,
  verifyOwnIdentification,
} from "./auth-selfprofile";

const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/http", () => ({
  requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
}));

const minimalUserDetail = {
  id: "user-self",
  organizationId: "org-1",
  username: "alice",
  displayName: "Alice",
  avatarUrl: "",
  email: "alice@example.com",
  phone: "",
  userType: "normal-user",
  status: "active",
  isAdmin: false,
  isForbidden: false,
  createdAt: "2026-01-01T00:00:00Z",
  revision: 1,
  externalId: "",
  preferredMfaType: "",
  mfaSetupSummary: [],
  profile: {},
  credentialSummary: {},
  identities: [],
  mfaMethods: [],
  webauthnCredentialCount: 0,
  roleBindings: [],
  groupMemberships: [],
  consentSummary: {},
};

function lastCall() {
  const call = mockRequestDocumentJson.mock.calls.at(-1);
  return { url: String(call?.[0]), options: (call?.[1] ?? {}) as Record<string, unknown> };
}

describe("auth self-profile client", () => {
  beforeEach(() => {
    mockRequestDocumentJson.mockReset();
    mockRequestDocumentJson.mockResolvedValue({});
  });

  it("reads own profile from /auth/profile bypassing HTTP cache", async () => {
    mockRequestDocumentJson.mockResolvedValue(minimalUserDetail);
    const detail = await getOwnProfile();
    const { url, options } = lastCall();
    expect(url).toBe("/api/v1/auth/profile");
    expect(options.cache).toBe("no-store");
    expect(detail.id).toBe("user-self");
    expect(detail.username).toBe("alice");
  });

  it("patches own profile with partial fields (merge semantics)", async () => {
    await updateOwnProfile({ displayName: "New", bio: "hi" });
    const { url, options } = lastCall();
    expect(url).toBe("/api/v1/auth/profile");
    expect(options.method).toBe("PATCH");
    expect(JSON.parse(String(options.body))).toEqual({ displayName: "New", bio: "hi" });
  });

  it("initiates self real-name verification via POST", async () => {
    mockRequestDocumentJson.mockResolvedValue({ status: "pending", data: {} });
    await verifyOwnIdentification();
    const { url, options } = lastCall();
    expect(url).toBe("/api/v1/auth/verify-identification");
    expect(options.method).toBe("POST");
  });

  it("enrolls own FaceID via multipart POST to /auth/faceids (distinct from public login /auth/faceid)", async () => {
    mockRequestDocumentJson.mockResolvedValue({ id: "f1", imageUrl: "", createdAt: "2026-01-01" });
    const file = new File(["bytes"], "face.png", { type: "image/png" });
    await registerOwnFaceId({ file, providerId: "p1", deviceInfo: "web" });
    const { url, options } = lastCall();
    expect(url).toBe("/api/v1/auth/faceids");
    expect(options.method).toBe("POST");
    const body = options.body as FormData;
    expect(body).toBeInstanceOf(FormData);
    expect(body.get("providerId")).toBe("p1");
    expect(body.get("file")).toBeInstanceOf(File);
  });

  it("revokes own FaceID via DELETE to /auth/faceids", async () => {
    await revokeOwnFaceId();
    const { url, options } = lastCall();
    expect(url).toBe("/api/v1/auth/faceids");
    expect(options.method).toBe("DELETE");
  });
});
