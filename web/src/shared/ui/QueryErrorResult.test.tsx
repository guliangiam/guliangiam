import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpError } from "@/shared/api/http";
import { createTestWrapper } from "@/test-utils/render";
import { NotFoundResult, QueryErrorResult } from "./QueryErrorResult";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
}));

describe("QueryErrorResult", () => {
  it("shows an accessible 403 without retry for permission errors", () => {
    render(
      <QueryErrorResult
        error={new HttpError(403, null, "Forbidden")}
        onRetry={vi.fn<() => void>()}
      />,
      {
        wrapper: createTestWrapper(),
      },
    );

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.getByText("common:forbidden")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "retry" })).not.toBeInTheDocument();
  });

  it("shows NotFound instead of a recoverable error for 404 responses", () => {
    render(
      <QueryErrorResult
        error={new HttpError(404, null, "Missing")}
        onRetry={vi.fn<() => void>()}
        notFoundTitle="token.notFound"
        notFoundDescription="token.tokenNotExist"
      />,
      { wrapper: createTestWrapper() },
    );

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("token.notFound")).toBeInTheDocument();
    expect(screen.getByText("token.tokenNotExist")).toBeInTheDocument();
    expect(screen.queryByText("loadFailed")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "retry" })).not.toBeInTheDocument();
  });

  it("shows server errors separately and retries them", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn<() => void>();
    render(<QueryErrorResult error={new Error("temporarily unavailable")} onRetry={onRetry} />, {
      wrapper: createTestWrapper(),
    });

    expect(screen.getByText("loadFailed")).toBeInTheDocument();
    expect(screen.getByText("Error: temporarily unavailable")).toBeInTheDocument();
    expect(screen.queryByText("404")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "retry" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("renders an accessible default NotFound result for missing data", () => {
    render(<NotFoundResult />, { wrapper: createTestWrapper() });

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("notFound")).toBeInTheDocument();
    expect(screen.getByText("resourceNotFound")).toBeInTheDocument();
  });
});
