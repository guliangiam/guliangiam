import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";

import { useSessionStore } from "@/features/session/session-store";
import { PasswordChangeBanner } from "./PasswordChangeBanner";

const navigateMock = vi.fn<(path: string) => void>();

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
    i18n: { language: "en" },
  }),
}));

vi.mock("react-router-dom", () => ({
  useNavigate: () => navigateMock,
}));

beforeEach(() => {
  navigateMock.mockReset();
  useSessionStore.setState({ needUpdatePassword: null });
});

describe("PasswordChangeBanner", () => {
  it("shows the notice and navigates to the account page when flagged", () => {
    useSessionStore.setState({ needUpdatePassword: true });

    render(<PasswordChangeBanner />);

    expect(screen.getByText("auth:needUpdatePasswordBanner")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Change Password" }));
    expect(navigateMock).toHaveBeenCalledWith("/account");
  });

  it("renders nothing when the flag is false or unset", () => {
    useSessionStore.setState({ needUpdatePassword: false });
    const { container } = render(<PasswordChangeBanner />);

    expect(screen.queryByText("auth:needUpdatePasswordBanner")).toBeNull();
    expect(container).toBeEmptyDOMElement();
  });
});
