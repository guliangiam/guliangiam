import { Button, Input, InputNumber, Space, Table } from "antd";
import { PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";
import type { ProductInfoItem } from "@/shared/api/iam/user";

type Props = {
  value: ProductInfoItem[];
  onChange: (value: ProductInfoItem[]) => void;
  readOnly?: boolean;
};

const CartTable: FC<Props> = ({ value, onChange, readOnly }) => {
  const handleAdd = useCallback(() => {
    onChange([...value, { productId: "", quantity: 1, price: 0 }]);
  }, [value, onChange]);

  const handleRemove = useCallback(
    (index: number) => {
      const next = [...value];
      next.splice(index, 1);
      onChange(next);
    },
    [value, onChange],
  );

  const handleChange = useCallback(
    (index: number, field: keyof ProductInfoItem, val: string | number) => {
      const next = [...value];
      next[index] = { ...next[index], [field]: val };
      onChange(next);
    },
    [value, onChange],
  );

  const columns = [
    {
      title: "Product ID",
      dataIndex: "productId",
      render: (_: string, __: ProductInfoItem, index: number) =>
        readOnly ? (
          value[index]?.productId
        ) : (
          <Input
            value={value[index]?.productId}
            onChange={(e) => handleChange(index, "productId", e.target.value)}
            placeholder="Product ID"
            size="small"
          />
        ),
    },
    {
      title: "Quantity",
      dataIndex: "quantity",
      render: (_: number, __: ProductInfoItem, index: number) =>
        readOnly ? (
          value[index]?.quantity
        ) : (
          <InputNumber
            value={value[index]?.quantity}
            onChange={(val) => handleChange(index, "quantity", val ?? 1)}
            min={1}
            size="small"
          />
        ),
    },
    {
      title: "Price",
      dataIndex: "price",
      render: (_: number, __: ProductInfoItem, index: number) =>
        readOnly ? (
          value[index]?.price
        ) : (
          <InputNumber
            value={value[index]?.price}
            onChange={(val) => handleChange(index, "price", val ?? 0)}
            min={0}
            step={0.01}
            size="small"
          />
        ),
    },
    ...(readOnly
      ? []
      : [
          {
            title: "",
            width: 48,
            render: (_: unknown, __: ProductInfoItem, index: number) => (
              <Button
                type="text"
                danger
                size="small"
                icon={<DeleteOutlined />}
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
      />
      {!readOnly && (
        <Button type="dashed" onClick={handleAdd} icon={<PlusOutlined />} size="small">
          Add Item
        </Button>
      )}
    </Space>
  );
};

export default CartTable;
