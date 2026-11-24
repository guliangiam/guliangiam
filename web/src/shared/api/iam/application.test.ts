import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  getApplications,
  getApplicationsPaginated,
  getApplicationOptions,
  getApplication,
  getApplicationByClientId,
  resolveApplication,
  getApplicationDeleteSummary,
  createApplication,
  updateApplication,
  deleteApplication,
  updateApplicationProviderBindings,
} from "./application";

// Mock the HTTP module
const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();
vi.mock("@/shared/api/http", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/http")>();
  return {
    ...actual,
    requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
  };
});

const mockAppListResponse = {
  items: [
    {
      id: "app-1",
      organizationId: "org-1",
      slug: "my-app",
      name: "my-app",
      displayName: "My Application",
      category: "Web",
      appType: "web",
      domain: "app.example.com",
      status: "active",
      logo: "",
      isVisible: true,
      clientId: "client-123",
      redirectUris: ["https://app.example.com/callback"],
      providerIds: ["prov-1"],
      createdAt: "2024-01-01T00:00:00Z",
      updatedAt: "2024-01-02T00:00:00Z",
    },
  ],
  page: { page: 1, pageSize: 50, total: 1 },
};

describe("Application API", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("getApplications", () => {
    it("fetches applications and maps to list items", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockAppListResponse);

      const result = await getApplications();
      expect(result).toHaveLength(1);
      expect(result[0].name).toBe("my-app");
      expect(result[0].displayName).toBe("My Application");
      expect(result[0].organizationId).toBe("org-1");
    });

    it("passes organizationId as query param", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockAppListResponse);

      await getApplications("org-1");
      const calledUrl = mockRequestDocumentJson.mock.calls[0][0] as string;
      expect(calledUrl).toContain("organizationId=org-1");
    });

    it("preserves homepageUrl from the list projection", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        ...mockAppListResponse,
        items: [
          {
            ...mockAppListResponse.items[0],
            homepageUrl: "https://portal.example.com",
          },
        ],
      });

      const result = await getApplications();
      expect((result[0] as { homepageUrl?: string }).homepageUrl).toBe(
        "https://portal.example.com",
      );
    });
  });

  describe("getApplicationsPaginated", () => {
    it("returns paginated result with correct structure", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockAppListResponse);

      const result = await getApplicationsPaginated({ page: 1, pageSize: 10 });
      expect(result.items).toHaveLength(1);
      expect(result.page).toBe(1);
      expect(result.pageSize).toBe(50);
      expect(result.total).toBe(1);
    });

    it("passes search as keyword param", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockAppListResponse);

      await getApplicationsPaginated({ search: "my app" });
      const calledUrl = mockRequestDocumentJson.mock.calls[0][0] as string;
      expect(calledUrl).toContain("keyword=my+app");
    });
  });

  describe("getApplicationOptions", () => {
    it("returns options with id, slug, displayName", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockAppListResponse);

      const result = await getApplicationOptions();
      expect(result).toHaveLength(1);
      expect(result[0].id).toBe("app-1");
      expect(result[0].slug).toBe("my-app");
      expect(result[0].displayName).toBe("My Application");
    });

    it("preserves homepageUrl for portal quick-launch cards", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        ...mockAppListResponse,
        items: [
          {
            ...mockAppListResponse.items[0],
            homepageUrl: "https://portal.example.com",
          },
        ],
      });

      const result = await getApplicationOptions();
      expect((result[0] as { homepageUrl?: string }).homepageUrl).toBe(
        "https://portal.example.com",
      );
    });
  });

  describe("getApplication", () => {
    it("fetches application detail by id", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        ...mockAppListResponse.items[0],
        description: "A test application",
        homepageUrl: "https://app.example.com",
        tokenEndpointAuthMethod: "client_secret_basic",
        grantTypes: ["authorization_code"],
        revision: 1,
      });

      const result = await getApplication("app-1");
      expect(result.id).toBe("app-1");
      expect(result.slug).toBe("my-app");
      expect(result.description).toBe("A test application");
      expect(result.revision).toBe(1);
      // Detail GET is served with private,max-age=30 + ETag; the editor must
      // bypass the browser cache so a post-save refetch is not a stale replay.
      expect(mockRequestDocumentJson.mock.calls[0][1]).toEqual({ cache: "no-store" });
    });
  });

  describe("getApplicationByClientId", () => {
    it("parses the minimal public discovery projection", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        id: "app-1",
        organizationId: "org-1",
        slug: "my-app",
        name: "my-app",
        displayName: "My Application",
        description: "A test application",
        homepageUrl: "https://app.example.com",
        clientId: "client-123",
        logo: "https://app.example.com/logo.png",
        title: "My Application",
        favicon: "https://app.example.com/favicon.ico",
        isShared: false,
        enableAutoSignin: true,
        enableSignup: true,
        forcedRedirectOrigin: "https://app.example.com",
        signupUrl: "/signup/my-app",
        forgetUrl: "/forgot-password/my-app",
        termsOfUse: "terms",
        privacyPolicy: "privacy",
        orgChoiceMode: "search",
        signinHtml: "",
        signupHtml: "<div>signup</div>",
        headerHtml: "<header>h</header>",
        footerHtml: "<footer>f</footer>",
        customCss: ".x { color: red; }",
        customHeader: '<meta name="description" content="x">',
        formCss: "",
        formCssMobile: "",
        formSideHtml: "",
        formBackgroundUrl: "",
        formBackgroundUrlMobile: "",
        formOffset: "4",
        signinMethods: ["password"],
        signinItems: [{ name: "Logo", visible: true }],
        signupItems: [{ name: "Username", visible: true }],
      });

      const result = await getApplicationByClientId("client-123");
      expect(result.clientId).toBe("client-123");
      expect(result.slug).toBe("my-app");
      expect(result.signinMethods).toEqual(["password"]);
      expect(result.enableSignup).toBe(true);
      expect(result.orgChoiceMode).toBe("search");
      expect(result.privacyPolicy).toBe("privacy");
      expect(result.signupHtml).toBe("<div>signup</div>");
      expect(result.headerHtml).toBe("<header>h</header>");
      expect(result.footerHtml).toBe("<footer>f</footer>");
      expect(result.customCss).toBe(".x { color: red; }");
      expect(result.customHeader).toBe('<meta name="description" content="x">');
      expect(mockRequestDocumentJson).toHaveBeenCalledWith(
        "/api/v1/applications/by-client-id/client-123",
      );
    });
  });

  describe("resolveApplication", () => {
    it("parses the same public projection as by-client-id", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        id: "app-1",
        organizationId: "org-1",
        slug: "my-app",
        name: "my-app",
        clientId: "client-123",
        customCss: ".x { color: red; }",
        footerHtml: "<footer>f</footer>",
        enableSignup: true,
        orgChoiceMode: "direct",
        signinMethods: ["password"],
        signinItems: [],
        signupItems: [],
      });

      const result = await resolveApplication("built-in", "my-app");
      expect(result?.slug).toBe("my-app");
      expect(result?.customCss).toBe(".x { color: red; }");
      expect(result?.footerHtml).toBe("<footer>f</footer>");
      expect(result?.enableSignup).toBe(true);
      expect(result?.orgChoiceMode).toBe("direct");
      expect(mockRequestDocumentJson).toHaveBeenCalledWith(
        "/api/v1/applications/resolve?ownerSlug=built-in&appSlug=my-app",
      );
    });

    it("returns null when the lookup fails", async () => {
      mockRequestDocumentJson.mockRejectedValue(new Error("404"));
      const result = await resolveApplication("built-in", "missing");
      expect(result).toBeNull();
    });
  });

  describe("getApplicationDeleteSummary", () => {
    it("fetches delete summary for an application", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        applicationId: "app-1",
        slug: "my-app",
        displayName: "My Application",
        providerBindingCount: 2,
        activeTokenCount: 5,
        activeConsentCount: 3,
      });

      const result = await getApplicationDeleteSummary("app-1");
      expect(result.applicationId).toBe("app-1");
      expect(result.providerBindingCount).toBe(2);
      expect(result.activeTokenCount).toBe(5);
    });
  });

  describe("createApplication", () => {
    it("sends POST request with payload and returns result", async () => {
      mockRequestDocumentJson.mockResolvedValue({ id: "app-new", status: "active" });

      const payload = {
        organizationId: "org-1",
        slug: "new-app",
        displayName: "New App",
        status: "active",
      };

      const result = await createApplication(payload);
      expect(result.id).toBe("app-new");
      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/applications", {
        method: "POST",
        body: JSON.stringify(payload),
        headers: { "Content-Type": "application/json" },
      });
    });
  });

  describe("updateApplication", () => {
    it("sends PATCH request with payload", async () => {
      mockRequestDocumentJson.mockResolvedValue(undefined);

      const payload = { displayName: "Updated App", revision: 1 };
      await updateApplication("app-1", payload);

      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/applications/app-1", {
        method: "PATCH",
        body: JSON.stringify(payload),
        headers: { "Content-Type": "application/json" },
      });
    });
  });

  describe("deleteApplication", () => {
    it("sends DELETE request", async () => {
      mockRequestDocumentJson.mockResolvedValue(undefined);
      await deleteApplication("app-1");
      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/applications/app-1", {
        method: "DELETE",
      });
    });
  });

  describe("updateApplicationProviderBindings", () => {
    it("sends PUT request with provider bindings", async () => {
      mockRequestDocumentJson.mockResolvedValue(undefined);

      await updateApplicationProviderBindings("app-1", ["prov-1", "prov-2"]);

      expect(mockRequestDocumentJson).toHaveBeenCalledWith(
        "/api/v1/applications/app-1/provider-bindings",
        {
          method: "PUT",
          body: JSON.stringify({ items: [{ providerId: "prov-1" }, { providerId: "prov-2" }] }),
          headers: { "Content-Type": "application/json" },
        },
      );
    });
  });
});
