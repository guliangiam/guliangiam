import { Button, Empty, Input, Select, Space, Switch, Table, Tag, Typography } from "antd";
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  PlusOutlined,
  DeleteOutlined,
} from "@ant-design/icons";
import { type FC, useCallback, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ProviderIcon } from "@/shared/ui/ProviderIcon";
import { ProviderCategoryBadge } from "@/shared/ui/ProviderCategoryBadge";
import { getGroupOptions } from "@/shared/api/iam/group";
import { queryKeys } from "@/shared/api/query-keys";

/**
 * 提供商下拉选项：除 label/value 外携带类型、分类与来源组织，
 * 供 optionRender/labelRender 渲染图标与来源标识；`suggested` 由调用方
 * （ProvidersTab 联动 signinMethods）计算。
 */
export type ProviderSelectOption = {
  label: string;
  value: string;
  type?: string;
  category?: string;
  orgName?: string;
  suggested?: boolean;
};

export type ProviderItemValue = {
  provider: string;
  canSignUp: boolean;
  canSignIn: boolean;
  canUnlink: boolean;
  alertType?: string;
  prompt?: string;
  bindingRule?: string[];
  countryCodes?: string[];
  prompted?: boolean;
  signupGroup?: string;
  rule?: string;
};

const ALERT_TYPES = [
  { label: "None", value: "" },
  { label: "Info", value: "info" },
  { label: "Warning", value: "warning" },
  { label: "Error", value: "error" },
];

// OAuth/SAML 类提供商支持账号绑定规则
const BINDING_RULE_OPTIONS = [
  { label: "Email", value: "Email" },
  { label: "Phone", value: "Phone" },
  { label: "Username", value: "Username" },
];

// 常用国家/地区代码（SMS 提供商可限定服务国家）
const COUNTRY_CODE_OPTIONS = [
  { label: "All", value: "All" },
  { label: "China (+86)", value: "CN" },
  { label: "United States (+1)", value: "US" },
  { label: "United Kingdom (+44)", value: "GB" },
  { label: "Singapore (+65)", value: "SG" },
  { label: "Germany (+49)", value: "DE" },
  { label: "France (+33)", value: "FR" },
  { label: "Japan (+81)", value: "JP" },
  { label: "India (+91)", value: "IN" },
  { label: "Australia (+61)", value: "AU" },
  { label: "Canada (+1)", value: "CA" },
];

// 可链接的提供商分类（OAuth/SAML 支持 canSignUp/canSignIn/canUnlink/prompted）
const LINKABLE_CATEGORIES = new Set(["OAuth", "SAML"]);

// 按提供商分类返回 rule 选项；无 rule 的分类返回 null
function getRuleOptions(category?: string) {
  switch (category) {
    case "Email":
    case "SMS":
      return [
        { label: "All", value: "all" },
        { label: "Signup only", value: "signup" },
        { label: "Signin only", value: "signin" },
      ];
    case "Captcha":
      return [
        { label: "Default", value: "default" },
        { label: "None", value: "none" },
      ];
    default:
      return null;
  }
}

// Email/SMS 提供商的默认 rule
function getDefaultRule(category?: string) {
  if (category === "Email" || category === "SMS") return "all";
  return "";
}

type Props = {
  value: ProviderItemValue[];
  onChange: (value: ProviderItemValue[]) => void;
  providerOptions?: ProviderSelectOption[];
  /** signupGroup 下拉的候选组来源组织（后端 signupGroup 存组名）。 */
  organizationId?: string;
  readOnly?: boolean;
};

