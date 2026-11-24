import { useEffect } from "react";
import { Alert, Button, Form, Input, Select, Space } from "antd";
import { CheckCircleOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import type {
  UserDetail,
  UpdateUserProfilePayload,
  IdentityVerificationTicket,
} from "@/shared/api/iam/user-types";
import type { UseMutationResult } from "@tanstack/react-query";
import { isHttpError } from "@/shared/api/http";
import { applyServerFieldErrors, buildServerFieldResolver } from "@/shared/lib/server-error";

type AccountRealNameSectionProps = {
  user: UserDetail | undefined;
  realNameForm: ReturnType<typeof Form.useForm>[0];
  updateProfileMutation: UseMutationResult<void, Error, UpdateUserProfilePayload>;
  verifyIdMutation: UseMutationResult<IdentityVerificationTicket | undefined, Error, void>;
  /** CloudAuth SDK 注入与 metaInfo 采集阶段：按钮改文案，避免点了像没反应。 */
  idvCollecting?: boolean;
};

export function AccountRealNameSection({
  user,
  realNameForm,
  updateProfileMutation,
  verifyIdMutation,
  idvCollecting = false,
}: AccountRealNameSectionProps) {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");

  // 规范 9.5：证件号永不回传原文，响应只有 Set/Masked 对。
  const idCardSet = user?.profile?.idCardSet === true;
  const idCardMasked =
    typeof user?.profile?.idCardMasked === "string" ? user.profile.idCardMasked : "";

  useEffect(() => {
    if (!user) return;
    const profile = user.profile;
    realNameForm.setFieldsValue({
      realName: (profile.realName as string | undefined) ?? "",
      idCardType: (profile.idCardType as string | undefined) ?? "",
      // 证件号不预填：留空 = 保持存储值（提交时空值会被剔除，见 onFinish）。
      idCard: "",
    });
  }, [user, realNameForm]);

  // useWatch 订阅字段变化驱动重渲染；getFieldsValue 在渲染期取值不会随输入更新，
  // 会让「验证身份」按钮在三字段齐备后仍持续 disabled。
  const realName = Form.useWatch("realName", realNameForm);
  const idCardType = Form.useWatch("idCardType", realNameForm);
  const idCard = Form.useWatch("idCard", realNameForm);
  // 已存储证件号时表单留空不代表缺失：验证按钮仍可用。
  const canVerify = !!realName && !!idCardType && (!!idCard || idCardSet);

  return (
    <>
      {user?.isVerified && (
        <Alert
          type="success"
          showIcon
          icon={<CheckCircleOutlined />}
          title={tc("verified", "Verified")}
          description={tc("verifiedDescription", "Your identity has been verified.")}
          style={{ marginBottom: 16 }}
        />
      )}
      <Form
        form={realNameForm}
        layout="vertical"
        onFinish={async (values) => {
          // PATCH /auth/profile 是部分合并：nil = 不改，空串 = 删除。证件号原文
          // 不回传，表单留空表示「保持不变」，必须剔除该键而不是提交空串。
          const payload = { ...(values as UpdateUserProfilePayload) };
          if (!payload.idCard) delete payload.idCard;
          // Await the mutation: a fire-and-forget save releases the submit lock
          // immediately and lets a double click submit twice.
          try {
            await updateProfileMutation.mutateAsync(payload);
          } catch (error) {
            // 裸 Form 没有 ResourceFormPage 的集中标红，这里自行落字段红字。
            if (isHttpError(error)) {
              applyServerFieldErrors(
                realNameForm,
                error,
                buildServerFieldResolver(realNameForm),
                tc,
              );
            }
          }
        }}
      >
        <Form.Item
          label={t("user.realName")}
          name="realName"
          rules={[{ required: true, message: t("user.realNameRequired") }]}
        >
          <Input disabled={user?.isVerified} />
        </Form.Item>
        <Form.Item
          label={t("user.idCardType")}
          name="idCardType"
          rules={[{ required: true, message: t("user.idCardTypeRequired") }]}
        >
          <Select disabled={user?.isVerified} placeholder={t("user.selectIdCardType")}>
            <Select.Option value="id_card">{t("user.idCardTypeIdCard")}</Select.Option>
            <Select.Option value="passport">{t("user.idCardTypePassport")}</Select.Option>
            <Select.Option value="driving_license">
              {t("user.idCardTypeDriverLicense")}
            </Select.Option>
            <Select.Option value="other">{t("user.idCardTypeOther")}</Select.Option>
          </Select>
        </Form.Item>
        <Form.Item
          label={t("user.idCard")}
          name="idCard"
          rules={[
            {
              // 已存储证件号时允许留空（= 保持不变）；未存储时仍必填。
              validator: (_rule, value: string | undefined) =>
                value || idCardSet
                  ? Promise.resolve()
                  : Promise.reject(new Error(t("user.idCardRequired"))),
            },
          ]}
          extra={idCardSet ? t("user.documentNumberKeepHint", { masked: idCardMasked }) : undefined}
        >
          <Input disabled={user?.isVerified} placeholder={idCardMasked || undefined} />
        </Form.Item>
        <Form.Item>
          <Space>
            <Button
              type="primary"
              htmlType="submit"
              loading={updateProfileMutation.isPending}
              disabled={user?.isVerified}
            >
              {t("account.saveChanges")}
            </Button>
            {!user?.isVerified && (
              <Button
                type="primary"
                icon={<CheckCircleOutlined />}
                onClick={() => verifyIdMutation.mutate()}
                loading={verifyIdMutation.isPending}
                disabled={!canVerify}
              >
                {idvCollecting ? tc("verifyIdCollecting") : t("user.verifyIdentity")}
              </Button>
            )}
          </Space>
        </Form.Item>
      </Form>
    </>
  );
}
