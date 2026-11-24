import { useRef } from "react";
import { AutoComplete, Button, Form, Input, Popconfirm, Space, Table, Typography } from "antd";
import type { FormInstance } from "antd";
import { useTranslation } from "react-i18next";
import type { ApplicationCustomScope } from "@/shared/api/iam/application";
import {
  CUSTOM_SCOPE_PRESETS,
  customScopePresetDescriptionKey,
  customScopePresetNameKey,
  findCustomScopePreset,
} from "../application-oidc-validation";

interface CustomScopesTabProps {
  form: FormInstance;
}

export default function CustomScopesTab({ form }: CustomScopesTabProps) {
  const { t: tr } = useTranslation("resource");
  const { t: tc } = useTranslation("common");

  // 行 key 与对象身份绑定：编辑时原地改字段、只有数组浅拷贝，同一行的 key 不变，
  // 输入框不会被卸载重建（不会丢焦点/光标）。
  const rowKeysRef = useRef(new WeakMap<object, string>());
  const rowSeqRef = useRef(0);
  const keyOf = (record: ApplicationCustomScope) => {
    let key = rowKeysRef.current.get(record);
    if (!key) {
      rowSeqRef.current += 1;
      key = `scope-row-${rowSeqRef.current}`;
      rowKeysRef.current.set(record, key);
    }
    return key;
  };

  // 预置作用域：既可直接选择（自动补全显示名/描述），也可手动输入自定义值。
  const presetOptions = CUSTOM_SCOPE_PRESETS.map((preset) => ({
    value: preset.scope,
    label: tr(customScopePresetNameKey(preset.scope)),
  }));

  return (
    <>
      {/* 无 UI 的注册项：让 customScopes 进入 validateFields()/onFinish 的 values，保存链路才能读到真实数组。 */}
      <Form.Item name="customScopes" noStyle />
      <Form.Item noStyle shouldUpdate={(prev, cur) => prev.customScopes !== cur.customScopes}>
        {({ getFieldValue }) => {
          const customScopes: ApplicationCustomScope[] =
            (getFieldValue("customScopes") as ApplicationCustomScope[] | undefined) ?? [];

          const commit = (next: ApplicationCustomScope[]) => {
            // 必须用 setFieldValue 而非 setFieldsValue：后者会 deep-merge 深拷贝数组元素，
            // 行对象身份每次都变，WeakMap 的 key 就会跟着变、输入框随之重挂载丢焦点。
            form.setFieldValue("customScopes", [...next]);
          };

          const handleAdd = () => {
            commit([...customScopes, { scope: "", displayName: "", description: "" }]);
          };

          const handleRemove = (index: number) => {
            const next = [...customScopes];
            next.splice(index, 1);
            commit(next);
          };

          // 选择/输入预置作用域时补全显示名/描述；用户已填写的内容不覆盖。
          const fillPresetDefaults = (row: ApplicationCustomScope, scope: string): boolean => {
            const preset = findCustomScopePreset(scope);
            if (!preset) return false;
            let changed = false;
            if (!row.displayName?.trim()) {
              row.displayName = tr(customScopePresetNameKey(preset));
              changed = true;
            }
            if (!row.description?.trim()) {
              row.description = tr(customScopePresetDescriptionKey(preset));
              changed = true;
            }
            return changed;
          };

          const handleFieldChange = (
            index: number,
            field: keyof ApplicationCustomScope,
            value: string,
          ) => {
            const next = [...customScopes];
            const row = next[index];
            if (!row) return;
            row[field] = value;
            commit(next);
          };

          const handlePresetSelect = (index: number, value: string) => {
            const row = customScopes[index];
            if (!row) return;
            if (fillPresetDefaults(row, value)) {
              commit([...customScopes]);
            }
          };

          const columns = [
            {
              title: tr("application.customScopesSection.scope"),
              dataIndex: "scope",
              key: "scope",
              width: "25%",
              render: (v: string, _: ApplicationCustomScope, i: number) => (
                <AutoComplete
                  value={v}
                  options={presetOptions}
                  onChange={(value) => handleFieldChange(i, "scope", value)}
                  onSelect={(value: string) => handlePresetSelect(i, value)}
                  onBlur={() => handlePresetSelect(i, customScopes[i]?.scope ?? "")}
                  filterOption={(input, option) =>
                    String(option?.value ?? "")
                      .toLowerCase()
                      .includes(input.toLowerCase())
                  }
                  placeholder="e.g. read:reports"
                  size="small"
                  style={{ width: "100%" }}
                />
              ),
            },
            {
              title: tr("application.customScopesSection.displayName"),
              dataIndex: "displayName",
              key: "displayName",
              width: "25%",
              render: (v: string, _: ApplicationCustomScope, i: number) => (
                <Input
                  autoComplete="off"
                  value={v}
                  onChange={(e) => handleFieldChange(i, "displayName", e.target.value)}
                  placeholder="e.g. Read Reports"
                  size="small"
                />
              ),
            },
            {
              title: tr("application.customScopesSection.description"),
              dataIndex: "description",
              key: "description",
              render: (v: string, _: ApplicationCustomScope, i: number) => (
                <Input
                  autoComplete="off"
                  value={v}
                  onChange={(e) => handleFieldChange(i, "description", e.target.value)}
                  placeholder={tc("scopeDescription")}
                  size="small"
                />
              ),
            },
            {
              title: "",
              key: "actions",
              width: 80,
              render: (_: unknown, __: ApplicationCustomScope, i: number) => (
                <Popconfirm
                  title={tr("application.customScopesSection.deleteConfirm")}
                  onConfirm={() => handleRemove(i)}
                >
                  <Button size="small" danger>
                    {tc("delete")}
                  </Button>
                </Popconfirm>
              ),
            },
          ];

          return (
            <Space orientation="vertical" style={{ width: "100%" }}>
              <Button size="small" onClick={handleAdd}>
                {tr("application.customScopesSection.add")}
              </Button>
              <Table
                dataSource={customScopes}
                columns={columns}
                pagination={false}
                size="small"
                rowKey={(record) => keyOf(record)}
              />
              <Typography.Text type="secondary">
                {tr("application.customScopesSection.presetHint")}
              </Typography.Text>
            </Space>
          );
        }}
      </Form.Item>
    </>
  );
}
