import { Alert, App, Button, Card, Space, Steps, Table, Typography, Upload } from "antd";
import { UploadOutlined } from "@ant-design/icons";
import { useQueryClient } from "@tanstack/react-query";
import { type FC, useState, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useSessionStore } from "@/features/session/session-store";
import { importUsersFromExcel } from "@/shared/api/iam/user";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { requestDocumentJson } from "@/shared/api/http";
import { queryKeys } from "@/shared/api/query-keys";

import { USER_UPLOAD_PREVIEW } from "@/shared/api/paths";
import { localizedError } from "@/shared/lib/server-error";

type PreviewUser = {
  row: number;
  username: string;
  displayName: string;
  email: string;
  phone: string;
  userType: string;
  status: string;
  error?: string;
};

type PreviewResult = {
  totalRows: number;
  users: PreviewUser[];
};

const UserImportPage: FC = () => {
  const { t } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const account = useSessionStore((state) => state.account);
  // 列表页跳转时会把当前组织过滤带进 URL；直接访问时回落到会话组织。
  // URL 不是可信凭据：后端 trustedOrgID 仍按会话裁决，这里只是让目标组织
  // 与用户在列表上看到的一致。
  const [searchParams] = useSearchParams();
  const organizationId = searchParams.get("organizationId") ?? account?.organization?.id ?? "";
  const navigate = useNavigate();
  // 深链直接进入时无浏览器历史可退（history.state.idx===0），回退到用户列表兜底。
  const goBack = () => {
    const idx = (window.history.state as { idx?: number } | null)?.idx ?? 0;
    if (idx > 0) navigate(-1);
    else navigate("/users");
  };

  const [step, setStep] = useState(0);
  const [file, setFile] = useState<File | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [importLoading, setImportLoading] = useState(false);
  const [preview, setPreview] = useState<PreviewResult | null>(null);
  const [importSuccess, setImportSuccess] = useState(0);
  const [importFailed, setImportFailed] = useState(0);
  const [importErrors, setImportErrors] = useState<string[]>([]);

  const handlePreview = useCallback(async () => {
    if (!file) return;
    setPreviewLoading(true);
    try {
      const formData = new FormData();
      formData.append("file", file);
      const query = organizationId ? `?organizationId=${organizationId}` : "";
      const result = await requestDocumentJson(`${USER_UPLOAD_PREVIEW}${query}`, {
        method: "POST",
        body: formData,
      });
      setPreview(result as PreviewResult);
      setStep(1);
    } catch (err) {
      message.error(localizedError(err, t, "common:error"));
    } finally {
      setPreviewLoading(false);
    }
  }, [file, organizationId, message, t]);

  const handleImport = useCallback(async () => {
    if (!file) return;
    setImportLoading(true);
    try {
      // 后端 /users/upload 从 query 的 organizationId 解析租户；与预览保持同一组织。
      const result = await importUsersFromExcel(file, organizationId || undefined);
      setImportSuccess(result.succeeded);
      setImportFailed(result.failed);
      setImportErrors(result.errors ?? []);
      setStep(2);
      // 导入不经本页 mutation，必须显式失效用户列表缓存，否则返回列表仍是旧数据。
      await queryClient.invalidateQueries({ queryKey: queryKeys.users.all });
    } catch (err) {
      message.error(localizedError(err, t, "common:error"));
    } finally {
      setImportLoading(false);
    }
  }, [file, organizationId, queryClient, message, t]);

  const handleReset = useCallback(() => {
    setStep(0);
    setFile(null);
    setPreview(null);
    setImportSuccess(0);
    setImportFailed(0);
    setImportErrors([]);
  }, []);

  const validCount = preview ? preview.users.filter((u) => !u.error).length : 0;
  const errorCount = preview ? preview.users.filter((u) => u.error).length : 0;

  const columns = [
    {
      title: t("userImport.row"),
      dataIndex: "row",
      width: 60,
    },
    {
      title: t("userImport.username"),
      dataIndex: "username",
    },
    {
      title: t("userImport.displayName"),
      dataIndex: "displayName",
    },
    {
      title: t("userImport.email"),
      dataIndex: "email",
    },
    {
      title: t("userImport.phone"),
      dataIndex: "phone",
    },
    {
      title: t("userImport.type"),
      dataIndex: "userType",
      width: 80,
    },
    {
      title: t("userImport.status"),
      dataIndex: "status",
      width: 80,
    },
    {
      title: t("userImport.validation"),
      dataIndex: "error",
      width: 150,
      render: (error: string | undefined) =>
        error ? (
          <Typography.Text type="danger">{error}</Typography.Text>
        ) : (
          <Typography.Text type="success">{t("userImport.valid")}</Typography.Text>
        ),
    },
  ];

  return (
    <PermissionGuard action="users:write" resource="users">
      <div className="gl-resource-page gl-content__inner--form">
        <Card variant="borderless">
          <Typography.Title level={3}>{t("userImport.title")}</Typography.Title>

          <Steps
            current={step}
            style={{ marginBottom: 32 }}
            items={[
              {
                title: t("userImport.upload"),
              },
              {
                title: t("userImport.preview"),
              },
              {
                title: t("userImport.result"),
              },
            ]}
          />

          {/* Step 0: Upload */}
          {step === 0 && (
            <>
              <Alert
                type="info"
                showIcon
                title={t("userImport.templateHint", {})}
                style={{ marginBottom: 16 }}
              />
              <Upload
                beforeUpload={(f) => {
                  setFile(f);
                  return false;
                }}
                accept=".xlsx,.xls,.csv"
                maxCount={1}
                fileList={
                  file
                    ? [
                        {
                          uid: "-1",
                          name: file.name,
                          status: "done" as const,
                        },
                      ]
                    : []
                }
                onRemove={() => setFile(null)}
              >
                <Button icon={<UploadOutlined />}>{t("batchImport.selectFile")}</Button>
              </Upload>
              <div style={{ marginTop: 24, textAlign: "right" }}>
                <Space>
                  <Button onClick={goBack}>{t("action.cancel")}</Button>
                  <Button
                    type="primary"
                    disabled={!file}
                    loading={previewLoading}
                    onClick={handlePreview}
                  >
                    {t("userImport.next")}
                  </Button>
                </Space>
              </div>
            </>
          )}

          {/* Step 1: Preview */}
          {step === 1 && preview && (
            <>
              <Space style={{ marginBottom: 16 }}>
                <Typography.Text>
                  {t("userImport.totalRows")}: {preview.totalRows}
                </Typography.Text>
                <Typography.Text type="success">
                  {t("userImport.valid")}: {validCount}
                </Typography.Text>
                {errorCount > 0 && (
                  <Typography.Text type="danger">
                    {t("userImport.errors")}: {errorCount}
                  </Typography.Text>
                )}
              </Space>
              <Table
                columns={columns}
                dataSource={preview.users}
                rowKey="row"
                size="small"
                pagination={{ pageSize: 20 }}
                scroll={{ x: 800 }}
              />
              <div style={{ marginTop: 24, textAlign: "right" }}>
                <Space>
                  <Button onClick={() => setStep(0)}>{t("action.back")}</Button>
                  <Button
                    type="primary"
                    loading={importLoading}
                    disabled={validCount === 0}
                    onClick={handleImport}
                  >
                    {t("action.import")}
                  </Button>
                </Space>
              </div>
            </>
          )}

          {/* Step 2: Result */}
          {step === 2 && (
            <>
              <Alert
                type={importFailed === 0 ? "success" : "warning"}
                showIcon
                title={t("userImport.complete", {})}
                description={`${t("userImport.succeeded")}: ${importSuccess}, ${t("userImport.failed")}: ${importFailed}`}
                style={{ marginBottom: 16 }}
              />
              {importErrors.length > 0 && (
                <Alert
                  type="error"
                  showIcon
                  style={{ marginBottom: 16 }}
                  title={t("userImport.errors")}
                  description={
                    <ul style={{ margin: 0, paddingLeft: 16 }}>
                      {importErrors.slice(0, 10).map((error, index) => (
                        <li key={index}>{error}</li>
                      ))}
                      {importErrors.length > 10 && <li>...{importErrors.length - 10} more</li>}
                    </ul>
                  }
                />
              )}
              <div style={{ textAlign: "right" }}>
                <Space>
                  <Button onClick={handleReset}>{t("userImport.importMore", {})}</Button>
                  <Button type="primary" onClick={goBack}>
                    {t("action.done")}
                  </Button>
                </Space>
              </div>
            </>
          )}
        </Card>
      </div>
    </PermissionGuard>
  );
};

export default UserImportPage;
