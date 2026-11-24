import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createTestWrapper } from "@/test-utils/render";
import { TrustFormModal } from "./TrustFormModal";

const mockCreateMutateAsync = vi.fn<(...args: unknown[]) => unknown>();
const mockPreviewMutateAsync = vi.fn<(...args: unknown[]) => unknown>();
const mockCreateReset = vi.fn<(...args: unknown[]) => unknown>();
const mockPreviewReset = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, options?: { defaultValue?: string }) => options?.defaultValue ?? key,
    i18n: { language: "en" },
  }),
}));

vi.mock("@/shared/hooks/use-identity-pools", () => ({
  useCreateFederationTrust: () => ({
    mutateAsync: mockCreateMutateAsync,
    reset: mockCreateReset,
    isPending: false,
  }),
  useMatchFederationTrustSubject: () => ({
    mutateAsync: mockPreviewMutateAsync,
    reset: mockPreviewReset,
    isPending: false,
  }),
}));

describe("TrustFormModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPreviewMutateAsync.mockResolvedValue([
      { subject: "repo:acme/api", allowed: true },
      { subject: "repo:other/api", allowed: false },
    ]);
    mockCreateMutateAsync.mockResolvedValue({});
  });

  it("previews allowed and denied samples, then creates with the backend DTO", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn<(...args: unknown[]) => unknown>();
    render(<TrustFormModal open poolId="pool-1" onClose={onClose} />, {
      wrapper: createTestWrapper(),
    });

    await user.type(screen.getByLabelText("Name"), "github-production");
    await user.type(screen.getByLabelText("Issuer"), "https://issuer.example.com");
    await user.type(screen.getByLabelText("JWKS URI"), "https://issuer.example.com/jwks.json");
    await user.type(screen.getByLabelText("Audience"), "api://workload{enter}");
    await user.type(screen.getByLabelText("Subject patterns"), "repo:acme/*{enter}");
    await user.type(
      screen.getByLabelText("Sample subjects"),
      "repo:acme/api,repo:other/api{enter}",
    );

    await user.click(screen.getByRole("button", { name: "Preview subject matches" }));

    await waitFor(() =>
      expect(mockPreviewMutateAsync).toHaveBeenCalledWith({
        patterns: ["repo:acme/*"],
        samples: ["repo:acme/api", "repo:other/api"],
      }),
    );
    expect(await screen.findByText("Allowed")).toBeInTheDocument();
    expect(screen.getByText("Denied")).toBeInTheDocument();
    expect(screen.getAllByText("repo:acme/api").length).toBeGreaterThan(0);
    expect(screen.getAllByText("repo:other/api").length).toBeGreaterThan(0);

    await user.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() =>
      expect(mockCreateMutateAsync).toHaveBeenCalledWith({
        name: "github-production",
        issuer: "https://issuer.example.com",
        jwksUri: "https://issuer.example.com/jwks.json",
        audience: ["api://workload"],
        subjects: ["repo:acme/*"],
      }),
    );
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
