import { beforeEach, describe, expect, it, vi } from "vitest";

const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => Promise<unknown>>();
const mockGetCaptcha = vi.fn<(...args: unknown[]) => Promise<unknown>>();
const mockGetNotification = vi.fn<(...args: unknown[]) => Promise<unknown>>();
const mockGetFaceId = vi.fn<(...args: unknown[]) => Promise<unknown>>();

vi.mock("@/shared/api/http", () => ({
  requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
}));

vi.mock("./captcha-provider", () => ({
  getIamCaptchaProvidersPaginated: (...args: unknown[]) => mockGetCaptcha(...args),
}));

vi.mock("./notification-provider", () => ({
  getIamNotificationProvidersPaginated: (...args: unknown[]) => mockGetNotification(...args),
}));

vi.mock("./face-id-provider", () => ({
  getIamFaceIdProvidersPaginated: (...args: unknown[]) => mockGetFaceId(...args),
}));

import {
  getUnifiedProvidersPaginated,
  getProvider,
  getProviderConfig,
  updateProviderConfig,
  PROVIDER_CONFIG_TYPE_DEFAULT,
} from "./provider-extended";

function unifiedResponse(items: unknown[]) {
  return { items, page: { page: 1, pageSize: 500, total: items.length } };
}

function unifiedRow(id: string, name: string, providerType: string, enabled = true) {
  return {
    id,
    organizationId: "org-1",
    name,
    displayName: name,
    category: "OAuth",
    providerType,
    subType: "",
    status: "active",
    enabled,
    clientId: "",
    providerUrl: "",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

function captchaRow(id: string, name: string, providerType: string, enabled = true) {
  return {
    id,
    organizationId: "org-1",
    name,
    providerType,
    siteKey: "site-key",
    isEnabled: enabled,
    revision: 3,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

function notificationRow(id: string, name: string, providerType: string, enabled = true) {
  return {
    id,
    organizationId: "org-1",
    name,
    providerType,
    isEnabled: enabled,
    revision: 2,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

function faceIdRow(id: string, name: string, providerType: string, enabled = true) {
  return {
    id,
    organizationId: "org-1",
    name,
    providerType,
    isEnabled: enabled,
    revision: 1,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

describe("getUnifiedProvidersPaginated", () => {
  beforeEach(() => {
    mockRequestDocumentJson.mockReset();
    mockGetCaptcha.mockReset();
    mockGetNotification.mockReset();
    mockGetFaceId.mockReset();
    mockGetCaptcha.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 500 });
    mockGetNotification.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 500 });
    mockGetFaceId.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 500 });
    mockRequestDocumentJson.mockResolvedValue(unifiedResponse([]));
  });

  it("merges all four sources and sorts by name when no category filter", async () => {
    mockRequestDocumentJson.mockResolvedValue(
      unifiedResponse([unifiedRow("u-1", "Zeta OAuth", "Google")]),
    );
    mockGetCaptcha.mockResolvedValue({
      items: [captchaRow("c-1", "Alpha Captcha", "recaptcha")],
      total: 1,
      page: 1,
      pageSize: 500,
    });
    mockGetNotification.mockResolvedValue({
      items: [notificationRow("n-1", "Beta Notify", "dingtalk")],
      total: 1,
      page: 1,
      pageSize: 500,
    });
    mockGetFaceId.mockResolvedValue({
      items: [faceIdRow("f-1", "Gamma Face", "aliyun_facebody")],
      total: 1,
      page: 1,
      pageSize: 500,
    });

    const result = await getUnifiedProvidersPaginated({ page: 1, pageSize: 50 });

    expect(mockGetCaptcha).toHaveBeenCalled();
    expect(mockGetNotification).toHaveBeenCalled();
    expect(mockGetFaceId).toHaveBeenCalled();
    expect(result.total).toBe(4);
    expect(result.items.map((item) => item.name)).toEqual([
      "Alpha Captcha",
      "Beta Notify",
      "Gamma Face",
      "Zeta OAuth",
    ]);
    expect(result.items[0].category).toBe("Captcha");
    expect(result.items[0].source).toBe("captcha");
    expect(result.items[0].enabled).toBe(true);
    expect(result.items[3].source).toBe("unified");
  });

  it("only queries the matching dedicated source when a category filter is set", async () => {
    mockGetCaptcha.mockResolvedValue({
      items: [captchaRow("c-1", "Alpha Captcha", "recaptcha")],
      total: 1,
      page: 1,
      pageSize: 500,
    });

    const result = await getUnifiedProvidersPaginated({
      page: 1,
      pageSize: 50,
      category: "Captcha",
    });

    expect(mockGetCaptcha).toHaveBeenCalled();
    expect(mockGetNotification).not.toHaveBeenCalled();
    expect(mockGetFaceId).not.toHaveBeenCalled();
    // 统一源仍按 category 过滤查询（历史遗留行）。
    const unifiedUrl = String(mockRequestDocumentJson.mock.calls[0][0]);
    expect(unifiedUrl).toContain("category=Captcha");
    expect(result.total).toBe(1);
    expect(result.items[0].source).toBe("captcha");
  });

  it("normalizes providerType filter across backend kinds and UI values", async () => {
    mockRequestDocumentJson.mockResolvedValue(
      unifiedResponse([
        unifiedRow("u-1", "Zeta OAuth", "Google"),
        unifiedRow("u-2", "Legacy reCAPTCHA", "reCAPTCHA"),
      ]),
    );
    mockGetCaptcha.mockResolvedValue({
      items: [
        captchaRow("c-1", "Alpha Captcha", "recaptcha"),
        captchaRow("c-2", "Beta Captcha", "hcaptcha"),
      ],
      total: 2,
      page: 1,
      pageSize: 500,
    });

    const result = await getUnifiedProvidersPaginated({
      page: 1,
      pageSize: 50,
      providerType: "recaptcha",
    });

    expect(result.items.map((item) => item.id).sort()).toEqual(["c-1", "u-2"]);
    expect(result.total).toBe(2);
  });

  it("paginates the merged rows client-side", async () => {
    mockRequestDocumentJson.mockResolvedValue(
      unifiedResponse([
        unifiedRow("u-1", "A One", "Google"),
        unifiedRow("u-2", "D Four", "GitHub"),
      ]),
    );
    mockGetCaptcha.mockResolvedValue({
      items: [captchaRow("c-1", "B Two", "recaptcha")],
      total: 1,
      page: 1,
      pageSize: 500,
    });
    mockGetNotification.mockResolvedValue({
      items: [notificationRow("n-1", "C Three", "dingtalk")],
      total: 1,
      page: 1,
      pageSize: 500,
    });

    const page1 = await getUnifiedProvidersPaginated({ page: 1, pageSize: 2 });
    expect(page1.total).toBe(4);
    expect(page1.items.map((item) => item.name)).toEqual(["A One", "B Two"]);

    const page2 = await getUnifiedProvidersPaginated({ page: 2, pageSize: 2 });
    expect(page2.items.map((item) => item.name)).toEqual(["C Three", "D Four"]);
  });

  it("passes organizationId and search through to every source", async () => {
    await getUnifiedProvidersPaginated({
      page: 1,
      pageSize: 50,
      search: "alpha",
      organizationId: "org-9",
    });

    const unifiedUrl = String(mockRequestDocumentJson.mock.calls[0][0]);
    expect(unifiedUrl).toContain("organizationId=org-9");
    expect(unifiedUrl).toContain("keyword=alpha");
    expect(mockGetCaptcha).toHaveBeenCalledWith(
      expect.objectContaining({ organizationId: "org-9", search: "alpha" }),
    );
    expect(mockGetNotification).toHaveBeenCalledWith(
      expect.objectContaining({ organizationId: "org-9", search: "alpha" }),
    );
    expect(mockGetFaceId).toHaveBeenCalledWith(
      expect.objectContaining({ organizationId: "org-9", search: "alpha" }),
    );
  });
});

describe("provider config endpoint (IDV idvSceneId)", () => {
  beforeEach(() => {
    mockRequestDocumentJson.mockReset();
  });

  it("GET parses the config map and targets the dedicated config path", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      providerId: "p-1",
      configType: "default",
      config: { idvSceneId: "scene-123" },
    });

    const config = await getProviderConfig("p-1", "org-9");

    const [url, init] = mockRequestDocumentJson.mock.calls[0];
    expect(String(url)).toBe("/api/v1/providers/p-1/config?organizationId=org-9");
    expect(init).toMatchObject({ cache: "no-store" });
    expect(config).toEqual({ idvSceneId: "scene-123" });
  });

  it("GET normalizes a null config to an empty object (backend returns no row)", async () => {
    mockRequestDocumentJson.mockResolvedValue({ providerId: "p-1", config: null });
    await expect(getProviderConfig("p-1")).resolves.toEqual({});
  });

  it("GET normalizes a missing config to an empty object", async () => {
    mockRequestDocumentJson.mockResolvedValue({ providerId: "p-1" });
    await expect(getProviderConfig("p-1")).resolves.toEqual({});
  });

  it("PUT sends configType=default plus the full config map", async () => {
    mockRequestDocumentJson.mockResolvedValue(undefined);

    await updateProviderConfig("p-1", { idvSceneId: "scene-123", other: "keep" }, "org-9");

    const [url, init] = mockRequestDocumentJson.mock.calls[0];
    expect(String(url)).toBe("/api/v1/providers/p-1/config?organizationId=org-9");
    expect(init).toMatchObject({
      method: "PUT",
      headers: { "Content-Type": "application/json" },
    });
    expect(JSON.parse(String((init as { body: string }).body))).toEqual({
      configType: PROVIDER_CONFIG_TYPE_DEFAULT,
      config: { idvSceneId: "scene-123", other: "keep" },
    });
  });

  it("PUT omits the org query when no organizationId is given", async () => {
    mockRequestDocumentJson.mockResolvedValue(undefined);
    await updateProviderConfig("p-1", { idvSceneId: "" });
    expect(String(mockRequestDocumentJson.mock.calls[0][0])).toBe("/api/v1/providers/p-1/config");
  });
});

