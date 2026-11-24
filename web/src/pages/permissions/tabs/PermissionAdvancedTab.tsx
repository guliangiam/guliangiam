/**
 * 权限详情页「高级编辑器」Tab
 *
 * 从 PermissionDetailPage.tsx 拆分而来。策略列表由页面持有，本组件只负责
 * 渲染 Casbin 策略编辑器、增删改回调与保存按钮；无 enforcer 时展示提示。
 */
import { Alert, Button, Skeleton, Space } from "antd";
import { useTranslation } from "react-i18next";
import type { Policy } from "@/shared/api/iam/enforcer";
import { CasbinPolicyEditor } from "@/shared/ui/CasbinPolicyEditor";

type PermissionAdvancedTabProps = {
  enforcerId?: string;
  loading: boolean;
  policies: Policy[];
  onPoliciesChange: (next: Policy[]) => void;
  saving: boolean;
  onSave: () => void;
};

export default function PermissionAdvancedTab({
  enforcerId,
  loading,
  policies,
  onPoliciesChange,
  saving,
  onSave,
}: PermissionAdvancedTabProps) {
  const { t: tr } = useTranslation("resource");

  if (!enforcerId) {
    return (
      <Alert
        type="info"
        title={tr("permission.noEnforcer")}
        description={tr("permission.noEnforcerDesc")}
      />
    );
  }

  return (
    <>
      {loading ? (
        <Skeleton active paragraph={{ rows: 6 }} />
      ) : (
        <CasbinPolicyEditor
          policies={policies}
          onAdd={(policy) => onPoliciesChange([...policies, policy])}
          onRemove={(index) => {
            const next = [...policies];
            next.splice(index, 1);
            onPoliciesChange(next);
          }}
          onUpdate={(index, policy) => {
            const next = [...policies];
            next[index] = policy;
            onPoliciesChange(next);
          }}
        />
      )}
      <Space style={{ marginTop: 12 }}>
        <Button type="primary" disabled={!enforcerId} loading={saving} onClick={onSave}>
          {tr("permission.savePolicies")}
        </Button>
      </Space>
    </>
  );
}
