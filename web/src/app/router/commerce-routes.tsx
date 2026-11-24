import { lazy } from "react";
import type { RouteObject } from "react-router-dom";
import { lazyRouteLoader, withSuspense } from "./shared";
const plansListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/commerce-loaders"),
  "plansListLoader",
);
const planDetailLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/commerce-loaders"),
  "planDetailLoader",
);
const pricingsListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/commerce-loaders"),
  "pricingsListLoader",
);
const pricingDetailLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/commerce-loaders"),
  "pricingDetailLoader",
);
const productsListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/commerce-loaders"),
  "productsListLoader",
);
const productDetailLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/commerce-loaders"),
  "productDetailLoader",
);
const keysListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/commerce-loaders"),
  "keysListLoader",
);
const keyDetailLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/commerce-loaders"),
  "keyDetailLoader",
);

const ProductsPage = lazy(() => import("@/pages/products/ProductsPage"));
const ProductCreatePage = lazy(() => import("@/pages/products/ProductCreatePage"));
const ProductStorePage = lazy(() => import("@/pages/products/ProductStorePage"));
const ProductDetailPage = lazy(() => import("@/pages/products/ProductDetailPage"));
const ProductBuyPage = lazy(() => import("@/pages/products/ProductBuyPage"));
const CartPage = lazy(() => import("@/pages/products/CartPage"));
const OrdersPage = lazy(() => import("@/pages/orders/OrdersPage"));
const OrderCreatePage = lazy(() => import("@/pages/orders/OrderCreatePage"));
const OrderDetailPage = lazy(() => import("@/pages/orders/OrderDetailPage"));
const OrderPayPage = lazy(() => import("@/pages/orders/OrderPayPage"));
const PaymentsPage = lazy(() => import("@/pages/payments/PaymentsPage"));
const PaymentCreatePage = lazy(() => import("@/pages/payments/PaymentCreatePage"));
const PaymentDetailPage = lazy(() => import("@/pages/payments/PaymentDetailPage"));
const PaymentResultPage = lazy(() => import("@/pages/payments/PaymentResultPage"));
const PaymentsChannelConfigsPage = lazy(
  () => import("@/pages/payments/PaymentsChannelConfigsPage"),
);
const PaymentsReconPage = lazy(() => import("@/pages/payments/PaymentsReconPage"));
const PaymentsCallbackReceiptsPage = lazy(
  () => import("@/pages/payments/PaymentsCallbackReceiptsPage"),
);
const TransactionsPage = lazy(() => import("@/pages/transactions/TransactionsPage"));
const TransactionCreatePage = lazy(() => import("@/pages/transactions/TransactionCreatePage"));
const TransactionDetailPage = lazy(() => import("@/pages/transactions/TransactionDetailPage"));
const PlansPage = lazy(() => import("@/pages/plans/PlansPage"));
const PlanCreatePage = lazy(() => import("@/pages/plans/PlanCreatePage"));
const PlanDetailPage = lazy(() => import("@/pages/plans/PlanDetailPage"));
const PricingsPage = lazy(() => import("@/pages/pricings/PricingsPage"));
const PricingCreatePage = lazy(() => import("@/pages/pricings/PricingCreatePage"));
const PricingDetailPage = lazy(() => import("@/pages/pricings/PricingDetailPage"));
const SubscriptionsPage = lazy(() => import("@/pages/subscriptions/SubscriptionsPage"));
const SubscriptionCreatePage = lazy(() => import("@/pages/subscriptions/SubscriptionCreatePage"));
const SubscriptionDetailPage = lazy(() => import("@/pages/subscriptions/SubscriptionDetailPage"));
const CouponsPage = lazy(() => import("@/pages/coupons/CouponsPage"));
const CouponCreatePage = lazy(() => import("@/pages/coupons/CouponCreatePage"));
const CouponDetailPage = lazy(() => import("@/pages/coupons/CouponDetailPage"));
const KeysPage = lazy(() => import("@/pages/keys/KeysPage"));
const KeyCreatePage = lazy(() => import("@/pages/keys/KeyCreatePage"));
const KeyDetailPage = lazy(() => import("@/pages/keys/KeyDetailPage"));
const WebhookEventsPage = lazy(() => import("@/pages/webhook-events/WebhookEventsPage"));
const MeteringPage = lazy(() => import("@/pages/metering/MeteringPage"));

