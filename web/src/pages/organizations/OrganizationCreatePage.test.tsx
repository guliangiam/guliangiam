import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "antd";
import { QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createTestQueryClient } from "@/test-utils/render";
import OrganizationCreatePage from "./OrganizationCreatePage";

const {
  mockCreateOrganization,
  mockGetOrganizationOptions,
  mockGetDeploymentRegions,
  mockGetApplicationOptions,
  mockNavigate,
  mockMessage,
} = vi.hoisted(() => ({
  mockCreateOrganization: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockGetOrganizationOptions: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockGetDeploymentRegions: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockGetApplicationOptions: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockNavigate: vi.fn<(path: string) => void>(),
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
  return { ...actual, useNavigate: () => mockNavigate };
});

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    isLoading: false,
    hasAction: () => true,
    hasResource: () => true,
  }),
}));

vi.mock("@/shared/api/iam/organization", () => ({
  createOrganization: (...args: unknown[]) => mockCreateOrganization(...args),
  getOrganizationOptions: (...args: unknown[]) => mockGetOrganizationOptions(...args),
  getDeploymentRegions: (...args: unknown[]) => mockGetDeploymentRegions(...args),
}));

vi.mock("@/shared/api/iam/application", () => ({
  getApplicationOptions: (...args: unknown[]) => mockGetApplicationOptions(...args),
}));

vi.mock("@/shared/ui/ImageCropUpload", () => ({ ImageCropUpload: () => null }));

function renderPage() {
  const queryClient = createTestQueryClient();
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  const router = createMemoryRouter(
    [{ path: "/organizations/new", element: <OrganizationCreatePage /> }],
    { initialEntries: ["/organizations/new"] },
  );

  render(
    <QueryClientProvider client={queryClient}>
      <App component={false}>
        <RouterProvider router={router} />
      </App>
    </QueryClientProvider>,
  );

  return { invalidateSpy };
}

async function fillRequiredFields(slug: string, displayName: string) {
  const user = userEvent.setup();
  await user.type(await screen.findByPlaceholderText("organization.placeholder.slug"), slug);
  await user.type(screen.getByPlaceholderText("organization.placeholder.displayName"), displayName);
  return user;
}

async function pickRegion(user: ReturnType<typeof userEvent.setup>, label: string) {
  await user.click(screen.getByLabelText("organization.regionCode"));
  await user.click(await screen.findByText(label));
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  mockCreateOrganization.mockResolvedValue({ id: "org-new", status: "active" });
  mockGetOrganizationOptions.mockResolvedValue([]);
  mockGetDeploymentRegions.mockResolvedValue([
    {
      id: "region-1",
      regionCode: "cn-north",
      name: "North",
      countryCode: "CN",
      timezone: "Asia/Shanghai",
      isPrimary: true,
    },
  ]);
  mockGetApplicationOptions.mockResolvedValue([]);
});

describe("OrganizationCreatePage save", () => {
  it("submits slug, displayName, status and region, then refreshes and navigates", async () => {
    const { invalidateSpy } = renderPage();
    const user = await fillRequiredFields("acme", "Acme Corp");

    await pickRegion(user, "North (cn-north)");
    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(mockCreateOrganization).toHaveBeenCalledTimes(1));
    expect(mockCreateOrganization.mock.calls[0][0]).toMatchObject({
      slug: "acme",
      displayName: "Acme Corp",
      status: "active",
      regionCode: "cn-north",
    });

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith("/organizations/org-new"));
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["organizations"],
    });
    expect(mockMessage.success).toHaveBeenCalledWith("createSuccess");
  });

  it("carries the password section values into the create payload", async () => {
    renderPage();
    const user = await fillRequiredFields("acme", "Acme Corp");

    fireEvent.change(screen.getByPlaceholderText("organization.passwordSaltAutoGenerated"), {
      target: { value: "operator-salt" },
    });
    fireEvent.change(screen.getByPlaceholderText("organization.placeholder.defaultPassword"), {
      target: { value: "InitPass123!" },
    });
    const masterInputs = screen.getAllByPlaceholderText("leaveEmptyToKeepCurrent");
    // masterPassword and masterVerificationCode share the same placeholder.
    fireEvent.change(masterInputs[0], { target: { value: "MasterPass123!" } });

    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(mockCreateOrganization).toHaveBeenCalledTimes(1));
    expect(mockCreateOrganization.mock.calls[0][0]).toMatchObject({
      slug: "acme",
      displayName: "Acme Corp",
      passwordSalt: "operator-salt",
      defaultPassword: "InitPass123!",
      masterPassword: "MasterPass123!",
      passwordType: "argon2id",
      passwordObfuscatorType: "Plain",
    });
  });

  it("carries security and sign-in switches as booleans in the payload", async () => {
    renderPage();
    const user = await fillRequiredFields("acme", "Acme Corp");

    fireEvent.click(screen.getByLabelText("organization.disableSignin"));
    fireEvent.click(screen.getByLabelText("organization.useEmailAsUsername"));

    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(mockCreateOrganization).toHaveBeenCalledTimes(1));
    expect(mockCreateOrganization.mock.calls[0][0]).toMatchObject({
      slug: "acme",
      displayName: "Acme Corp",
      disableSignin: true,
      useEmailAsUsername: true,
    });
  });
});
