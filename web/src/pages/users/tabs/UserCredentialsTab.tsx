import { useState } from "react";
import {
  Alert,
  Button,
  Col,
  Descriptions,
  Form,
  Modal,
  Row,
  Space,
  Switch,
  Tag,
  Typography,
} from "antd";
import { useQueryClient } from "@tanstack/react-query";
import { CopyOutlined, ExclamationCircleOutlined, WarningOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import { usePermissions } from "@/shared/hooks/usePermissions";
import { FormSection } from "@/shared/ui/FormSection";
import { PasswordModal } from "@/shared/ui/PasswordModal";
import { formatDateTime, getBool, getStr } from "./types";
import type { FaceIdItem, UserDetail } from "./types";
import UserFaceIdSection from "./UserFaceIdSection";

/** 与后端 userdomain.AccessSecretGracePeriod 保持一致（30 分钟）。 */
const ACCESS_SECRET_GRACE_MINUTES = 30;

interface RotatedSecret {
  accessKey: string;
  accessSecret: string;
}

interface UserCredentialsTabProps {
  user: UserDetail | undefined;
  canWrite?: boolean;
  /** 轮换 Access Secret 的回调（独立按钮触发，不走表单保存）。 */
  onRotateSecret?: () => void;
  rotatePending?: boolean;
  /** 轮换成功后一次性返回的新密钥（仅本次会话可见）。 */
  rotatedSecret?: RotatedSecret | null;
  onDismissRotatedSecret?: () => void;
  /** Face ID 底图草稿（collections），与 PATCH collections 全量替换共享。 */
  faceIds?: FaceIdItem[];
  onRemoveFaceId?: (index: number) => void;
  onAppendFaceId?: (faceId: FaceIdItem) => void;
  /** 未实名时引导跳转到实名分区。 */
  onGoToRealName?: () => void;
}

export default function UserCredentialsTab({
  user,
  canWrite = false,
  onRotateSecret,
  rotatePending = false,
  rotatedSecret = null,
  onDismissRotatedSecret,
  faceIds = [],
  onRemoveFaceId,
  onAppendFaceId,
  onGoToRealName,
}: UserCredentialsTabProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  // 口令重置使用独立权限点：无 users:password:reset 时按钮不渲染，而不是置灰。
  const { hasAction } = usePermissions();
  const canResetPassword = hasAction("users:password:reset");

  const [confirmOpen, setConfirmOpen] = useState(false);

  const copySecret = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      /* 静默：用户可能禁用了剪贴板权限，不阻塞主流程。 */
    }
  };

  const openConfirm = () => setConfirmOpen(true);
  const closeConfirm = () => setConfirmOpen(false);

  const handleConfirmRotate = () => {
    closeConfirm();
    onRotateSecret?.();
  };

  return (
    <>
      <FormSection
        title={t("resource:user.credentialsSection")}
        description={t("resource:user.credentialsSectionDesc")}
      >
        {canResetPassword && (
          <Row gutter={16}>
            <Col span={12}>
              <PasswordModal
                userId={user?.id ?? ""}
                organizationId={user?.organizationId}
                mode="admin"
                hasPassword={getBool(user?.credentialSummary, "hasPassword")}
                disabled={!user}
                onSuccess={() => {
                  // 重置后 credentialSummary（hasPassword/lastChangePasswordTime）
                  // 已过期；详情页主查询键为 ["user","path",…]，按前缀失效。
                  void queryClient.invalidateQueries({ queryKey: ["user"] });
                }}
              />
            </Col>
          </Row>
        )}
        <Row gutter={16} style={{ marginTop: 16 }}>
          <Col span={12}>
            <Form.Item label={t("resource:user.rotateAccessSecret")}>
              <Button
                danger
                onClick={openConfirm}
                loading={rotatePending}
                disabled={!canWrite || !user}
              >
                {t("resource:user.rotateAccessSecret")}
              </Button>
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              label={t("resource:user.needUpdatePassword")}
              name="needUpdatePassword"
              valuePropName="checked"
            >
              <Switch disabled={!canWrite} />
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      {/* ── Face ID 底图：主登录凭证，已实名门槛；未实名引导实名分区 ── */}
      <div style={{ marginTop: 16 }}>
        <UserFaceIdSection
          user={user}
          faceIds={faceIds}
          onRemoveFaceId={onRemoveFaceId}
          onAppendFaceId={onAppendFaceId}
          canWrite={canWrite}
          onGoToRealName={onGoToRealName}
        />
      </div>

      <FormSection
        title={t("resource:user.overviewSection")}
        description={t("resource:user.overviewSectionDesc")}
        style={{ marginTop: 16 }}
      >
        <Descriptions column={{ xs: 1, sm: 2 }} size="small" bordered>
          <Descriptions.Item label={t("resource:user.hasPassword")}>
            {getBool(user?.credentialSummary, "hasPassword") ? (
              <Tag color="green">{t("yes")}</Tag>
            ) : (
              <Tag>{t("no")}</Tag>
            )}
          </Descriptions.Item>
          <Descriptions.Item label={t("resource:user.passwordAlgorithm")}>
            {getStr(user?.credentialSummary, "algorithm") || "-"}
          </Descriptions.Item>
          <Descriptions.Item label={t("resource:user.lastPasswordChange")}>
            {formatDateTime(getStr(user?.credentialSummary, "lastChangePasswordTime"))}
          </Descriptions.Item>
          <Descriptions.Item label={t("resource:user.hasAccessKey")}>
            {getBool(user?.credentialSummary, "hasAccessKey") ? (
              <Tag color="green">{t("yes")}</Tag>
            ) : (
              <Tag>{t("no")}</Tag>
            )}
          </Descriptions.Item>
          <Descriptions.Item label={t("resource:user.accessKeyPreview")}>
            <Typography.Text code>
              {getStr(user?.credentialSummary, "accessKeyMasked") || "-"}
            </Typography.Text>
          </Descriptions.Item>
          <Descriptions.Item label={t("resource:user.secretLastRotated")}>
            {formatDateTime(getStr(user?.credentialSummary, "secretLastRotated"))}
          </Descriptions.Item>
          <Descriptions.Item label={t("resource:user.lastLogin")}>
            {formatDateTime(user?.lastSigninAt)}
          </Descriptions.Item>
          <Descriptions.Item label={t("resource:user.recoveryCodeCount")}>
            {user?.recoveryCodes?.length ?? 0}
          </Descriptions.Item>
        </Descriptions>
      </FormSection>

      {/* ── 轮换确认 Modal：明确宽限期语义 + 一次性警告 ── */}
      <Modal
        open={confirmOpen}
        onCancel={closeConfirm}
        title={
          <Space>
            <ExclamationCircleOutlined style={{ color: "var(--gl-color-warning)" }} />
            {t("resource:user.rotateAccessSecretConfirmTitle")}
          </Space>
        }
        okText={t("resource:user.rotateAccessSecretConfirmOk")}
        cancelText={t("common:cancel")}
        okButtonProps={{ danger: true }}
        okType="danger"
        onOk={handleConfirmRotate}
        confirmLoading={rotatePending}
        mask={{ closable: !rotatePending }}
        closable={!rotatePending}
      >
        <Space orientation="vertical" size="large" style={{ display: "flex" }}>
          <Alert
            type="warning"
            showIcon
            icon={<WarningOutlined />}
            title={t("resource:user.rotateAccessSecretConfirmRevokeHint")}
          />
          <Typography.Paragraph style={{ marginBottom: 0 }}>
            {t("resource:user.rotateAccessSecretConfirmDesc", {
              minutes: ACCESS_SECRET_GRACE_MINUTES,
            })}
          </Typography.Paragraph>
        </Space>
      </Modal>

      {/* ── 新凭据 Modal：后端成功后一次性展示 ── */}
      <Modal
        open={!!rotatedSecret}
        onCancel={onDismissRotatedSecret}
        title={t("resource:user.newAccessSecretTitle")}
        mask={{ closable: false }}
        destroyOnHidden
        footer={
          <Button type="primary" onClick={onDismissRotatedSecret}>
            {t("resource:user.newAccessSecretSaved")}
          </Button>
        }
      >
        {rotatedSecret && (
          <Space orientation="vertical" size="large" style={{ display: "flex" }}>
            <Alert type="warning" showIcon title={t("resource:user.newAccessSecretDesc")} />
            <Descriptions bordered size="small" column={1}>
              <Descriptions.Item label={t("resource:user.accessKeyLabel")}>
                <Space>
                  <Typography.Text code>{rotatedSecret.accessKey}</Typography.Text>
                  <Button
                    size="small"
                    icon={<CopyOutlined />}
                    aria-label={t("resource:user.copyAccessKey")}
                    onClick={() => void copySecret(rotatedSecret.accessKey)}
                  />
                </Space>
              </Descriptions.Item>
              <Descriptions.Item label={t("resource:user.accessSecretLabel")}>
                <Space>
                  <Typography.Text code>{rotatedSecret.accessSecret}</Typography.Text>
                  <Button
                    size="small"
                    icon={<CopyOutlined />}
                    aria-label={t("resource:user.copyAccessSecret")}
                    onClick={() => void copySecret(rotatedSecret.accessSecret)}
                  />
                </Space>
              </Descriptions.Item>
            </Descriptions>
            <Alert
              type="info"
              showIcon
              title={t("resource:user.rotatedSecretGraceHint", {
                minutes: ACCESS_SECRET_GRACE_MINUTES,
              })}
            />
          </Space>
        )}
      </Modal>
    </>
  );
}
