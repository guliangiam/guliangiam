import { Divider, Select, Space } from "antd";
import type { SelectProps } from "antd";
import { InlineProviderCreateButton } from "./InlineProviderCreateButton";

export type ProviderOption = {
  id?: string;
  displayName?: string;
  name?: string;
};

type ProviderSelectProps = Omit<SelectProps<string | string[]>, "options"> & {
  providers: ProviderOption[] | undefined;
  /**
   * When true, shows a "+ New Provider" footer at the bottom of the
   * dropdown. Clicking it opens an inline create modal.
   */
  showCreateOption?: boolean;
  /** Called after a provider is created via the footer option. */
  onCreate?: (id: string) => void;
};

/**
 * Shared provider picker. It centralises the label fallback (displayName →
 * name → id) and the search behaviour so user identity, application bindings
 * and other provider consumers cannot drift apart.
 */
export function ProviderSelect({
  providers,
  mode,
  showCreateOption,
  onCreate,
  ...rest
}: ProviderSelectProps) {
  const options = (providers ?? [])
    .filter((provider) => typeof provider.id === "string" && provider.id !== "")
    .map((provider) => ({
      label: provider.displayName ?? provider.name ?? provider.id!,
      value: provider.id!,
    }));

  if (!showCreateOption) {
    return (
      <Select
        {...rest}
        mode={mode}
        options={options}
        showSearch
        optionFilterProp="label"
        allowClear={rest.allowClear ?? mode === "multiple"}
      />
    );
  }

  return (
    <Select
      {...rest}
      mode={mode}
      options={options}
      showSearch
      optionFilterProp="label"
      allowClear={rest.allowClear ?? mode === "multiple"}
      popupRender={(menu) => (
        <>
          {menu}
          <Divider style={{ margin: "4px 0" }} />
          <Space style={{ padding: "4px 12px" }}>
            <InlineProviderCreateButton
              variant="link"
              buttonText="+ New Provider"
              onCreated={(id) => {
                onCreate?.(id);
              }}
            />
          </Space>
        </>
      )}
    />
  );
}