const ProviderItemsEditor: FC<Props> = ({
  value,
  onChange,
  providerOptions = [],
  organizationId,
  readOnly,
}) => {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");

  // 关联现有数据：注册分组从该组织的组列表选择，不再手工输入。
  const { data: groupOptionsData } = useQuery({
    queryKey: [...queryKeys.groups.all, "options", organizationId ?? "all"],
    queryFn: () => getGroupOptions(organizationId),
    enabled: !readOnly,
    staleTime: 60_000,
  });

  const usedProviders = useMemo(
    () => new Set(value.map((item) => item.provider).filter(Boolean)),
    [value],
  );
  const providerValues = useMemo(
    () => new Set(providerOptions.map((option) => option.value)),
    [providerOptions],
  );

  // 通过 provider value 查找完整 provider 对象（含 category/type）
  const findProvider = useCallback(
    (providerValue: string) => providerOptions.find((option) => option.value === providerValue),
    [providerOptions],
  );

  /** 行级问题：必填、重复、未知提供商。 */
  const providerIssue = useCallback(
    (provider: string, index: number): string | undefined => {
      if (!provider) return t("application.providerEditor.required");
      if (value.some((item, i) => i !== index && item.provider === provider)) {
        return t("application.providerEditor.duplicate");
      }
      if (providerOptions.length > 0 && !providerValues.has(provider)) {
        return t("application.providerEditor.unknown");
      }
      return undefined;
    },
    [value, providerOptions.length, providerValues, t],
  );

  const handleAdd = useCallback(() => {
    onChange([
      ...value,
      {
        provider: "",
        canSignUp: false,
        canSignIn: false,
        canUnlink: false,
        alertType: "",
        prompt: "",
        bindingRule: [],
        countryCodes: [],
        prompted: false,
        signupGroup: "",
        rule: "",
      },
    ]);
  }, [value, onChange]);

  const handleRemove = useCallback(
    (index: number) => {
      const next = [...value];
      next.splice(index, 1);
      onChange(next);
    },
    [value, onChange],
  );

  // 排序即契约：providerItems 数组顺序就是渲染与保存顺序（绑定派生保序）。
  const handleMove = useCallback(
    (index: number, delta: -1 | 1) => {
      const target = index + delta;
      if (target < 0 || target >= value.length) return;
      const next = [...value];
      [next[index], next[target]] = [next[target], next[index]];
      onChange(next);
    },
    [value, onChange],
  );

  const handleChange = useCallback(
    (index: number, field: keyof ProviderItemValue, val: string | boolean | string[]) => {
      const next = [...value];
      next[index] = { ...next[index], [field]: val };
      onChange(next);
    },
    [value, onChange],
  );

  // 选择提供商时：自动设置默认 rule（Email/SMS → "all"）
  const handleProviderSelect = useCallback(
    (index: number, providerValue: string) => {
      const provider = findProvider(providerValue);
      const defaultRule = getDefaultRule(provider?.category);
      const next = [...value];
      next[index] = {
        ...next[index],
        provider: providerValue,
        rule: defaultRule || next[index].rule,
      };
      onChange(next);
    },
    [value, onChange, findProvider],
  );

  const boolCell = (index: number, field: "canSignUp" | "canSignIn" | "canUnlink" | "prompted") =>
    readOnly ? (
      value[index]?.[field] ? (
        tc("yes")
      ) : (
        tc("no")
      )
    ) : (
      <Switch
        size="small"
        checked={value[index]?.[field]}
        onChange={(val) => handleChange(index, field, val)}
      />
    );

  const signupGroupCell = (index: number) => {
    if (readOnly) return value[index]?.signupGroup || "-";
    return (
      <Select
        size="small"
        style={{ width: "100%" }}
        allowClear
        showSearch
        value={value[index]?.signupGroup || undefined}
        placeholder={t("application.providerEditor.columns.signupGroup")}
        onChange={(val) => handleChange(index, "signupGroup", (val as string | undefined) ?? "")}
        options={(groupOptionsData ?? []).map((group) => ({
          label: group.displayName || group.name,
          value: group.name,
        }))}
      />
    );
  };

  // 计算 Email/SMS 行的覆盖情况：被前序同分类行遮蔽则显示提示
  const coverageCell = (index: number) => {
    const provider = findProvider(value[index]?.provider ?? "");
    const category = provider?.category;
    if (category !== "Email" && category !== "SMS") return null;

    const rule = value[index]?.rule || "all";
    const isShadowed = value.some((other, i) => {
      if (i >= index) return false;
      const otherProvider = findProvider(other.provider);
      if (otherProvider?.category !== category) return false;
      const otherRule = other.rule || "all";
      return otherRule === rule || otherRule === "all";
    });

    if (isShadowed) {
      return (
        <Typography.Text type="warning" style={{ fontSize: 12 }}>
          {t("application.providerEditor.coverage.shadowed")}
        </Typography.Text>
      );
    }

    if (rule === "all") {
      return (
        <Space size={4} wrap>
          <Tag>{t("application.providerEditor.coverage.signin")}</Tag>
          <Tag>{t("application.providerEditor.coverage.signup")}</Tag>
        </Space>
      );
    }
    return (
      <Tag>
        {rule === "signin"
          ? t("application.providerEditor.coverage.signin")
          : t("application.providerEditor.coverage.signup")}
      </Tag>
    );
  };

  const columns = [
    ...(readOnly
      ? []
      : [
          {
            title: t("application.providerEditor.order"),
            width: 64,
            fixed: "left" as const,
            render: (_: unknown, __: ProviderItemValue, index: number) => (
              <Space size={0}>
                <Button
                  type="text"
                  size="small"
                  aria-label={t("application.providerEditor.moveUp")}
                  disabled={index === 0}
                  icon={<ArrowUpOutlined />}
                  onClick={() => handleMove(index, -1)}
                />
                <Button
                  type="text"
                  size="small"
                  aria-label={t("application.providerEditor.moveDown")}
                  disabled={index === value.length - 1}
                  icon={<ArrowDownOutlined />}
                  onClick={() => handleMove(index, 1)}
                />
              </Space>
            ),
          },
        ]),
    {
      title: t("application.providerEditor.columns.provider"),
      dataIndex: "provider",
      width: 260,
      fixed: "left" as const,
      render: (_: string, __: ProviderItemValue, index: number) => {
        if (readOnly) return value[index]?.provider;
        const provider = value[index]?.provider ?? "";
        const issue = providerIssue(provider, index);
        return (
          <div>
            <Select
              value={provider || undefined}
              onChange={(val) => handleProviderSelect(index, val)}
              placeholder={t("application.providerEditor.columns.provider")}
              size="small"
              style={{ width: "100%" }}
              showSearch
              status={issue ? "error" : undefined}
              options={providerOptions.map((option) => ({
                ...option,
                disabled: option.value !== provider && usedProviders.has(option.value),
              }))}
              optionRender={(option) => {
                const data = option.data as ProviderSelectOption;
                return (
                  <Space size={8} align="center" style={{ width: "100%" }}>
                    {data.type ? <ProviderIcon type={data.type} size={20} /> : null}
                    <span>{data.label}</span>
                    {data.category ? <ProviderCategoryBadge category={data.category} /> : null}
                    {data.orgName ? (
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        {t("application.providerEditor.originOrganization", { org: data.orgName })}
                      </Typography.Text>
                    ) : null}
                    {data.suggested ? (
                      <Typography.Text type="warning" style={{ fontSize: 12 }}>
                        {t("application.providerEditor.suggested")}
                      </Typography.Text>
                    ) : null}
                  </Space>
                );
              }}
              labelRender={(labelProps) => {
                const selected = providerOptions.find(
                  (option) => option.value === labelProps.value,
                );
                if (!selected?.type) return <span>{labelProps.label ?? labelProps.value}</span>;
                return (
                  <Space size={6} align="center">
                    <ProviderIcon type={selected.type} size={20} />
                    <span>{selected.label}</span>
                  </Space>
                );
              }}
            />
            {issue ? (
              <Typography.Text type="danger" style={{ fontSize: 12 }}>
                {issue}
              </Typography.Text>
            ) : null}
          </div>
        );
      },
    },
    {
      title: t("application.providerEditor.columns.category"),
      dataIndex: "provider",
      width: 100,
      render: (_: string, __: ProviderItemValue, index: number) => {
        const provider = findProvider(value[index]?.provider ?? "");
        return provider?.category ? <ProviderCategoryBadge category={provider.category} /> : "-";
      },
    },
    {
      title: t("application.providerEditor.columns.type"),
      dataIndex: "provider",
      width: 100,
      render: (_: string, __: ProviderItemValue, index: number) => {
        const provider = findProvider(value[index]?.provider ?? "");
        return provider?.type || "-";
      },
    },
    {
      title: t("application.providerEditor.columns.canSignUp"),
      dataIndex: "canSignUp",
      width: 96,
      render: (_: boolean, row: ProviderItemValue, index: number) => {
        const provider = findProvider(row.provider);
        return LINKABLE_CATEGORIES.has(provider?.category ?? "")
          ? boolCell(index, "canSignUp")
          : null;
      },
    },
    {
      title: t("application.providerEditor.columns.canSignIn"),
      dataIndex: "canSignIn",
      width: 96,
      render: (_: boolean, row: ProviderItemValue, index: number) => {
        const provider = findProvider(row.provider);
        return LINKABLE_CATEGORIES.has(provider?.category ?? "")
          ? boolCell(index, "canSignIn")
          : null;
      },
    },
    {
      title: t("application.providerEditor.columns.canUnlink"),
      dataIndex: "canUnlink",
      width: 96,
      render: (_: boolean, row: ProviderItemValue, index: number) => {
        const provider = findProvider(row.provider);
        return LINKABLE_CATEGORIES.has(provider?.category ?? "")
          ? boolCell(index, "canUnlink")
          : null;
      },
    },
    {
      title: t("application.providerEditor.columns.prompted"),
      dataIndex: "prompted",
      width: 96,
      render: (_: boolean, row: ProviderItemValue, index: number) => {
        const provider = findProvider(row.provider);
        return LINKABLE_CATEGORIES.has(provider?.category ?? "")
          ? boolCell(index, "prompted")
          : null;
      },
    },
    {
      title: t("application.providerEditor.columns.alertType"),
      dataIndex: "alertType",
      width: 110,
      render: (_: string, __: ProviderItemValue, index: number) =>
        readOnly ? (
          value[index]?.alertType || "-"
        ) : (
          <Select
            value={value[index]?.alertType || ""}
            onChange={(val) => handleChange(index, "alertType", val)}
            placeholder={t("application.providerEditor.columns.alertType")}
            size="small"
            style={{ width: "100%" }}
            options={ALERT_TYPES}
          />
        ),
    },
    {
      title: t("application.providerEditor.columns.prompt"),
      dataIndex: "prompt",
      width: 150,
      render: (_: string, __: ProviderItemValue, index: number) =>
        readOnly ? (
          value[index]?.prompt || "-"
        ) : (
          <Input
            value={value[index]?.prompt}
            onChange={(e) => handleChange(index, "prompt", e.target.value)}
            placeholder={t("application.providerEditor.columns.prompt")}
            size="small"
          />
        ),
    },
    {
      title: t("application.providerEditor.columns.bindingRule"),
      dataIndex: "bindingRule",
      width: 180,
      render: (_: string[], __: ProviderItemValue, index: number) => {
        const provider = findProvider(value[index]?.provider ?? "");
        if (!LINKABLE_CATEGORIES.has(provider?.category ?? "")) return null;
        if (readOnly) return (value[index]?.bindingRule ?? []).join(", ") || "-";
        return (
          <Select
            mode="multiple"
            size="small"
            style={{ width: "100%" }}
            placeholder={t("application.providerEditor.columns.bindingRule")}
            value={value[index]?.bindingRule ?? []}
            onChange={(val) => handleChange(index, "bindingRule", val as string[])}
            options={BINDING_RULE_OPTIONS}
            maxTagCount="responsive"
          />
        );
      },
    },
    {
      title: t("application.providerEditor.columns.countryCodes"),
      dataIndex: "countryCodes",
      width: 180,
      render: (_: string[], __: ProviderItemValue, index: number) => {
        const provider = findProvider(value[index]?.provider ?? "");
        if (provider?.category !== "SMS") return null;
        if (readOnly) return (value[index]?.countryCodes ?? []).join(", ") || "-";
        return (
          <Select
            mode="multiple"
            size="small"
            style={{ width: "100%" }}
            placeholder={t("application.providerEditor.columns.countryCodes")}
            value={value[index]?.countryCodes ?? []}
            onChange={(val) => handleChange(index, "countryCodes", val as string[])}
            options={COUNTRY_CODE_OPTIONS}
            maxTagCount="responsive"
          />
        );
      },
    },
    {
      title: t("application.providerEditor.columns.signupGroup"),
      dataIndex: "signupGroup",
      width: 160,
      render: (_: string, row: ProviderItemValue, index: number) => {
        const provider = findProvider(row.provider);
        if (provider?.category !== "OAuth") return null;
        return signupGroupCell(index);
      },
    },
    {
      title: t("application.providerEditor.columns.rule"),
      dataIndex: "rule",
      width: 140,
      render: (_: string, row: ProviderItemValue, index: number) => {
        const provider = findProvider(row.provider);
        const ruleOptions = getRuleOptions(provider?.category);
        if (!ruleOptions) return readOnly ? value[index]?.rule || "-" : null;
        if (readOnly) return value[index]?.rule || "-";
        return (
          <Select
            size="small"
            style={{ width: "100%" }}
            placeholder={t("application.providerEditor.columns.rule")}
            value={value[index]?.rule || undefined}
            onChange={(val) => handleChange(index, "rule", val)}
            options={ruleOptions}
          />
        );
      },
    },
    {
      title: t("application.providerEditor.columns.coverage"),
      dataIndex: "rule",
      width: 160,
      render: (_: string, __: ProviderItemValue, index: number) => coverageCell(index),
    },
    ...(readOnly
      ? []
      : [
          {
            title: "",
            width: 48,
            fixed: "right" as const,
            render: (_: unknown, __: ProviderItemValue, index: number) => (
              <Button
                type="text"
                danger
                size="small"
                icon={<DeleteOutlined />}
                aria-label={t("application.providerEditor.remove")}
                onClick={() => handleRemove(index)}
              />
            ),
          },
        ]),
  ];

  return (
    <Space orientation="vertical" style={{ width: "100%" }}>
      <Table
        dataSource={value.map((item, i) => ({ ...item, _idx: i }))}
        columns={columns}
        pagination={false}
        size="small"
        rowKey="_idx"
        scroll={{ x: "max-content" }}
        locale={{
          emptyText: (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={
                <Space orientation="vertical" size={2}>
                  <span>{t("application.providerEditor.empty")}</span>
                  <Typography.Text type="secondary">
                    {t("application.providerEditor.emptyHint")}
                  </Typography.Text>
                </Space>
              }
            />
          ),
        }}
      />
      {!readOnly && (
        <Button type="dashed" onClick={handleAdd} icon={<PlusOutlined />} size="small">
          {t("application.providerEditor.add")}
        </Button>
      )}
    </Space>
  );
};

export default ProviderItemsEditor;
export { ProviderItemsEditor };
