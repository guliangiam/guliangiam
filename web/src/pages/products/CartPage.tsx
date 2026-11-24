import { useState, useCallback } from "react";
import { Card, Button, Table, InputNumber, Typography, Empty, App } from "antd";
import { DeleteOutlined, ShoppingCartOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { placeOrder, type PlaceOrderResult } from "@/shared/api/iam/order";
import { useSessionStore } from "@/features/session/session-store";
import { localizedError } from "@/shared/lib/server-error";

type CartItem = {
  productId: string;
  name: string;
  displayName: string;
  price: number;
  currency: string;
  quantity: number;
};

const { Title, Text } = Typography;

export default function CartPage() {
  const { t } = useTranslation("resource");
  const navigate = useNavigate();
  const { message } = App.useApp();
  const organizationId = useSessionStore((state) => state.organization?.id);
  const [checkingOut, setCheckingOut] = useState(false);
  const [cartItems, setCartItems] = useState<CartItem[]>(() => {
    try {
      const stored = localStorage.getItem("guliangiam-cart");
      return stored ? JSON.parse(stored) : [];
    } catch {
      return [];
    }
  });

  const updateQuantity = useCallback((productId: string, quantity: number) => {
    if (quantity < 1) return;
    setCartItems((prev) => {
      const updated = prev.map((item) =>
        item.productId === productId ? { ...item, quantity } : item,
      );
      localStorage.setItem("guliangiam-cart", JSON.stringify(updated));
      return updated;
    });
  }, []);

  const removeItem = useCallback((productId: string) => {
    setCartItems((prev) => {
      const updated = prev.filter((item) => item.productId !== productId);
      localStorage.setItem("guliangiam-cart", JSON.stringify(updated));
      return updated;
    });
  }, []);

  // 后端按 products 数组逐项计价（每个 ID 计 1 件），且单订单强制单一币种。
  // 小计必须按币种分组，不能把不同币种的金额加成一个无单位的数。
  const totalsByCurrency = cartItems.reduce<Record<string, number>>((acc, item) => {
    acc[item.currency] = (acc[item.currency] ?? 0) + item.price * item.quantity;
    return acc;
  }, {});
  const currencyEntries = Object.entries(totalsByCurrency);

  const handleCheckout = useCallback(async () => {
    if (cartItems.length === 0) return;

    setCheckingOut(true);
    try {
      // Place orders for all cart items sequentially (API supports single-item only)
      const results: PlaceOrderResult[] = [];
      const errors: { item: CartItem; error: unknown }[] = [];

      for (const item of cartItems) {
        try {
          // 数量通过重复 productId 表达：后端对数组逐项计价（每项 Quantity=1）。
          const result = await placeOrder({
            organizationId: organizationId ?? undefined,
            products: Array.from({ length: item.quantity }, () => item.productId),
          });
          results.push(result);
        } catch (err) {
          errors.push({ item, error: err });
        }
      }

      if (errors.length > 0) {
        const failedNames = errors.map((e) => e.item.displayName).join(", ");
        message.error(t("cart.partialCheckoutFailed", { failedItems: failedNames }));
        // If some orders succeeded, still navigate to the last successful one
        if (results.length > 0) {
          const lastResult = results[results.length - 1];
          if (lastResult.payUrl) {
            window.location.href = lastResult.payUrl;
          } else {
            navigate(`/orders/${lastResult.id}`, { replace: true });
          }
        }
        return;
      }

      // All orders succeeded — clear the cart
      localStorage.removeItem("guliangiam-cart");

      // Redirect to the payment URL of the last order (or order detail)
      const lastResult = results[results.length - 1];
      if (lastResult.payUrl) {
        window.location.href = lastResult.payUrl;
      } else {
        navigate(`/orders/${lastResult.id}`, { replace: true });
      }
    } catch (err) {
      message.error(localizedError(err, t, "cart.checkoutFailed"));
    } finally {
      setCheckingOut(false);
    }
  }, [cartItems, navigate, message, t, organizationId]);

  const columns = [
    {
      title: t("cart.product"),
      dataIndex: "displayName",
      key: "displayName",
      render: (text: string) => <Text strong>{text}</Text>,
    },
    {
      title: t("cart.price"),
      dataIndex: "price",
      key: "price",
      render: (price: number, record: CartItem) => (
        <Text>
          {record.currency} {price.toFixed(2)}
        </Text>
      ),
    },
    {
      title: t("cart.quantity"),
      dataIndex: "quantity",
      key: "quantity",
      render: (quantity: number, record: CartItem) => (
        <InputNumber
          min={1}
          max={99}
          value={quantity}
          onChange={(value) => updateQuantity(record.productId, value ?? 1)}
        />
      ),
    },
    {
      title: t("cart.subtotal"),
      key: "lineTotal",
      render: (_: unknown, record: CartItem) => (
        <Text strong>
          {record.currency} {(record.price * record.quantity).toFixed(2)}
        </Text>
      ),
    },
    {
      title: "",
      key: "action",
      render: (_: unknown, record: CartItem) => (
        <Button
          type="text"
          danger
          icon={<DeleteOutlined />}
          onClick={() => removeItem(record.productId)}
        />
      ),
    },
  ];

  return (
    <Card>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          marginBottom: 24,
        }}
      >
        <Title level={4} style={{ margin: 0 }}>
          <ShoppingCartOutlined style={{ marginRight: 8 }} />
          {t("cart.title")}
        </Title>
        <Button type="link" onClick={() => navigate("/store")}>
          {t("cart.continueShopping")}
        </Button>
      </div>

      {cartItems.length === 0 ? (
        <Empty description={t("cart.empty")}>
          <Button type="primary" onClick={() => navigate("/store")}>
            {t("cart.browseProducts")}
          </Button>
        </Empty>
      ) : (
        <>
          <Table dataSource={cartItems} columns={columns} rowKey="productId" pagination={false} />
          <div
            style={{
              display: "flex",
              justifyContent: "flex-end",
              alignItems: "center",
              gap: 24,
              marginTop: 24,
              padding: 16,
              background: "rgba(0, 0, 0, 0.02)",
              borderRadius: 8,
            }}
          >
            <div>
              <Text type="secondary">{t("cart.total")}:</Text>
              {currencyEntries.map(([cur, amount]) => (
                <Text key={cur} strong style={{ fontSize: 20, marginLeft: 8 }}>
                  {cur} {amount.toFixed(2)}
                </Text>
              ))}
            </div>
            <Button
              type="primary"
              size="large"
              loading={checkingOut}
              onClick={handleCheckout}
              style={{ minWidth: 160 }}
            >
              {t("cart.checkout")}
            </Button>
          </div>
        </>
      )}
    </Card>
  );
}
