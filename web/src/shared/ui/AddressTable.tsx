import {
  App,
  Button,
  Form,
  Input,
  Modal,
  Row,
  Col,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
} from "antd";
import { DeleteOutlined, EditOutlined, PlusOutlined } from "@ant-design/icons";
import { type FC, useCallback, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { AddressItem } from "@/shared/api/iam/user";
import { CountryCodeSelect } from "./CountryCodeSelect";
import { getCountryCodeData } from "./country-data";

type Props = {
  value: AddressItem[];
  onChange: (value: AddressItem[]) => void;
  readOnly?: boolean;
};

type AddressFormValues = {
  countryCode?: string;
  province?: string;
  city?: string;
  district?: string;
  postalCode?: string;
  street: string;
  details?: string;
  isDefault?: boolean;
};

/**
 * Address editor.
 *
 * One address is a flat record with a strict hierarchy:
 * countryCode → province/state → city → district → street → postal code.
 * The legacy `region` free-text column is never written by this editor; it is
 * only shown as a fallback for rows created before the hierarchy existed.
 */
const AddressTable: FC<Props> = ({ value, onChange, readOnly }) => {
  const { t, i18n } = useTranslation();
  const { message } = App.useApp();
  const [form] = Form.useForm<AddressFormValues>();
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [open, setOpen] = useState(false);

  const countryNames = useMemo(() => {
    const language = i18n.resolvedLanguage ?? i18n.language;
    return new Map(getCountryCodeData(undefined, language).map((item) => [item.code, item.name]));
  }, [i18n.resolvedLanguage, i18n.language]);

  const openCreate = useCallback(() => {
    form.resetFields();
    form.setFieldsValue({ isDefault: value.length === 0 });
    setEditingIndex(null);
    setOpen(true);
  }, [form, value.length]);

  const openEdit = useCallback(
    (index: number) => {
      form.resetFields();
      form.setFieldsValue({ ...value[index] });
      setEditingIndex(index);
      setOpen(true);
    },
    [form, value],
  );

  const handleRemove = useCallback(
    (index: number) => {
      const next = [...value];
      next.splice(index, 1);
      onChange(next);
    },
    [value, onChange],
  );

  const handleSubmit = useCallback(async () => {
    const values = await form.validateFields();
    // Hierarchy fields supersede the legacy free-text region, but a legacy row
    // with no hierarchy yet must keep its region instead of silently losing it.
    const hasHierarchy = Boolean(values.province || values.city || values.district);
    const address: AddressItem = {
      ...values,
      region: hasHierarchy ? undefined : value[editingIndex ?? -1]?.region,
    };
    const next = [...value];
    if (address.isDefault) {
      // Exactly one default address is allowed by the backend validator.
      for (let i = 0; i < next.length; i += 1) {
        next[i] = { ...next[i], isDefault: false };
      }
    }
    if (editingIndex === null) {
      next.push(address);
    } else {
      // Preserve the opaque row id so the backend can keep stable identities.
      next[editingIndex] = { ...next[editingIndex], ...address };
    }
    onChange(next);
    setOpen(false);
    message.success(t("resource:user.address.saved", { defaultValue: "Address saved" }));
  }, [editingIndex, form, message, onChange, t, value]);

  const columns = [
    {
      title: t("resource:user.address.country", { defaultValue: "Country" }),
      dataIndex: "countryCode",
      render: (code?: string) => (code ? `${countryNames.get(code) ?? code} (${code})` : "-"),
    },
    {
      title: t("resource:user.address.region", { defaultValue: "Province / City" }),
      render: (_: unknown, record: AddressItem) =>
        [record.province, record.city, record.district].filter(Boolean).join(" / ") ||
        record.region ||
        "-",
    },
    {
      title: t("resource:user.address.street", { defaultValue: "Street" }),
      dataIndex: "street",
    },
    {
      title: t("resource:user.address.postalCode", { defaultValue: "Postal code" }),
      dataIndex: "postalCode",
      render: (postal?: string) => postal || "-",
    },
    {
      title: t("resource:user.address.isDefault", { defaultValue: "Default" }),
      dataIndex: "isDefault",
      width: 100,
      render: (isDefault?: boolean) =>
        isDefault ? (
          <Tag color="success">
            {t("resource:user.address.defaultTag", { defaultValue: "Default" })}
          </Tag>
        ) : null,
    },
    ...(readOnly
      ? []
      : [
          {
            title: "",
            width: 96,
            render: (_: unknown, __: AddressItem, index: number) => (
              <Space size={0}>
                <Tooltip title={t("common:edit", { defaultValue: "Edit" })}>
                  <Button
                    type="text"
                    size="small"
                    icon={<EditOutlined />}
                    aria-label={t("common:edit", { defaultValue: "Edit" })}
                    onClick={() => openEdit(index)}
                  />
                </Tooltip>
                <Tooltip title={t("common:delete", { defaultValue: "Delete" })}>
                  <Button
                    type="text"
                    danger
                    size="small"
                    icon={<DeleteOutlined />}
                    aria-label={t("common:delete", { defaultValue: "Delete" })}
                    onClick={() => handleRemove(index)}
                  />
                </Tooltip>
              </Space>
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
        locale={{ emptyText: t("resource:user.address.empty", { defaultValue: "No addresses" }) }}
      />
      {!readOnly && (
        <Button type="dashed" onClick={openCreate} icon={<PlusOutlined />} size="small">
          {t("resource:user.address.add", { defaultValue: "Add address" })}
        </Button>
      )}

      <Modal
        open={open}
        title={
          editingIndex === null
            ? t("resource:user.address.add", { defaultValue: "Add address" })
            : t("resource:user.address.edit", { defaultValue: "Edit address" })
        }
        onCancel={() => setOpen(false)}
        onOk={handleSubmit}
        forceRender
        okText={t("common:save", { defaultValue: "Save" })}
        cancelText={t("common:cancel", { defaultValue: "Cancel" })}
      >
        <Form form={form} layout="vertical" requiredMark="optional">
          <Form.Item
            label={t("resource:user.address.country", { defaultValue: "Country" })}
            name="countryCode"
            rules={[
              {
                required: true,
                message: t("resource:user.address.countryRequired", {
                  defaultValue: "Select a country",
                }),
              },
            ]}
          >
            <CountryCodeSelect
              style={{ width: "100%" }}
              id="address-country-code"
              ariaLabel={t("resource:user.address.country", { defaultValue: "Country" })}
            />
          </Form.Item>
          <Form.Item
            label={t("resource:user.address.province", { defaultValue: "Province / State" })}
            name="province"
          >
            <Input
              placeholder={t("resource:user.address.provincePlaceholder", {
                defaultValue: "e.g. Shanghai",
              })}
            />
          </Form.Item>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                label={t("resource:user.address.city", { defaultValue: "City" })}
                name="city"
              >
                <Input />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label={t("resource:user.address.district", { defaultValue: "District" })}
                name="district"
              >
                <Input />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item
            label={t("resource:user.address.street", { defaultValue: "Street" })}
            name="street"
            rules={[
              {
                required: true,
                whitespace: true,
                message: t("resource:user.address.streetRequired", {
                  defaultValue: "Street is required",
                }),
              },
            ]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            label={t("resource:user.address.details", { defaultValue: "Building / Room" })}
            name="details"
          >
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item
            label={t("resource:user.address.postalCode", { defaultValue: "Postal code" })}
            name="postalCode"
            rules={[
              {
                pattern: /^[A-Za-z0-9 -]{3,12}$/,
                message: t("resource:user.address.postalCodeInvalid", {
                  defaultValue: "Enter a valid postal code",
                }),
              },
            ]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            label={t("resource:user.address.isDefault", { defaultValue: "Default address" })}
            name="isDefault"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  );
};

export default AddressTable;
