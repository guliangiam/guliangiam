import type { FormInstance } from "antd";
import { App, Button, Col, DatePicker, Form, Input, Row, Select, Tag, Upload } from "antd";
import { CheckCircleOutlined, SafetyCertificateOutlined, ScanOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ImageCropUpload } from "@/shared/ui/ImageCropUpload";
import { FormSection } from "@/shared/ui/FormSection";
import { IDCARD_TYPE_OPTIONS } from "./types";
import type { UserDetailFormValues } from "./types";
import type { IdentityDocumentType } from "@/shared/api/iam/user";

interface UserRealNameTabProps {
  form: FormInstance<UserDetailFormValues>;
  owner?: string;
  name?: string;
  isVerified?: boolean;
  onVerifyIdentification?: () => void;
  isVerifying?: boolean;
  /** SDK 采集阶段按钮改文案，避免点了像没反应。 */
  verifyingLabel?: string;
  canWrite?: boolean;
  onRecognizeDocument?: (
    documentType: IdentityDocumentType,
    file: File,
    side?: "front" | "back",
  ) => void;
  recognizing?: boolean;
  /** 规范 9.5：证件号永不回传原文。已存储时表单留空 = 保持不变，
   * Set/Masked 只用于提示与「验证身份」可用性判断。 */
  idCardSet?: boolean;
  idCardMasked?: string;
  passportNoSet?: boolean;
  passportNoMasked?: string;
  driverLicenseNoSet?: boolean;
  driverLicenseNoMasked?: string;
}

const DOCUMENT_TYPE_OPTIONS: { value: IdentityDocumentType; labelKey: string }[] = [
  { value: "id_card", labelKey: "resource:user.idCardTypeIdCard" },
  { value: "passport", labelKey: "resource:user.idCardTypePassport" },
  { value: "driver_license", labelKey: "resource:user.idCardTypeDriverLicense" },
];

