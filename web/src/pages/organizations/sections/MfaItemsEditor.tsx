/**
 * MFA 方法编辑器。
 *
 * - 方法不可重复；`mfaRequired=true` 时至少一个方法必须是 Required；
 * - 支持上移/下移调整服务端返回顺序；
 * - 每个方法给出业务前提说明，避免用户配置出「不可用的必需项」。
 */
import { Alert, Button, Col, Form, Row, Select, Space, Typography } from "antd";
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  DeleteOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import { useTranslation } from "react-i18next";

const MFA_METHODS = ["SMS", "Email", "TOTP", "RADIUS", "Push", "WebAuthn"] as const;

type MfaItem = { name?: string; rule?: string };

export function MfaItemsEditor() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const form = Form.useFormInstance();
  const items = (Form.useWatch("mfaItems", form) ?? []) as MfaItem[];
  const mfaRequired = Boolean(Form.useWatch("mfaRequired", form));

  const usedMethods = new Set(items.map((item) => item.name).filter(Boolean) as string[]);
  const requiredCount = items.filter((item) => item.rule === "Required").length;
  const missingRequired = mfaRequired && requiredCount === 0;

  const move = (index: number, direction: -1 | 1) => {
    const next = [...items];
    const target = index + direction;
    if (target < 0 || target >= next.length) return;
    [next[index], next[target]] = [next[target], next[index]];
    form.setFieldValue("mfaItems", next);
  };

  const methodOptions = MFA_METHODS.map((value) => ({
    value,
    label: tr(`organization.mfaMethods.${value}`),
  }));

  const ruleOptions = [
    { label: tr("organization.mfaRequiredOn"), value: "Required" },
    { label: tr("organization.mfaRequiredOff"), value: "Optional" },
    { label: t("disabled"), value: "Disabled" },
  ];

  return (
    <Form.List name="mfaItems">
      {(fields, { add, remove }) => (
        <Space orientation="vertical" style={{ width: "100%" }} size={12}>
          <div>
            <Typography.Text strong>{tr("organization.mfaItems")}</Typography.Text>
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
              {tr("organization.mfaItemsHint")}
            </Typography.Paragraph>
          </div>

          {missingRequired ? (
            <Alert type="warning" showIcon title={tr("organization.mfaMissingRequired")} />
          ) : null}

          {fields.map((field, index) => {
            const currentName = items[index]?.name;
            return (
              <Row
                key={field.key}
                gutter={12}
                align="top"
                style={{
                  border: "1px solid var(--ant-color-border-secondary)",
                  borderRadius: 8,
                  padding: 12,
                }}
              >
                <Col xs={24} md={8}>
                  <Form.Item
                    name={[field.name, "name"]}
                    rules={[
                      { required: true, message: t("required") },
                      {
                        validator: async (_rule, value: string) => {
                          if (!value) return;
                          const duplicate = items.some(
                            (item, itemIndex) => itemIndex !== index && item.name === value,
                          );
                          if (duplicate) {
                            throw new Error(tr("organization.mfaDuplicateMethod"));
                          }
                        },
                      },
                    ]}
                    style={{ marginBottom: 0 }}
                  >
                    <Select
                      placeholder={t("placeholder.selectMfaMethod")}
                      options={methodOptions.map((option) => ({
                        ...option,
                        disabled: usedMethods.has(option.value) && option.value !== currentName,
                      }))}
                    />
                  </Form.Item>
                  {currentName ? (
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      {tr(`organization.mfaMethodHelp.${currentName}`)}
                    </Typography.Text>
                  ) : null}
                </Col>
                <Col xs={24} md={8}>
                  <Form.Item name={[field.name, "rule"]} style={{ marginBottom: 0 }}>
                    <Select allowClear placeholder={t("rulePlaceholder")} options={ruleOptions} />
                  </Form.Item>
                </Col>
                <Col xs={24} md={8}>
                  <Space>
                    <Button
                      icon={<ArrowUpOutlined />}
                      size="small"
                      disabled={index === 0}
                      onClick={() => move(index, -1)}
                    />
                    <Button
                      icon={<ArrowDownOutlined />}
                      size="small"
                      disabled={index === fields.length - 1}
                      onClick={() => move(index, 1)}
                    />
                    <Button
                      danger
                      size="small"
                      icon={<DeleteOutlined />}
                      aria-label={tr("organization.removeMfaItem")}
                      title={tr("organization.removeMfaItem")}
                      onClick={() => remove(field.name)}
                    />
                  </Space>
                </Col>
              </Row>
            );
          })}

          <Button
            icon={<PlusOutlined />}
            onClick={() => add({ name: "", rule: "Optional" })}
            disabled={items.length >= MFA_METHODS.length}
          >
            {tr("organization.addMfaItem")}
          </Button>
        </Space>
      )}
    </Form.List>
  );
}
