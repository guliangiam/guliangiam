import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CountryCodeSelect } from "./CountryCodeSelect";

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

describe("CountryCodeSelect", () => {
  it("rebuilds country and calling-code options after the language changes", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <CountryCodeSelect ariaLabel="Country code" countryCodes={COUNTRIES} value="CN" />,
    );

    await user.click(screen.getByRole("combobox", { name: "Country code" }));
    expect(await screen.findByText("People's Republic of China")).toBeInTheDocument();

    i18nState.language = "zh-CN";
    rerender(<CountryCodeSelect ariaLabel="Country code" countryCodes={COUNTRIES} value="CN" />);

    expect(await screen.findByText("中国")).toBeInTheDocument();
    expect(screen.queryByText("People's Republic of China")).not.toBeInTheDocument();
    expect(screen.getAllByText("+86").length).toBeGreaterThan(0);
    expect(document.querySelector('img[src*="cdn.casbin.org"]')).not.toBeInTheDocument();
  });

  it("synchronizes a later initValue without overriding controlled empty values", () => {
    const onChange = vi.fn<(value: string) => void>();
    const { container, rerender } = render(
      <CountryCodeSelect countryCodes={COUNTRIES} initValue="US" onChange={onChange} allowClear />,
    );
    expect(container).toHaveTextContent("+1");

    rerender(<CountryCodeSelect countryCodes={COUNTRIES} initValue="CN" onChange={onChange} />);
    expect(container).toHaveTextContent("+86");

    rerender(
      <CountryCodeSelect
        countryCodes={COUNTRIES}
        initValue="US"
        value="CN"
        onChange={onChange}
        allowClear
      />,
    );
    fireEvent.click(container.querySelector(".ant-select-clear")!);
    expect(onChange).toHaveBeenLastCalledWith("");

    rerender(
      <CountryCodeSelect
        countryCodes={COUNTRIES}
        initValue="US"
        value=""
        onChange={onChange}
        allowClear
      />,
    );
    expect(container).not.toHaveTextContent("+1");
    expect(container).not.toHaveTextContent("+86");
  });

  it.each(["multiple", "tags"] as const)(
    "keeps %s mode values and onChange payloads as arrays",
    async (mode) => {
      const onChange = vi.fn<(value: string[]) => void>();
      const user = userEvent.setup();
      render(
        <CountryCodeSelect
          ariaLabel="Country codes"
          countryCodes={COUNTRIES}
          mode={mode}
          value={["US"]}
          onChange={onChange}
        />,
      );

      await user.click(screen.getByRole("combobox", { name: "Country codes" }));
      await user.click(await screen.findByText("People's Republic of China"));

      expect(onChange).toHaveBeenLastCalledWith(["US", "CN"]);
    },
  );
});
