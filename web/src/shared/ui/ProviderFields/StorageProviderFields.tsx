import { Form, Input } from "antd";
import { useTranslation } from "react-i18next";

export type StorageProviderFieldsProps = {
  value: Record<string, string>;
  onChange: (field: string, val: string) => void;
  subType?: string;
};

/** Types that need endpoint (MinIO-style S3-compatible) */
const ENDPOINT_TYPES = new Set([
  "MinIO",
  "AliOSS",
  "TencentCOS",
  "HuaweiOBS",
  "Qiniu",
  "CUCloud",
  "CloudflareR2",
  "BackblazeB2",
  "DigitalOceanSpaces",
  "Synology",
  "Custom",
]);
/** Types that need bucket */
const BUCKET_TYPES = new Set([
  "AWS S3",
  "MinIO",
  "AliOSS",
  "TencentCOS",
  "HuaweiOBS",
  "Qiniu",
  "CUCloud",
  "GCS",
  "AzureBlob",
  "CloudflareR2",
  "BackblazeB2",
  "DigitalOceanSpaces",
  "Synology",
  "Custom",
]);
/** Types that need region */
const REGION_TYPES = new Set([
  "AWS S3",
  "AliOSS",
  "TencentCOS",
  "HuaweiOBS",
  "GCS",
  "CloudflareR2",
  "BackblazeB2",
  "DigitalOceanSpaces",
  "Custom",
]);
/** Types that need accessKey/secretKey */
const CREDENTIAL_TYPES = new Set([
  "AWS S3",
  "MinIO",
  "AliOSS",
  "TencentCOS",
  "HuaweiOBS",
  "Qiniu",
  "CUCloud",
  "CloudflareR2",
  "BackblazeB2",
  "DigitalOceanSpaces",
  "Synology",
  "Custom",
]);

export function StorageProviderFields({ value, onChange, subType }: StorageProviderFieldsProps) {
  const { t } = useTranslation("common");

  const selectedType = subType ?? value.providerType ?? "";

  return (
    <>
      {selectedType === "Local" && (
        <Form.Item label="Base Directory">
          <Input
            value={value.endpoint ?? ""}
            onChange={(e) => onChange("endpoint", e.target.value)}
            placeholder="/data/uploads"
          />
        </Form.Item>
      )}

      {ENDPOINT_TYPES.has(selectedType) && (
        <Form.Item label={t("endpoint")}>
          <Input
            value={value.endpoint ?? ""}
            onChange={(e) => onChange("endpoint", e.target.value)}
            placeholder={
              selectedType === "MinIO"
                ? "minio.example.com:9000"
                : selectedType === "AliOSS"
                  ? "oss-cn-hangzhou.aliyuncs.com"
                  : selectedType === "TencentCOS"
                    ? "cos.ap-guangzhou.myqcloud.com"
                    : selectedType === "HuaweiOBS"
                      ? "obs.cn-north-4.myhuaweicloud.com"
                      : selectedType === "Qiniu"
                        ? "s3.qiniucs.com"
                        : selectedType === "CloudflareR2"
                          ? "<account_id>.r2.cloudflarestorage.com"
                          : selectedType === "BackblazeB2"
                            ? "s3.us-west-004.backblazeb2.com"
                            : selectedType === "DigitalOceanSpaces"
                              ? "nyc3.digitaloceanspaces.com"
                              : "Endpoint URL"
            }
          />
        </Form.Item>
      )}

      {BUCKET_TYPES.has(selectedType) && (
        <Form.Item label={t("bucket")}>
          <Input
            value={value.bucket ?? ""}
            onChange={(e) => onChange("bucket", e.target.value)}
            placeholder={selectedType === "AzureBlob" ? "Container name" : "Bucket name"}
          />
        </Form.Item>
      )}

      {REGION_TYPES.has(selectedType) && (
        <Form.Item label={t("region")}>
          <Input
            value={value.region ?? ""}
            onChange={(e) => onChange("region", e.target.value)}
            placeholder={
              selectedType === "AWS S3"
                ? "us-east-1"
                : selectedType === "AliOSS"
                  ? "cn-hangzhou"
                  : selectedType === "TencentCOS"
                    ? "ap-guangzhou"
                    : selectedType === "HuaweiOBS"
                      ? "cn-north-4"
                      : selectedType === "CloudflareR2"
                        ? "auto"
                        : selectedType === "BackblazeB2"
                          ? "us-west-004"
                          : selectedType === "DigitalOceanSpaces"
                            ? "nyc3"
                            : "Region"
            }
          />
        </Form.Item>
      )}

      {selectedType === "GCS" && (
        <Form.Item label="Credentials JSON">
          <Input.TextArea
            value={value.accessKeyId ?? ""}
            onChange={(e) => onChange("accessKeyId", e.target.value)}
            placeholder='{"type":"service_account","project_id":"..."}'
            rows={3}
          />
        </Form.Item>
      )}

      {selectedType === "AzureBlob" && (
        <>
          <Form.Item label="Account Name">
            <Input
              value={value.accessKeyId ?? ""}
              onChange={(e) => onChange("accessKeyId", e.target.value)}
              placeholder="Storage account name"
            />
          </Form.Item>
          <Form.Item label="Account Key">
            <Input.Password
              value={value.accessKeySecret ?? ""}
              onChange={(e) => onChange("accessKeySecret", e.target.value)}
              placeholder="Storage account key"
            />
          </Form.Item>
        </>
      )}

      {CREDENTIAL_TYPES.has(selectedType) && (
        <>
          <Form.Item label={t("accessKeyId")}>
            <Input
              value={value.accessKeyId ?? ""}
              onChange={(e) => onChange("accessKeyId", e.target.value)}
              placeholder="Access Key ID"
            />
          </Form.Item>
          <Form.Item label={t("accessKeySecret")}>
            <Input.Password
              value={value.accessKeySecret ?? ""}
              onChange={(e) => onChange("accessKeySecret", e.target.value)}
              placeholder="Access Key Secret"
            />
          </Form.Item>
        </>
      )}
    </>
  );
}
