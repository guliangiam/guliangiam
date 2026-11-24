import { useState } from "react";
import { Alert, App, Button, Modal, Select, Space, Typography, Upload } from "antd";
import { CameraOutlined, PlusOutlined } from "@ant-design/icons";
import { useQuery } from "@tanstack/react-query";
import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { FormSection } from "@/shared/ui/FormSection";
import FaceIdTable from "@/shared/ui/FaceIdTable";
import { getIamFaceIdProviders } from "@/shared/api/iam/face-id-provider";
import type { FaceIdItem, UserDetail } from "@/shared/api/iam/user-types";

type AccountFaceIdSectionProps = {
  user: UserDetail | undefined;
  faceIds: FaceIdItem[];
  registerFaceIdMutation: UseMutationResult<FaceIdItem, Error, { file: File; providerId?: string }>;
  revokeFaceIdMutation: UseMutationResult<void, Error, void>;
  /** 未实名时引导跳转到实名分区。 */
  onGoToRealName?: () => void;
};

/**
 * 自助 Face ID 底图管理（account 安全分区）。
 *
 * 与 admin UserFaceIdSection 的差异：注册/撤销走会话绑定的 /auth/faceids
 * 自助端点（无 userId/orgId 参数）；provider 列表读取失败或为空时仍允许注册
 * （providerId 留空，由后端裁决默认 provider），不因普通成员无 provider 读权限
 * 而隐藏注册入口。实名门槛与 admin 一致：未实名只展示引导。
 */
export function AccountFaceIdSection({
  user,
  faceIds,
  registerFaceIdMutation,
  revokeFaceIdMutation,
  onGoToRealName,
}: AccountFaceIdSectionProps) {
  const { t } = useTranslation("resource");
  const { message, modal } = App.useApp();
  const [faceIdProvider, setFaceIdProvider] = useState<string | undefined>(undefined);
  const [faceIdRegisterOpen, setFaceIdRegisterOpen] = useState(false);
  const [faceIdImage, setFaceIdImage] = useState<File | null>(null);

  const isVerified = !!user?.isVerified;

  // 普通成员可能无 face-id-providers 读权限：retry:false 容错，失败/为空时
  // 注册仍可用（providerId 留空，后端用默认 provider）。
  const faceIdProvidersQuery = useQuery({
    queryKey: ["face-id-providers", user?.organizationId],
    queryFn: () => getIamFaceIdProviders({ organizationId: user?.organizationId }),
    enabled: isVerified,
    retry: false,
  });

  const faceIdProviderOptions: { label: string; value: string }[] = (
    faceIdProvidersQuery.data ?? []
  )
    .filter((p) => p.isEnabled)
    .map((p) => ({ label: p.name, value: p.id }));

  const openRegister = () => setFaceIdRegisterOpen(true);

  const closeRegister = () => {
    setFaceIdRegisterOpen(false);
    setFaceIdImage(null);
  };

  const submitRegister = () => {
    if (!faceIdImage) return;
    registerFaceIdMutation.mutate(
      { file: faceIdImage, providerId: faceIdProvider },
      { onSuccess: closeRegister },
    );
  };

  const confirmRevoke = () => {
    modal.confirm({
      title: t("user.faceIdRevokeConfirmTitle"),
      content: t("user.faceIdRevokeConfirmContent"),
      okText: t("user.faceIdRevoke"),
      okButtonProps: { danger: true },
      cancelText: t("common:cancel"),
      onOk: () => revokeFaceIdMutation.mutate(),
    });
  };

  // 未实名：只展示引导，不暴露注册/撤销入口（与后端实名门槛一致）。
  if (!isVerified) {
    return (
      <FormSection title={t("user.faceIds")}>
        <Alert
          type="warning"
          showIcon
          title={t("user.faceIdRealNameRequired")}
          action={
            onGoToRealName ? (
              <Button size="small" onClick={onGoToRealName}>
                {t("user.faceIdGoToRealName")}
              </Button>
            ) : undefined
          }
        />
      </FormSection>
    );
  }

  return (
    <FormSection
      title={t("user.faceIds")}
      extra={
        <Space>
          {faceIdProviderOptions.length > 0 && (
            <>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {t("user.faceIdProviderSelect")}:
              </Typography.Text>
              <Select
                size="small"
                style={{ minWidth: 160 }}
                placeholder={t("user.faceIdProviderSelectPlaceholder")}
                options={faceIdProviderOptions}
                value={faceIdProvider}
                onChange={setFaceIdProvider}
              />
            </>
          )}
          <Button type="primary" size="small" icon={<PlusOutlined />} onClick={openRegister}>
            {t("user.faceIdRegisterNew")}
          </Button>
        </Space>
      }
    >
      <FaceIdTable
        value={faceIds}
        onRemove={confirmRevoke}
        showProviderColumn={faceIds.some((f) => f.provider)}
        onRegisterNew={openRegister}
      />

      <Modal
        title={t("user.faceIdRegisterTitle")}
        open={faceIdRegisterOpen}
        onCancel={closeRegister}
        onOk={submitRegister}
        okText={t("user.faceIdRegister")}
        confirmLoading={registerFaceIdMutation.isPending}
        okButtonProps={{ disabled: !faceIdImage }}
        width={420}
      >
        <Space orientation="vertical" style={{ width: "100%" }} size="middle">
          <Typography.Text type="secondary">
            {t("user.faceIdRegisterHint", {
              name: faceIdProviderOptions.find((o) => o.value === faceIdProvider)?.label ?? "",
            })}
          </Typography.Text>
          {faceIdProviderOptions.length > 0 && (
            <Select
              value={faceIdProvider}
              onChange={setFaceIdProvider}
              options={faceIdProviderOptions}
              placeholder={t("user.faceIdProvider")}
              loading={faceIdProvidersQuery.isLoading}
              style={{ width: "100%" }}
            />
          )}
          <Upload.Dragger
            accept="image/jpeg,image/png,image/webp,image/gif"
            maxCount={1}
            beforeUpload={(file) => {
              if (file.size > 5 * 1024 * 1024) {
                message.error(t("user.faceIdImageTooLarge"));
                return Upload.LIST_IGNORE;
              }
              setFaceIdImage(file);
              return false;
            }}
            onRemove={() => setFaceIdImage(null)}
          >
            <p className="ant-upload-drag-icon">
              <CameraOutlined />
            </p>
            <p className="ant-upload-text">{t("user.selectFaceImage")}</p>
          </Upload.Dragger>
        </Space>
      </Modal>
    </FormSection>
  );
}
