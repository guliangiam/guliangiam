import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RegionSelect } from "./RegionSelect";

const i18nState = vi.hoisted(() => ({ language: "en" }));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    i18n: { language: i18nState.language },
    t: (key: string) => key,
  }),
}));

const COUNTRIES = ["US", "CN"];

beforeEach(() => {
  i18nState.language = "en";
});

describe("RegionSelect", () => {
  it("does not fall back to defaultValue for a controlled empty string", () => {
    const { container } = render(
      <RegionSelect value="" defaultValue="US" countryCodes={COUNTRIES} />,
    );

    expect(container).not.toHaveTextContent("United States");
    expect(container.querySelector(".ant-select-content")).toHaveAttribute("title", "");
  });

  it("supports clearing and localizes options without a flag CDN", async () => {
    const onChange = vi.fn<(value: string) => void>();
    const user = userEvent.setup();
    const { container, rerender } = render(
      <RegionSelect
        ariaLabel="Region"
        value="US"
        countryCodes={COUNTRIES}
        allowClear
        onChange={onChange}
      />,
    );

    fireEvent.click(container.querySelector(".ant-select-clear")!);
    expect(onChange).toHaveBeenLastCalledWith("");

    rerender(<RegionSelect ariaLabel="Region" value="US" countryCodes={COUNTRIES} allowClear />);
    await user.click(screen.getByRole("combobox", { name: "Region" }));
    expect((await screen.findAllByText("United States of America (US)")).length).toBeGreaterThan(0);
    expect(document.querySelector('img[src*="cdn.casbin.org"]')).not.toBeInTheDocument();

    i18nState.language = "zh";
    rerender(<RegionSelect ariaLabel="Region" value="US" countryCodes={COUNTRIES} allowClear />);
    expect((await screen.findAllByText("美国 (US)")).length).toBeGreaterThan(0);
  });
});
