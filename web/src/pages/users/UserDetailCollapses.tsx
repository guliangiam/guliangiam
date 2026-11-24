/**
 * UserDetailPage 的 MFA & Finance Collapse 段
 *
 * 从 UserDetailPage.tsx 拆分而来，封装两个嵌套的 ant Collapse：
 *   - UserMfaCollapse   MFA Methods / WebAuthn / Face ID 三段折叠
 *   - UserFinanceCollapse  Finance 总览 / Orders / Payments 三段折叠
 *
 * 所有状态通过 props 传递，避免父组件里内联 JSX 导致的重渲染闭包开销
 * （Vercel `rerender-no-inline-components` 规则）。
 */
import { Collapse } from "antd";
import { useTranslation } from "react-i18next";
import { type ProductInfoItem, type UserDetail } from "@/shared/api/iam/user";
import UserMfaTab from "./tabs/UserMfaTab";
import UserFinanceTab from "./tabs/UserFinanceTab";
import UserOrdersTab from "./tabs/UserOrdersTab";
import UserPaymentsTab from "./tabs/UserPaymentsTab";

type MfaMethodRow = {
  methodType: string;
  enabled: boolean;
  displayName?: string;
};

type UserMfaCollapseProps = {
  user?: UserDetail;
  onUpdateMfaMethods: (methods: MfaMethodRow[]) => void;
  /** Promote one MFA method to the user's preferred factor. */
  onSetPreferredMfa: (mfaType: string) => void;
  /** users:write 写权限；无权限时 MFA 写入口只读。 */
  canWrite?: boolean;
  /** MFA 方法替换进行中，禁用开关避免基于旧快照的连点丢更新。 */
  mfaPending?: boolean;
};

/**
 * MFA 分区直出：UserMfaTab 内部已用 FormSection 卡片，
 * 不再需要外层 Collapse 包装。组件保留原名以便调用端不做改动。
 * Face ID 是主登录凭证，已迁入凭证分区（UserCredentialsTab），MFA 分区只留二次因子。
 */
export function UserMfaCollapse({
  user,
  onUpdateMfaMethods,
  onSetPreferredMfa,
  canWrite = true,
  mfaPending = false,
}: UserMfaCollapseProps) {
  return (
    <UserMfaTab
      user={user}
      onUpdateMfaMethods={onUpdateMfaMethods}
      onSetPreferredMfa={onSetPreferredMfa}
      canWrite={canWrite}
      mfaPending={mfaPending}
    />
  );
}

type UserFinanceCollapseProps = {
  user?: UserDetail;
  cart: ProductInfoItem[];
  onCartChange: (next: ProductInfoItem[]) => void;
  canWrite?: boolean;
};

/** Collapse grouping the finance overview with orders and payments sub-sections. */
export function UserFinanceCollapse({
  user,
  cart,
  onCartChange,
  canWrite = true,
}: UserFinanceCollapseProps) {
  const { t } = useTranslation();
  return (
    <Collapse
      defaultActiveKey={["overview", "orders", "payments"]}
      ghost
      items={[
        {
          key: "overview",
          label: t("resource:user.tabs.finance", "Finance"),
          children: (
            <UserFinanceTab
              user={user}
              cart={cart}
              onChangeCart={onCartChange}
              canWrite={canWrite}
            />
          ),
        },
        {
          key: "orders",
          label: t("resource:user.tabs.orders"),
          children: user ? (
            <UserOrdersTab userId={user.id} organizationId={user.organizationId} />
          ) : null,
        },
        {
          key: "payments",
          label: t("resource:user.tabs.payments"),
          children: user ? (
            <UserPaymentsTab userId={user.id} organizationId={user.organizationId} />
          ) : null,
        },
      ]}
    />
  );
}
