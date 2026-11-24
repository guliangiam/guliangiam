import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { createTestQueryClient } from "@/test-utils/render";
import { queryKeys } from "@/shared/api/query-keys";
import type { OrganizationDetail } from "@/shared/api/iam/organization";
import { useOrganizationDetailPageController } from "./useOrganizationDetailPageController";

const {
  mockGetOrganizationIdentityBySlug,
  mockGetOrganization,
  mockGetOrganizationOptions,
  mockGetDeploymentRegions,
  mockGetApplicationOptions,
  mockSaveOrganizationConfiguration,
  mockMessage,
} = vi.hoisted(() => ({
  mockGetOrganizationIdentityBySlug: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockGetOrganization: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockGetOrganizationOptions: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockGetDeploymentRegions: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockGetApplicationOptions: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockSaveOrganizationConfiguration: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockMessage: {
    success: vi.fn<(content: unknown) => void>(),
    error: vi.fn<(content: unknown) => void>(),
    warning: vi.fn<(content: unknown) => void>(),
    info: vi.fn<(content: unknown) => void>(),
  },
}));

vi.mock("antd", async (importOriginal) => {
  const actual = await importOriginal<typeof import("antd")>();
  const AppWithMock = Object.assign(actual.App, {
    useApp: () => ({ message: mockMessage, notification: {}, modal: {} }),
  });
  return { ...actual, App: AppWithMock };
});

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("react-router-dom", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router-dom")>();
  return {
    ...actual,
    useParams: () => ({ organizationName: "my-org" }),
    useNavigate: () => vi.fn<(path: string) => void>(),
  };
});

vi.mock("@/shared/api/iam/organization", () => ({
  getOrganization: (...args: unknown[]) => mockGetOrganization(...args),
  getDeploymentRegions: (...args: unknown[]) => mockGetDeploymentRegions(...args),
  getOrganizationOptions: (...args: unknown[]) => mockGetOrganizationOptions(...args),
  getOrganizationIdentityBySlug: (...args: unknown[]) => mockGetOrganizationIdentityBySlug(...args),
  deleteOrganization: vi.fn<() => void>(),
  saveOrganizationConfiguration: (...args: unknown[]) => mockSaveOrganizationConfiguration(...args),
}));

vi.mock("@/shared/api/iam/application", () => ({
  getApplicationOptions: (...args: unknown[]) => mockGetApplicationOptions(...args),
}));

function createOrganizationDetail(): OrganizationDetail {
  return {
    id: "org-1",
    parentId: "",
    regionCode: "",
    slug: "my-org",
    displayName: "My Org",
    websiteUrl: "",
    logoUrl: "",
    logoDarkUrl: "",
    faviconUrl: "",
    status: "active",
    passwordOptions: [],
    ldapAttributes: [],
    settings: {},
    theme: {},
    securityPolicySummary: {},
    revision: 7,
  } as unknown as OrganizationDetail;
}

function renderController() {
  const queryClient = createTestQueryClient();
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  }

  return {
    ...renderHook(() => useOrganizationDetailPageController(), { wrapper: Wrapper }),
    invalidateSpy,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockGetOrganizationIdentityBySlug.mockResolvedValue({ id: "org-1", slug: "my-org" });
  mockGetOrganization.mockResolvedValue(createOrganizationDetail());
  mockGetOrganizationOptions.mockResolvedValue([]);
  mockGetDeploymentRegions.mockResolvedValue([]);
  mockGetApplicationOptions.mockResolvedValue([]);
  mockSaveOrganizationConfiguration.mockResolvedValue(undefined);
});

describe("useOrganizationDetailPageController save", () => {
  it("saves through the resolved organization id and refreshes the detail view", async () => {
    const { result, invalidateSpy } = renderController();
    await waitFor(() => expect(result.current.detailQuery.data).toBeTruthy());

    act(() => {
      result.current.form.setFieldsValue({ displayName: "Renamed Org" });
    });

    const values = result.current.form.getFieldsValue(true);
    await act(async () => {
      await result.current.saveMutation.mutateAsync(values);
    });

    expect(mockGetOrganizationIdentityBySlug).toHaveBeenCalledWith("my-org");
    expect(mockGetOrganization).toHaveBeenCalledWith("org-1");
    expect(mockSaveOrganizationConfiguration).toHaveBeenCalledTimes(1);

    const [orgId, payload] = mockSaveOrganizationConfiguration.mock.calls[0] as [
      string,
      { organization: { displayName: string; revision: number } },
    ];
    // The route carries the slug; the save must use the resolved id so the
    // backend can locate the resource.
    expect(orgId).toBe("org-1");
    expect(orgId).not.toBe("my-org");
    expect(payload.organization.displayName).toBe("Renamed Org");
    expect(payload.organization.revision).toBe(7);

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.organizations.all });
    expect(mockMessage.success).toHaveBeenCalledWith("updateSuccess");
  });
});
