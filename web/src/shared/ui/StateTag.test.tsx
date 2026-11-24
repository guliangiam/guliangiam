import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { StateTag } from "./StateTag";

// A miniature caption pack standing in for the real ones, which
// `scripts/check-enums.py` proves carry `enumValues.<value>` for every declared
// value. Having the mock resolve only two keys is what lets the test show both
// branches: a key that resolves, and a key that does not.
vi.mock("react-i18next", () => {
  const pack = new Set(["enumValues.paid", "enumValues.payment_pending"]);
  return {
    useTranslation: () => ({
      t: (key: string, opts?: { defaultValue?: string }) =>
        pack.has(key) ? key.replace("enumValues.", "") : (opts?.defaultValue ?? key),
    }),
  };
});

describe("StateTag", () => {
  it("captions a declared value from the registry key", () => {
    render(<StateTag value="payment_pending" />);
    expect(screen.getByText("payment_pending")).toBeInTheDocument();
  });

  it("falls back to the raw value for a state this build does not know", () => {
    render(<StateTag value="some_future_state" />);
    expect(screen.getByText("some_future_state")).toBeInTheDocument();
  });

  it("renders a dash for an absent value", () => {
    const { container } = render(<StateTag value={null} />);
    expect(container.textContent).toBe("-");
  });
});
