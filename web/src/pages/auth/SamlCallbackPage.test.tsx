import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

import SamlCallbackPage from "./SamlCallbackPage";
import { createTestWrapper } from "../../test-utils/render";

const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => Promise<unknown>>();
const mockNavigate = vi.fn<(...args: unknown[]) => unknown>();
let mockSearch = "";

vi.mock("react-router-dom", () => ({
  useLocation: () => ({ search: mockSearch }),
  useNavigate: () => mockNavigate,
}));

vi.mock("@/shared/api/http", () => ({
  requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key, i18n: { language: "en" } }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("i18next", () => ({
  t: (key: string) => key,
}));

function buildAcsRelayState(organizationSlug: string, providerId: string, nonce: string): string {
  const encoded = window
    .btoa(JSON.stringify({ o: organizationSlug, p: providerId, s: nonce }))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
  return `v1.1793000000.${"b".repeat(64)}.${encoded}`;
}

function renderPage() {
  return render(<SamlCallbackPage />, { wrapper: createTestWrapper() });
}

describe("SamlCallbackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSearch = "";
    mockRequestDocumentJson.mockResolvedValue(undefined);
  });

  it("forwards SAMLResponse/RelayState to the SP ACS and navigates home", async () => {
    const relayState = buildAcsRelayState("acme", "provider-1", "nonce-1");
    mockSearch = `?${new URLSearchParams({
      SAMLResponse: "saml-response",
      RelayState: relayState,
    }).toString()}`;

    renderPage();

    await waitFor(() => {
      expect(mockRequestDocumentJson).toHaveBeenCalledWith(
        "/api/v1/auth/saml/acs/acme/provider-1",
        {
          method: "POST",
          body: `SAMLResponse=saml-response&RelayState=${encodeURIComponent(relayState)}`,
          headers: { "Content-Type": "application/x-www-form-urlencoded" },
        },
      );
    });
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/", { replace: true });
    });
  });

  it("does not call the ACS when SAMLResponse is missing", async () => {
    const relayState = buildAcsRelayState("acme", "provider-1", "nonce-1");
    mockSearch = `?${new URLSearchParams({ SAMLRequest: "saml-request", RelayState: relayState }).toString()}`;

    renderPage();

    expect(await screen.findByText("saml.missingCallbackParams")).toBeInTheDocument();
    expect(mockRequestDocumentJson).not.toHaveBeenCalled();
  });

  it("does not call the ACS when RelayState is missing", async () => {
    mockSearch = `?${new URLSearchParams({ SAMLResponse: "saml-response" }).toString()}`;

    renderPage();

    expect(await screen.findByText("saml.missingCallbackParams")).toBeInTheDocument();
    expect(mockRequestDocumentJson).not.toHaveBeenCalled();
  });

  it("surfaces the server error without echoing the assertion", async () => {
    const relayState = buildAcsRelayState("acme", "provider-1", "nonce-1");
    mockSearch = `?${new URLSearchParams({
      SAMLResponse: "saml-response",
      RelayState: relayState,
    }).toString()}`;
    mockRequestDocumentJson.mockRejectedValue(
      new Error("SAML state is missing, expired, or already used"),
    );

    renderPage();

    expect(
      await screen.findByText("SAML state is missing, expired, or already used"),
    ).toBeInTheDocument();
    expect(screen.queryByText("saml-response")).not.toBeInTheDocument();
  });
});
