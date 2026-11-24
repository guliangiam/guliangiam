import { Alert, App, Button, Modal, Select, Space, Typography, Upload } from "antd";
import { CameraOutlined, PlusOutlined } from "@ant-design/icons";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { FormSection } from "@/shared/ui/FormSection";
import FaceIdTable from "@/shared/ui/FaceIdTable";
import { registerUserFaceId, revokeUserFaceId } from "@/shared/api/iam/user";
import { getIamFaceIdProviders } from "@/shared/api/iam/face-id-provider";
import { serverErrorMessage } from "@/shared/lib/server-error";
import type { FaceIdItem, UserDetail } from "./types";

interface UserFaceIdSectionProps {
  user: UserDetail | undefined;
  faceIds: FaceIdItem[];
  onRemoveFaceId?: (index: number) => void;
  /** 注册成功后追加到父级 collections 草稿，保持与 PATCH collections 全量替换一致。 */
  onAppendFaceId?: (faceId: FaceIdItem) => void;
  /** users:write：无写权限时所有写入口只读化，而不是等后端 403。 */
  canWrite?: boolean;
  /** 未实名时引导跳转到实名分区。 */
  onGoToRealName?: () => void;
}

/**
 * Face ID 底图管理（凭证分区）。
 *
 * Face ID 是主登录凭证（与密码 / WebAuthn 同级），不是 MFA 二次因子，因此从 MFA
 * 分区迁入凭证分区。可用性前置门槛 = 已实名：未实名时只展示引导，不暴露注册入口
 * （后端 EnrollUserFaceID / InitiateFaceID 同样以实名门槛拒绝）。
 *
 * 注册：图片由后端交给 FaceID 提供商做活体检测，通过后以加密引用存入
 * user_face_templates（绝不存明文图片），并在 face_ids JSONB 留兼容标记。
 * 撤销：DELETE /users/{id}/faceids 在同一事务撤销加密底图 + 清标记 + 审计，
 * 撤销后该用户无法再用 Face ID 登录（仅改 JSONB 草稿不会撤销底图，故走专用端点）。
 */