describe("provider detail credential indicators", () => {
  const baseDetail = {
    id: "p-1",
    organizationId: "org-1",
    name: "smtp",
    displayName: "SMTP",
    category: "Email",
    providerType: "SMTP",
    clientId: "",
    providerUrl: "",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    revision: 2,
  };

  beforeEach(() => {
    mockRequestDocumentJson.mockReset();
  });

  it("keeps all eight Set/Masked fields returned by the backend", async () => {
    // Regression: Zod strip used to drop these indicators, so the editor could
    // not tell that a secret was already stored.
    mockRequestDocumentJson.mockResolvedValue({
      ...baseDetail,
      clientSecretSet: true,
      clientSecretMasked: "****abcd",
      clientSecret2Set: true,
      clientSecret2Masked: "****efgh",
      passwordSet: true,
      passwordMasked: "****ijkl",
      appSecretSet: true,
      appSecretMasked: "****mnop",
    });

    const detail = await getProvider("p-1", "org-1");

    expect(detail.clientSecretSet).toBe(true);
    expect(detail.clientSecretMasked).toBe("****abcd");
    expect(detail.clientSecret2Set).toBe(true);
    expect(detail.clientSecret2Masked).toBe("****efgh");
    expect(detail.passwordSet).toBe(true);
    expect(detail.passwordMasked).toBe("****ijkl");
    expect(detail.appSecretSet).toBe(true);
    expect(detail.appSecretMasked).toBe("****mnop");
    // 明文机密仍不回显。
    expect(detail.clientSecret).toBe("");
    expect(detail.password).toBe("");
  });

  it("defaults missing indicators to unset without failing the parse", async () => {
    mockRequestDocumentJson.mockResolvedValue(baseDetail);

    const detail = await getProvider("p-1");

    expect(detail.clientSecretSet).toBe(false);
    expect(detail.clientSecretMasked).toBe("");
    expect(detail.clientSecret2Set).toBe(false);
    expect(detail.passwordSet).toBe(false);
    expect(detail.appSecretSet).toBe(false);
    expect(detail.appSecretMasked).toBe("");
  });
});
