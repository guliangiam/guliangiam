import { App } from "antd";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { PasswordModal } from "@/shared/ui/PasswordModal";
import { FormSection } from "@/shared/ui/FormSection";

type AccountPasswordSectionProps = {
  userId: string;
};

export function AccountPasswordSection({ userId }: AccountPasswordSectionProps) {
  const { message } = App.useApp();
  const { t } = useTranslation("resource");
  const queryClient = useQueryClient();

  return (
    <FormSection title={t("account.passwordSection")}>
      <PasswordModal
        userId={userId}
        mode="self"
        hasPassword
        onSuccess={async () => {
          message.success(t("account.passwordChanged"));
          // password:change triggers ForceLogoutUser full cascade — every device dies,
          // caller session is rotated. 详情主查询按 ["user"] 前缀取键（path 键），
          // 前缀失效才能刷新；连同会话状态一起失效。
          await queryClient.invalidateQueries({ queryKey: ["user"] });
          await queryClient.invalidateQueries({ queryKey: ["session"] });
        }}
      />
    </FormSection>
  );
}