export default function UserRealNameTab({
  form,
  owner,
  name,
  isVerified = false,
  onVerifyIdentification,
  isVerifying = false,
  verifyingLabel,
  canWrite = false,
  onRecognizeDocument,
  recognizing = false,
  idCardSet = false,
  idCardMasked = "",
  passportNoSet = false,
  passportNoMasked = "",
  driverLicenseNoSet = false,
  driverLicenseNoMasked = "",
}: UserRealNameTabProps) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [documentType, setDocumentType] = useState<IdentityDocumentType>("id_card");
  const [documentSide, setDocumentSide] = useState<"front" | "back">("front");

  // 监听表单中的证件类型值，驱动条件渲染
  const formIdCardType = Form.useWatch("idCardType", form) as string | undefined;
  const idCardType = formIdCardType ?? "id_card";

  const handleVerifyIdentification = () => {
    const realName = form.getFieldValue("realName");
    const idCardTypeValue = form.getFieldValue("idCardType");
    const idCard = form.getFieldValue("idCard");
    // 证件号原文不回传（规范 9.5）：已存储时表单留空不代表缺失。
    if (!realName || !idCardTypeValue || (!idCard && !idCardSet)) {
      message.warning(t("verifyIdRequired"));
      return;
    }
    onVerifyIdentification?.();
  };

  const documentNumberKeepHint = (isSet: boolean, masked: string) =>
    isSet
      ? t("resource:user.documentNumberKeepHint", { masked, defaultValue: "" }) || undefined
      : undefined;

  const handleOcrUpload = (file: File) => {
    onRecognizeDocument?.(
      documentType,
      file,
      documentType === "id_card" ? documentSide : undefined,
    );
    // Prevent antd from uploading on its own; the OCR request owns the file.
    return false;
  };

  const disabled = isVerified;

  return (
    <>
      {/* 1. 证件图片上传（放最前，最直观的实名材料入口） */}
      <FormSection title={t("idCardImages")}>
        {/* ─── 身份证：正面 / 反面 / 手持 ─── */}
        {idCardType === "id_card" && (
          <>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label={t("idCardFront", "ID Card Front")}>
                  <ImageCropUpload
                    value={form.getFieldValue("idCardFront" as never)}
                    onChange={(url) => form.setFieldValue("idCardFront" as never, url)}
                    tag="id-card-front"
                    owner={owner}
                    name={name}
                    shape="square"
                    label={t("upload")}
                    disabled={disabled}
                  />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label={t("idCardBack", "ID Card Back")}>
                  <ImageCropUpload
                    value={form.getFieldValue("idCardBack" as never)}
                    onChange={(url) => form.setFieldValue("idCardBack" as never, url)}
                    tag="id-card-back"
                    owner={owner}
                    name={name}
                    shape="square"
                    label={t("upload")}
                    disabled={disabled}
                  />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label={t("idCardWithPerson", "ID Card with Person")}>
                  <ImageCropUpload
                    value={form.getFieldValue("idCardWithPerson" as never)}
                    onChange={(url) => form.setFieldValue("idCardWithPerson" as never, url)}
                    tag="id-card-person"
                    owner={owner}
                    name={name}
                    shape="square"
                    label={t("upload")}
                    disabled={disabled}
                  />
                </Form.Item>
              </Col>
            </Row>
          </>
        )}

        {/* ─── 护照：护照照片 ─── */}
        {idCardType === "passport" && (
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                label={t("resource:user.passportPhoto", { defaultValue: "Passport Photo" })}
              >
                <ImageCropUpload
                  value={form.getFieldValue("passportPhotoUrl" as never)}
                  onChange={(url) => form.setFieldValue("passportPhotoUrl" as never, url)}
                  tag="passport-photo"
                  owner={owner}
                  name={name}
                  shape="square"
                  label={t("upload")}
                  disabled={disabled}
                />
              </Form.Item>
            </Col>
          </Row>
        )}

        {/* ─── 驾照：正面 / 反面 ─── */}
        {idCardType === "driver_license" && (
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                label={t("resource:user.driverLicenseFront", {
                  defaultValue: "Driver License Front",
                })}
              >
                <ImageCropUpload
                  value={form.getFieldValue("driverLicenseFrontUrl" as never)}
                  onChange={(url) => form.setFieldValue("driverLicenseFrontUrl" as never, url)}
                  tag="driver-license-front"
                  owner={owner}
                  name={name}
                  shape="square"
                  label={t("upload")}
                  disabled={disabled}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label={t("resource:user.driverLicenseBack", {
                  defaultValue: "Driver License Back",
                })}
              >
                <ImageCropUpload
                  value={form.getFieldValue("driverLicenseBackUrl" as never)}
                  onChange={(url) => form.setFieldValue("driverLicenseBackUrl" as never, url)}
                  tag="driver-license-back"
                  owner={owner}
                  name={name}
                  shape="square"
                  label={t("upload")}
                  disabled={disabled}
                />
              </Form.Item>
            </Col>
          </Row>
        )}
      </FormSection>

      {/* 2. OCR 智能识别（利用上传的材料一键填充） */}
      <FormSection
        title={t("resource:user.documentRecognition", { defaultValue: "Document recognition" })}
        style={{ marginTop: 16 }}
      >
        <Row gutter={16} align="bottom">
          <Col xs={24} sm={12} md={8}>
            <Form.Item
              label={t("resource:user.documentType.label", { defaultValue: "Document type" })}
            >
              <Select
                value={documentType}
                disabled={isVerified || !canWrite}
                onChange={(value) => setDocumentType(value)}
                options={DOCUMENT_TYPE_OPTIONS.map((option) => ({
                  value: option.value,
                  label: t(option.labelKey),
                }))}
              />
            </Form.Item>
          </Col>
          {documentType === "id_card" && (
            <Col xs={24} sm={12} md={8}>
              <Form.Item label={t("resource:user.documentSideLabel", { defaultValue: "Side" })}>
                <Select
                  value={documentSide}
                  disabled={isVerified || !canWrite}
                  onChange={(value) => setDocumentSide(value)}
                  options={[
                    {
                      value: "front",
                      label: t("resource:user.documentSide.front", { defaultValue: "Front" }),
                    },
                    {
                      value: "back",
                      label: t("resource:user.documentSide.back", { defaultValue: "Back" }),
                    },
                  ]}
                />
              </Form.Item>
            </Col>
          )}
          <Col xs={24} sm={12} md={8}>
            <Form.Item label=" ">
              <Upload
                accept="image/jpeg,image/png,image/webp"
                showUploadList={false}
                beforeUpload={handleOcrUpload}
                disabled={isVerified || !canWrite || recognizing}
              >
                <Button icon={<ScanOutlined />} loading={recognizing}>
                  {t("resource:user.recognizeDocument", {
                    defaultValue: "Upload & recognize",
                  })}
                </Button>
              </Upload>
            </Form.Item>
          </Col>
        </Row>
        <div style={{ color: "var(--ant-color-text-secondary)", fontSize: 12 }}>
          {t("resource:user.documentRecognitionHint", {
            defaultValue:
              "The provider fills the name, document type and number below. Review them before saving; nothing is stored until you save.",
          })}
        </div>
      </FormSection>

      {/* 3. 实名字段表单（放在上传 + OCR 之后，用户通常先传/识别再核对） */}
      <FormSection title={t("resource:user.realNameInfoSection", "Real Name Information")}>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label={t("realName")} name="realName">
              <Input disabled={disabled} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label={t("resource:user.idCardType")} name="idCardType">
              <Select options={IDCARD_TYPE_OPTIONS} allowClear disabled={disabled} />
            </Form.Item>
          </Col>
        </Row>

        {/* ─── 身份证号（仅 id_card 显示） ─── */}
        {idCardType === "id_card" && (
          <Row gutter={16}>
            <Col span={24}>
              <Form.Item
                label={t("resource:user.idCard")}
                name="idCard"
                extra={documentNumberKeepHint(idCardSet, idCardMasked)}
              >
                <Input disabled={disabled} placeholder={idCardMasked || undefined} />
              </Form.Item>
            </Col>
          </Row>
        )}

        {/* ─── 护照字段 ─── */}
        {idCardType === "passport" && (
          <>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("resource:user.passportNo", { defaultValue: "Passport Number" })}
                  name="passportNo"
                  extra={documentNumberKeepHint(passportNoSet, passportNoMasked)}
                >
                  <Input disabled={disabled} placeholder={passportNoMasked || undefined} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={t("resource:user.issueCountry", { defaultValue: "Issue Country" })}
                  name="issueCountry"
                >
                  <Input disabled={disabled} placeholder="e.g. CHN" />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("resource:user.issuingAuthority", {
                    defaultValue: "Issuing Authority",
                  })}
                  name="issuingAuthority"
                >
                  <Input disabled={disabled} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={t("resource:user.idCardExpiry", {
                    defaultValue: "Document Expiry",
                  })}
                  name="idCardExpiry"
                  getValueProps={(v) => (v ? { value: dayjs(v) } : { value: undefined })}
                >
                  <DatePicker disabled={disabled} style={{ width: "100%" }} format="YYYY-MM-DD" />
                </Form.Item>
              </Col>
            </Row>
          </>
        )}

        {/* ─── 驾照字段 ─── */}
        {idCardType === "driver_license" && (
          <>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("resource:user.driverLicenseNo", {
                    defaultValue: "Driver License Number",
                  })}
                  name="driverLicenseNo"
                  extra={documentNumberKeepHint(driverLicenseNoSet, driverLicenseNoMasked)}
                >
                  <Input disabled={disabled} placeholder={driverLicenseNoMasked || undefined} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={t("resource:user.issuingAuthority", {
                    defaultValue: "Issuing Authority",
                  })}
                  name="issuingAuthority"
                >
                  <Input disabled={disabled} />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("resource:user.idCardExpiry", {
                    defaultValue: "Document Expiry",
                  })}
                  name="idCardExpiry"
                  getValueProps={(v) => (v ? { value: dayjs(v) } : { value: undefined })}
                >
                  <DatePicker disabled={disabled} style={{ width: "100%" }} format="YYYY-MM-DD" />
                </Form.Item>
              </Col>
            </Row>
          </>
        )}
      </FormSection>

      {/* 地址子模块已迁至「资料」分区（basic），此处不再承载多地址集合。 */}

      {/* 4. 身份认证状态 + 触发按钮 */}
      <FormSection title={t("identityVerification")} style={{ marginTop: 16 }}>
        {isVerified ? (
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <Tag
              icon={<CheckCircleOutlined />}
              color="success"
              style={{ fontSize: 14, padding: "4px 12px" }}
            >
              {t("verified")}
            </Tag>
            <span style={{ color: "var(--ant-color-text-secondary)" }}>
              {t("verifiedDescription")}
            </span>
          </div>
        ) : (
          <Button
            icon={<SafetyCertificateOutlined />}
            onClick={handleVerifyIdentification}
            loading={isVerifying}
          >
            {verifyingLabel ?? t("verifyIdentification")}
          </Button>
        )}
      </FormSection>
    </>
  );
}