/** Commerce routes — products, orders, payments, plans, pricings, subscriptions, coupons, transactions, keys */
export const commerceRoutes: RouteObject[] = [
  // ── Store & Cart ──
  { path: "store", element: withSuspense(<ProductStorePage />) },
  { path: "cart", element: withSuspense(<CartPage />) },

  // ── Products ──
  { path: "products", element: withSuspense(<ProductsPage />), loader: productsListLoader },
  { path: "products/new", element: withSuspense(<ProductCreatePage />) },
  {
    path: "products/:id",
    element: withSuspense(<ProductDetailPage />),
    loader: productDetailLoader,
  },
  { path: "products/:productId/buy", element: withSuspense(<ProductBuyPage />) },

  // ── Orders ──
  { path: "orders", element: withSuspense(<OrdersPage />) },
  { path: "orders/new", element: withSuspense(<OrderCreatePage />) },
  { path: "orders/:id", element: withSuspense(<OrderDetailPage />) },
  { path: "orders/:id/pay", element: withSuspense(<OrderPayPage />) },

  // ── Payments (static before param routes — React Router matching order) ──
  { path: "payments", element: withSuspense(<PaymentsPage />) },
  { path: "payments/new", element: withSuspense(<PaymentCreatePage />) },
  { path: "payments/channel-configs", element: withSuspense(<PaymentsChannelConfigsPage />) },
  { path: "payments/reconciliation", element: withSuspense(<PaymentsReconPage />) },
  { path: "payments/callback-receipts", element: withSuspense(<PaymentsCallbackReceiptsPage />) },
  { path: "payments/:id", element: withSuspense(<PaymentDetailPage />) },
  { path: "payments/:id/result", element: withSuspense(<PaymentResultPage />) },

  // ── Plans & Pricings ──
  { path: "plans", element: withSuspense(<PlansPage />), loader: plansListLoader },
  { path: "plans/new", element: withSuspense(<PlanCreatePage />) },
  { path: "plans/:id", element: withSuspense(<PlanDetailPage />), loader: planDetailLoader },
  { path: "pricings", element: withSuspense(<PricingsPage />), loader: pricingsListLoader },
  { path: "pricings/new", element: withSuspense(<PricingCreatePage />) },
  {
    path: "pricings/:id",
    element: withSuspense(<PricingDetailPage />),
    loader: pricingDetailLoader,
  },

  // ── Subscriptions ──
  { path: "subscriptions", element: withSuspense(<SubscriptionsPage />) },
  { path: "subscriptions/new", element: withSuspense(<SubscriptionCreatePage />) },
  { path: "subscriptions/:id", element: withSuspense(<SubscriptionDetailPage />) },

  // ── Coupons ──
  { path: "coupons", element: withSuspense(<CouponsPage />) },
  { path: "coupons/new", element: withSuspense(<CouponCreatePage />) },
  { path: "coupons/:id", element: withSuspense(<CouponDetailPage />) },

  // ── Transactions ──
  { path: "transactions", element: withSuspense(<TransactionsPage />) },
  { path: "transactions/new", element: withSuspense(<TransactionCreatePage />) },
  { path: "transactions/:id", element: withSuspense(<TransactionDetailPage />) },

  // ── Keys ──
  { path: "keys", element: withSuspense(<KeysPage />), loader: keysListLoader },
  { path: "keys/new", element: withSuspense(<KeyCreatePage />) },
  { path: "keys/:id", element: withSuspense(<KeyDetailPage />), loader: keyDetailLoader },

  // ── Webhook Events ──
  { path: "webhooks/:webhookId/events", element: withSuspense(<WebhookEventsPage />) },

  // ── Metering ──
  { path: "metering", element: withSuspense(<MeteringPage />) },
];
