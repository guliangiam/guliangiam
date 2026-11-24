import { Button, Form, Input, Modal, Popconfirm, Select, Space, Table } from "antd";
import {
  CheckOutlined,
  CloseOutlined,
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import { useCallback, useState, memo } from "react";

export type Policy = {
  ptype: string;
  subject: string;
  object: string;
  action: string;
};

type Props = {
  policies: Policy[];
  onAdd: (policy: Policy) => void;
  onRemove: (index: number) => void;
  onUpdate: (index: number, policy: Policy) => void;
  loading?: boolean;
};

const PTYPE_OPTIONS = [
  { label: "p (Policy)", value: "p" },
  { label: "g (Grouping)", value: "g" },
];

export const CasbinPolicyEditor = memo(function CasbinPolicyEditor({
  policies,
  onAdd,
  onRemove,
  onUpdate,
  loading,
}: Props) {
  const [modalOpen, setModalOpen] = useState(false);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [editValues, setEditValues] = useState<Policy | null>(null);
  const [form] = Form.useForm();

  // ── Add modal handlers ──────────────────────────────
  const handleOpenAdd = useCallback(() => {
    form.resetFields();
    form.setFieldsValue({ ptype: "p", subject: "", object: "", action: "" });
    setModalOpen(true);
  }, [form]);

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

  // ── Inline editing handlers ─────────────────────────
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

  // ── Table columns ───────────────────────────────────
  const columns = [
    {
      title: "Type",
      dataIndex: "ptype",
      width: "15%",
      render: (ptype: string, __: Policy, index: number) =>
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
      render: (subject: string, __: Policy, index: number) =>
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
      render: (object: string, __: Policy, index: number) =>
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
      render: (action: string, __: Policy, index: number) =>
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
              title="Are you sure you want to delete this policy?"
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
          Add Policy
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
        title="Add Policy"
        open={modalOpen}
        onOk={handleModalOk}
        onCancel={handleModalCancel}
        okText="Add"
      >
        <Form form={form} layout="vertical">
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
