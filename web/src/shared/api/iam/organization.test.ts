import { describe, it, expect, vi, beforeEach } from "vitest";
import { HttpError } from "@/shared/api/http";
import {
  getOrganizations,
  getOrganizationsPaginated,
  getOrganizationOptions,
  getOrganization,
  createOrganization,
  updateOrganization,
  deleteOrganization,
  getOrganizationIdentityBySlug,
  getDeploymentRegions,
  saveOrganizationConfiguration,
} from "./organization";

// Mock the HTTP module
const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();
vi.mock("@/shared/api/http", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/http")>();
  return {
    ...actual,
    requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
  };
});

const mockOrgListResponse = {
  items: [
    {
      id: "org-1",
      slug: "default-org",
      displayName: "Default Organization",
      websiteUrl: "https://example.com",
      logoUrl: "https://example.com/logo.png",
      defaultApplicationId: "app-1",
      status: "active",
      createdAt: "2024-01-01T00:00:00Z",
      updatedAt: "2024-01-02T00:00:00Z",
    },
  ],
  page: { page: 1, pageSize: 50, total: 1 },
};

describe("Organization API", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("getOrganizations", () => {
    it("fetches organizations and maps to list items", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockOrgListResponse);

      const result = await getOrganizations();
      expect(result).toHaveLength(1);
      expect(result[0].name).toBe("default-org");
      expect(result[0].displayName).toBe("Default Organization");
    });
  });

  describe("getOrganizationsPaginated", () => {
    it("returns paginated result", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockOrgListResponse);

      const result = await getOrganizationsPaginated({ page: 1, pageSize: 10 });
      expect(result.items).toHaveLength(1);
      expect(result.page).toBe(1);
      expect(result.total).toBe(1);
    });

    it("passes search as keyword param", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockOrgListResponse);

      await getOrganizationsPaginated({ search: "default" });
      const calledUrl = mockRequestDocumentJson.mock.calls[0][0] as string;
      expect(calledUrl).toContain("keyword=default");
    });

    it("passes status as query param", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockOrgListResponse);

      await getOrganizationsPaginated({ status: "active" });
      const calledUrl = mockRequestDocumentJson.mock.calls[0][0] as string;
      expect(calledUrl).toContain("status=active");
    });
  });

  describe("getOrganizationOptions", () => {
    it("returns options with id, name, slug, displayName, label, value", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockOrgListResponse);

      const result = await getOrganizationOptions();
      expect(result).toHaveLength(1);
      expect(result[0].id).toBe("org-1");
      expect(result[0].name).toBe("default-org");
      expect(result[0].slug).toBe("default-org");
      expect(result[0].label).toBe("Default Organization");
      expect(result[0].value).toBe("org-1");
    });
  });

  describe("getOrganization", () => {
    it("fetches organization detail by id", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        ...mockOrgListResponse.items[0],
        settings: {},
        theme: {},
        securityPolicySummary: {},
        revision: 1,
      });

      const result = await getOrganization("org-1");
      expect(result.id).toBe("org-1");
      expect(result.slug).toBe("default-org");
      expect(result.revision).toBe(1);
      // Detail GET is served with private,max-age=30 + ETag; the editor must
      // bypass the browser cache so a post-save refetch is not a stale replay.
      expect(mockRequestDocumentJson.mock.calls[0][1]).toEqual({ cache: "no-store" });
    });
  });

  describe("createOrganization", () => {
    it("sends POST request with payload and returns result", async () => {
      mockRequestDocumentJson.mockResolvedValue({ id: "org-new", status: "active" });

      const payload = {
        slug: "new-org",
        displayName: "New Organization",
        status: "active",
        defaultApplicationId: "app-1",
        initScore: 100,
      };

      const result = await createOrganization(payload);
      expect(result.id).toBe("org-new");
      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/organizations", {
        method: "POST",
        body: JSON.stringify(payload),
        headers: { "Content-Type": "application/json" },
      });
    });

    it("forwards languages in the create request body", async () => {
      mockRequestDocumentJson.mockResolvedValue({ id: "org-new", status: "created" });

      const payload = {
        slug: "new-org",
        displayName: "New Organization",
        status: "active",
        languages: ["en", "zh-CN"],
      };

      await createOrganization(payload);
      const options = mockRequestDocumentJson.mock.calls[0][1] as { body: string };
      expect(JSON.parse(options.body)).toMatchObject({ languages: ["en", "zh-CN"] });
    });
  });

  describe("updateOrganization", () => {
    it("sends PATCH request with payload", async () => {
      mockRequestDocumentJson.mockResolvedValue(undefined);

      const payload = { displayName: "Updated Org", revision: 1 };
      await updateOrganization("org-1", payload);

      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/organizations/org-1", {
        method: "PATCH",
        body: JSON.stringify(payload),
        headers: { "Content-Type": "application/json" },
      });
    });
  });

  describe("deleteOrganization", () => {
    it("sends DELETE request", async () => {
      mockRequestDocumentJson.mockResolvedValue(undefined);
      await deleteOrganization("org-1");
      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/organizations/org-1", {
        method: "DELETE",
      });
    });
  });

  describe("getDeploymentRegions", () => {
    it("requests the organizations deployment-regions endpoint under /api/v1", async () => {
      mockRequestDocumentJson.mockResolvedValue({ items: [] });

      await getDeploymentRegions();

      expect(mockRequestDocumentJson).toHaveBeenCalledWith(
        "/api/v1/organizations/deployment-regions",
      );
    });
  });

  describe("saveOrganizationConfiguration", () => {
    it("PUTs the atomic configuration payload to the /api/v1 route", async () => {
      mockRequestDocumentJson.mockResolvedValue(undefined);
      const payload = {
        organization: { displayName: "Updated Org", revision: 1 },
        settings: {},
        theme: {},
        securityPolicy: {},
      };

      await saveOrganizationConfiguration("org-1", payload);

      expect(mockRequestDocumentJson).toHaveBeenCalledWith(
        "/api/v1/organizations/org-1/configuration",
        {
          method: "PUT",
          body: JSON.stringify(payload),
          headers: { "Content-Type": "application/json" },
        },
      );
    });
  });

  describe("getOrganizationIdentityBySlug", () => {
    it("returns organization identity for valid slug", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockOrgListResponse.items[0]);

      const result = await getOrganizationIdentityBySlug("default-org");
      expect(result).not.toBeNull();
      expect(result!.id).toBe("org-1");
      expect(result!.slug).toBe("default-org");
    });

    it("returns null when organization not found", async () => {
      mockRequestDocumentJson.mockRejectedValue(new HttpError(404, null, "Not Found"));

      const result = await getOrganizationIdentityBySlug("nonexistent");
      expect(result).toBeNull();
    });

    it("propagates non-404 failures instead of masking them as not-found", async () => {
      mockRequestDocumentJson.mockRejectedValue(new HttpError(500, null, "Server Error"));

      await expect(getOrganizationIdentityBySlug("default-org")).rejects.toThrow("Server Error");
    });

    it("resolves an organization id through the by-id endpoint", async () => {
      const id = "01a1aaf0-540c-7966-af48-a1b5fd6e8ed1";
      mockRequestDocumentJson.mockResolvedValue({ id, slug: "created-org" });

      const result = await getOrganizationIdentityBySlug(id);

      expect(result).toEqual({ id, slug: "created-org" });
      expect(mockRequestDocumentJson).toHaveBeenCalledTimes(1);
      expect(mockRequestDocumentJson).toHaveBeenCalledWith(`/api/v1/organizations/${id}`);
    });

    it("returns null for a non-existent organization id and falls back to slug", async () => {
      const id = "01a1aaf0-540c-7966-af48-a1b5fd6e8ed1";
      mockRequestDocumentJson.mockRejectedValue(new HttpError(404, null, "Not Found"));

      const result = await getOrganizationIdentityBySlug(id);

      expect(result).toBeNull();
      expect(mockRequestDocumentJson).toHaveBeenNthCalledWith(1, `/api/v1/organizations/${id}`);
      expect(mockRequestDocumentJson).toHaveBeenNthCalledWith(
        2,
        `/api/v1/organizations/by-slug/${id}`,
      );
    });

    it("treats a non-UUID identifier as a slug only", async () => {
      mockRequestDocumentJson.mockRejectedValue(new HttpError(404, null, "Not Found"));

      const result = await getOrganizationIdentityBySlug("not a uuid");

      expect(result).toBeNull();
      expect(mockRequestDocumentJson).toHaveBeenCalledTimes(1);
      expect(mockRequestDocumentJson).toHaveBeenCalledWith(
        "/api/v1/organizations/by-slug/not%20a%20uuid",
      );
    });
  });
});