export default function UserFaceIdSection({
  user,
  faceIds,
  onRemoveFaceId,
  onAppendFaceId,
  canWrite = true,
  onGoToRealName,
}: UserFaceIdSectionProps) {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const queryClient = useQueryClient();
  const [faceIdProvider, setFaceIdProvider] = useState<string | undefined>(undefined);
  const [faceIdRegisterOpen, setFaceIdRegisterOpen] = useState(false);
  const [faceIdImage, setFaceIdImage] = useState<File | null>(null);

  const isVerified = !!user?.isVerified;

  const faceIdProvidersQuery = useQuery({
    queryKey: ["face-id-providers", user?.organizationId],
    queryFn: () => getIamFaceIdProviders({ organizationId: user?.organizationId }),
    enabled: !!user?.organizationId && isVerified,
  });

  const faceIdProviderOptions: { label: string; value: string }[] = (
    faceIdProvidersQuery.data ?? []
  )
    .filter((p: { isEnabled: boolean }) => p.isEnabled)
    .map((p: { id: string; name: string }) => ({ label: p.name, value: p.id }));

  const registerFaceIdMutation = useMutation({
    mutationFn: () =>
      registerUserFaceId(
        user!.id,
        {
          file: faceIdImage as File,
          providerId: faceIdProvider,
          deviceInfo: typeof navigator === "undefined" ? undefined : navigator.userAgent,
        },
        user?.organizationId,
      ),
    onSuccess: (faceId) => {
      onAppendFaceId?.(faceId);
      message.success(t("resource:user.faceIdRegistered", "Face template registered"));
      setFaceIdRegisterOpen(false);
      setFaceIdImage(null);
      queryClient.invalidateQueries({ queryKey: ["user"] });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
    onError: (err) =>
      message.error(
        serverErrorMessage(err, t, "common:loadFailed") ??
          t("resource:user.faceIdRegisterFailed", "Face registration failed"),
      ),
  });

  const revokeFaceIdMutation = useMutation({
    mutationFn: (_index: number) => revokeUserFaceId(user!.id, user?.organizationId),
    onSuccess: (_data, index) => {
      onRemoveFaceId?.(index);
      message.success(t("resource:user.faceIdRevoked", "Face template revoked"));
      queryClient.invalidateQueries({ queryKey: ["user"] });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
    onError: (err) =>
      message.error(
        serverErrorMessage(err, t, "common:loadFailed") ??
          t("resource:user.faceIdRevokeFailed", "Face revocation failed"),
      ),
  });

  const confirmRevoke = (index: number) => {
    modal.confirm({
      title: t("resource:user.faceIdRevokeConfirmTitle", "Revoke this face template?"),
      content: t(
        "resource:user.faceIdRevokeConfirmContent",
        "The user will no longer be able to sign in with Face ID. This cannot be undone.",
      ),
      okText: t("resource:user.faceIdRevoke", "Revoke"),
      okButtonProps: { danger: true },
      cancelText: t("common:cancel", { defaultValue: "Cancel" }),
      onOk: () => revokeFaceIdMutation.mutate(index),
    });
  };

  // 未实名：只展示引导，不暴露注册/撤销入口（与后端实名门槛一致）。
  if (!isVerified) {
    return (
      <FormSection title={t("resource:user.faceIds", "Face IDs")}>
        <Alert
          type="warning"
          showIcon
          title={t(
            "resource:user.faceIdRealNameRequired",
            "Face ID requires real-name verification. Complete it in the Real-name tab first.",
          )}
          action={
            onGoToRealName ? (
              <Button size="small" onClick={onGoToRealName}>
                {t("resource:user.faceIdGoToRealName", "Go to Real-name verification")}
              </Button>
            ) : undefined
          }
        />
      </FormSection>
    );
  }

  return (
    <FormSection
      title={t("resource:user.faceIds", "Face IDs")}
      extra={
        canWrite && faceIdProviderOptions.length > 0 ? (
          <Space>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {t("resource:user.faceIdProviderSelect", "Provider")}:
            </Typography.Text>
            <Select
              size="small"
              style={{ minWidth: 160 }}
              placeholder={t("resource:user.faceIdProviderSelectPlaceholder")}
              options={faceIdProviderOptions}
              value={faceIdProvider}
              onChange={setFaceIdProvider}
            />
            <Button
              type="primary"
              size="small"
              icon={<PlusOutlined />}
              disabled={!faceIdProvider}
              onClick={() => setFaceIdRegisterOpen(true)}
            >
              {t("resource:user.faceIdRegisterNew", "Register New Face ID")}
            </Button>
          </Space>
        ) : null
      }
    >
      <FaceIdTable
        value={faceIds}
        onRemove={confirmRevoke}
        readOnly={!canWrite}
        showProviderColumn={faceIds.some((f) => f.provider)}
        onRegisterNew={() => setFaceIdRegisterOpen(true)}
      />

      <Modal
        title={t("resource:user.faceIdRegisterTitle", "Register New Face ID")}
        open={faceIdRegisterOpen}
        onCancel={() => {
          setFaceIdRegisterOpen(false);
          setFaceIdImage(null);
        }}
        onOk={() => registerFaceIdMutation.mutate()}
        okText={t("resource:user.faceIdRegister", "Register")}
        confirmLoading={registerFaceIdMutation.isPending}
        okButtonProps={{ disabled: !faceIdImage }}
        width={420}
      >
        <Space orientation="vertical" style={{ width: "100%" }} size="middle">
          <Typography.Text type="secondary">
            {t(
              "resource:user.faceIdRegisterHint",
              "The photo is liveness-checked by provider {{name}} before it is stored.",
              { name: faceIdProviderOptions.find((o) => o.value === faceIdProvider)?.label ?? "" },
            )}
          </Typography.Text>
          <Select
            value={faceIdProvider}
            onChange={setFaceIdProvider}
            options={faceIdProviderOptions}
            placeholder={t("resource:user.faceIdProvider", "Face ID provider")}
            loading={faceIdProvidersQuery.isLoading}
            style={{ width: "100%" }}
          />
          <Upload.Dragger
            accept="image/jpeg,image/png,image/webp,image/gif"
            maxCount={1}
            beforeUpload={(file) => {
              if (file.size > 5 * 1024 * 1024) {
                message.error(
                  t("resource:user.faceIdImageTooLarge", "Image exceeds the 5MiB limit"),
                );
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
            <p className="ant-upload-text">
              {t("resource:user.selectFaceImage", "Click or drag a face photo to upload")}
            </p>
          </Upload.Dragger>
        </Space>
      </Modal>
    </FormSection>
  );
}
