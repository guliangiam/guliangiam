import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  getIamOrders,
  getIamOrdersPaginated,
  getIamOrder,
  createIamOrder,
  patchIamOrder,
  deleteIamOrder,
  getIamOrderPayUrl,
  notifyIamOrderPaid,
  placeOrder,
  getUserOrders,
} from "./order";

// Mock the IAM client module
const mockIamList = vi.fn<(...args: unknown[]) => unknown>();
const mockIamListWithPagination = vi.fn<(...args: unknown[]) => unknown>();
const mockIamGet = vi.fn<(...args: unknown[]) => unknown>();
const mockIamCreate = vi.fn<(...args: unknown[]) => unknown>();
const mockIamPatch = vi.fn<(...args: unknown[]) => unknown>();
const mockIamDelete = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("./client", () => ({
  iamList: (...args: unknown[]) => mockIamList(...args),
  iamListWithPagination: (...args: unknown[]) => mockIamListWithPagination(...args),
  iamGet: (...args: unknown[]) => mockIamGet(...args),
  iamCreate: (...args: unknown[]) => mockIamCreate(...args),
  iamPatch: (...args: unknown[]) => mockIamPatch(...args),
  iamDelete: (...args: unknown[]) => mockIamDelete(...args),
}));

describe("Order API", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("getIamOrders", () => {
    it("calls iamList with correct path", async () => {
      mockIamList.mockResolvedValue([]);
      await getIamOrders();
      expect(mockIamList).toHaveBeenCalledWith("/api/v1/orders", undefined);
    });

    it("passes params to iamList", async () => {
      mockIamList.mockResolvedValue([]);
      await getIamOrders({ page: 1, pageSize: 20 });
      expect(mockIamList).toHaveBeenCalledWith("/api/v1/orders", { page: 1, pageSize: 20 });
    });
  });

  describe("getIamOrdersPaginated", () => {
    it("calls iamListWithPagination with correct path", async () => {
      mockIamListWithPagination.mockResolvedValue({ items: [], page: 1, pageSize: 50, total: 0 });
      await getIamOrdersPaginated();
      expect(mockIamListWithPagination).toHaveBeenCalledWith("/api/v1/orders", undefined);
    });
  });

  describe("getIamOrder", () => {
    it("calls iamGet with correct path", async () => {
      mockIamGet.mockResolvedValue({ id: "order-1", name: "test" });
      await getIamOrder("order-1");
      expect(mockIamGet).toHaveBeenCalledWith("/api/v1/orders/order-1");
    });
  });

  describe("createIamOrder", () => {
    it("calls iamCreate with correct path and payload", async () => {
      mockIamCreate.mockResolvedValue({ id: "order-new" });
      const payload = {
        organizationId: "org-1",
        name: "order-1",
        displayName: "Order 1",
        type: "subscription",
      };
      await createIamOrder(payload);
      expect(mockIamCreate).toHaveBeenCalledWith("/api/v1/orders", payload);
    });
  });

  describe("patchIamOrder", () => {
    it("calls iamPatch with correct path and payload", async () => {
      mockIamPatch.mockResolvedValue(undefined);
      const payload = { displayName: "Updated Order", revision: 1 };
      await patchIamOrder("order-1", payload);
      expect(mockIamPatch).toHaveBeenCalledWith("/api/v1/orders/order-1", payload);
    });
  });

  describe("deleteIamOrder", () => {
    it("calls iamDelete with correct path", async () => {
      mockIamDelete.mockResolvedValue(undefined);
      await deleteIamOrder("order-1");
      expect(mockIamDelete).toHaveBeenCalledWith("/api/v1/orders/order-1");
    });
  });

  describe("getIamOrderPayUrl", () => {
    it("calls iamGet with correct path", async () => {
      mockIamGet.mockResolvedValue({
        payUrl: "https://pay.example.com/checkout",
        state: "pending",
      });
      await getIamOrderPayUrl("order-1");
      expect(mockIamGet).toHaveBeenCalledWith("/api/v1/orders/order-1/pay-url");
    });
  });

  describe("notifyIamOrderPaid", () => {
    it("calls iamCreate with correct path and payload", async () => {
      mockIamCreate.mockResolvedValue(undefined);
      await notifyIamOrderPaid("order-1", { paymentId: "pay-1" });
      expect(mockIamCreate).toHaveBeenCalledWith("/api/v1/orders/order-1/notify-paid", {
        paymentId: "pay-1",
      });
    });
  });

  describe("placeOrder", () => {
    it("calls iamCreate with correct path and params", async () => {
      mockIamCreate.mockResolvedValue({
        id: "order-new",
        payUrl: "https://pay.example.com",
        state: "pending",
      });
      await placeOrder({ organizationId: "org-1", productId: "prod-1", pricingId: "pricing-1" });
      expect(mockIamCreate).toHaveBeenCalledWith("/api/v1/orders/place", {
        organizationId: "org-1",
        products: ["prod-1", "pricing-1"],
      });
    });
  });

  describe("getUserOrders", () => {
    it("calls iamGet with correct path and query params", async () => {
      mockIamGet.mockResolvedValue({ items: [], page: { page: 1, pageSize: 50, total: 0 } });
      await getUserOrders({ organizationId: "org-1", user: "user-1" });
      expect(mockIamGet).toHaveBeenCalled();
      const calledUrl = mockIamGet.mock.calls[0][0] as string;
      expect(calledUrl).toContain("/api/v1/orders/user-orders");
      expect(calledUrl).toContain("organizationId=org-1");
      expect(calledUrl).toContain("user=user-1");
    });
  });
});
