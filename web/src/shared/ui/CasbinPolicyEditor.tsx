import { Table, Button, Input, Select, Space, Popconfirm, Form, Modal } from "antd";
import {
  PlusOutlined,
  DeleteOutlined,
  CheckOutlined,
  CloseOutlined,
  EditOutlined,
} from "@ant-design/icons";
import { useState, useCallback, memo } from "react";
import { useTranslation } from "react-i18next";

export type Policy = {
  ptype: string;
  subject: string;
  object: string;
  action: string;
};

type CasbinPolicyEditorProps = {
  /** Initial policy rows */
  policies: Policy[];
  /** Callback when a policy is added */
  onAdd: (policy: Policy) => void;
  /** Callback when a policy is removed by index */
  onRemove: (index: number) => void;
  /** Callback when a policy is updated by index */
  onUpdate: (index: number, policy: Policy) => void;
  /** Whether the editor is loading */
  loading?: boolean;
};

const PTYPE_OPTIONS = [
  { label: "p (Policy)", value: "p" },
  { label: "g (Grouping)", value: "g" },
];

/**
 * CasbinPolicyEditor provides an inline-editable table for managing
 * Casbin policy rules with ptype, subject, object, action fields.
 */
export const CasbinPolicyEditor = memo(function CasbinPolicyEditor({
  policies,
  onAdd,
  onRemove,
  onUpdate,
  loading = false,
}: CasbinPolicyEditorProps) {
  const { t } = useTranslation("resource");
  const [modalOpen, setModalOpen] = useState(false);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [editValues, setEditValues] = useState<Policy | null>(null);
  const [form] = Form.useForm();

  const handleOpenAdd = useCallback(() => {
    setModalOpen(true);
  }, []);

  const handleModalOk = useCallback(async () => {
    const values = await form.validateFields();
    onAdd(values as Policy);
    setModalOpen(false);
    form.resetFields();
  }, [form, onAdd]);

  const handleModalCancel = useCallback(() => {
    setModalOpen(false);
    form.resetFields();
  }, [form]);

  const handleStartEdit = useCallback(
    (index: number) => {
      const policy = policies[index];
      setEditingIndex(index);
      setEditValues({ ...policy });
    },
    [policies],
  );

  const handleSaveEdit = useCallback(() => {
    if (editingIndex !== null && editValues) {
      onUpdate(editingIndex, editValues);
      setEditingIndex(null);
      setEditValues(null);
    }
  }, [editingIndex, editValues, onUpdate]);

  const handleCancelEdit = useCallback(() => {
    setEditingIndex(null);
    setEditValues(null);
  }, []);

  const handleEditFieldChange = useCallback((field: keyof Policy, value: string) => {
    setEditValues((prev) => (prev ? { ...prev, [field]: value } : null));
  }, []);

  const columns = [
    {
      title: "Type",
      dataIndex: "ptype",
      width: "15%",
      render: (ptype: string, _: Policy, index: number) =>
        editingIndex === index ? (
          <Select
            value={editValues?.ptype}
            onChange={(v) => handleEditFieldChange("ptype", v)}
            options={PTYPE_OPTIONS}
            size="small"
            style={{ width: "100%" }}
          />
        ) : ptype === "g" ? (
          "g (Grouping)"
        ) : (
          "p (Policy)"
        ),
    },
    {
      title: "Subject",
      dataIndex: "subject",
      width: "25%",
      render: (subject: string, _: Policy, index: number) =>
        editingIndex === index ? (
          <Input
            value={editValues?.subject}
            onChange={(e) => handleEditFieldChange("subject", e.target.value)}
            placeholder="user/role/*"
            size="small"
          />
        ) : (
          subject
        ),
    },
    {
      title: "Object",
      dataIndex: "object",
      width: "25%",
      render: (object: string, _: Policy, index: number) =>
        editingIndex === index ? (
          <Input
            value={editValues?.object}
            onChange={(e) => handleEditFieldChange("object", e.target.value)}
            placeholder="resource"
            size="small"
          />
        ) : (
          object
        ),
    },
    {
      title: "Action",
      dataIndex: "action",
      width: "20%",
      render: (action: string, _: Policy, index: number) =>
        editingIndex === index ? (
          <Input
            value={editValues?.action}
            onChange={(e) => handleEditFieldChange("action", e.target.value)}
            placeholder="read/write"
            size="small"
          />
        ) : (
          action
        ),
    },
    {
      title: "",
      width: "15%",
      render: (_: unknown, __: Policy, index: number) =>
        editingIndex === index ? (
          <Space>
            <Button type="text" icon={<CheckOutlined />} onClick={handleSaveEdit} size="small" />
            <Button type="text" icon={<CloseOutlined />} onClick={handleCancelEdit} size="small" />
          </Space>
        ) : (
          <Space>
            <Button
              type="text"
              icon={<EditOutlined />}
              onClick={() => handleStartEdit(index)}
              size="small"
            />
            <Popconfirm
              title={t("policy.deleteConfirm")}
              onConfirm={() => onRemove(index)}
              okText="Yes"
              cancelText="No"
            >
              <Button type="text" danger icon={<DeleteOutlined />} size="small" />
            </Popconfirm>
          </Space>
        ),
    },
  ];

  return (
    <div className="gl-casbin-policy-editor">
      <div className="gl-casbin-policy-editor__toolbar">
        <Button type="dashed" icon={<PlusOutlined />} onClick={handleOpenAdd} loading={loading}>
          {t("policy.addRow")}
        </Button>
      </div>
      <Table
        columns={columns}
        dataSource={policies.map((item, i) => ({ ...item, key: i }))}
        pagination={false}
        size="small"
        bordered
        loading={loading}
      />
      <Modal
        destroyOnHidden
        title={t("common:policy.addTitle")}
        open={modalOpen}
        onOk={handleModalOk}
        onCancel={handleModalCancel}
        okText="Add"
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{ ptype: "p", subject: "", object: "", action: "" }}
        >
          <Form.Item
            label="Type"
            name="ptype"
            rules={[{ required: true, message: "Please select policy type" }]}
          >
            <Select options={PTYPE_OPTIONS} />
          </Form.Item>
          <Form.Item
            label="Subject"
            name="subject"
            rules={[{ required: true, message: "Please enter subject" }]}
          >
            <Input placeholder="user/role/*" />
          </Form.Item>
          <Form.Item
            label="Object"
            name="object"
            rules={[{ required: true, message: "Please enter object" }]}
          >
            <Input placeholder="resource" />
          </Form.Item>
          <Form.Item
            label="Action"
            name="action"
            rules={[{ required: true, message: "Please enter action" }]}
          >
            <Input placeholder="read/write" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
});
