import { UploadOutlined } from "@ant-design/icons";
import {
  Alert,
  App,
  Button,
  Modal,
  Progress,
  Space,
  Steps,
  Typography,
  Upload,
  type UploadFile,
} from "antd";
import { type FC, useState, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { localizedError } from "@/shared/lib/server-error";

type BatchImportModalProps = {
  open: boolean;
  onClose: () => void;
  onImport: (file: File) => Promise<ImportResult>;
  title?: string;
  accept?: string;
  templateHint?: string;
};

export type ImportResult = {
  total: number;
  succeeded: number;
  failed: number;
  errors?: string[];
};

export const BatchImportModal: FC<BatchImportModalProps> = ({
  open,
  onClose,
  onImport,
  title,
  accept = ".xlsx,.xls,.csv",
  templateHint,
}) => {
  const { t } = useTranslation("common");
  const { message } = App.useApp();
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [loading, setLoading] = useState(false);
  const [step, setStep] = useState<0 | 1 | 2>(0);
  const [result, setResult] = useState<ImportResult | null>(null);

  const handleImport = useCallback(async () => {
    const file = fileList[0]?.originFileObj;
    if (!file) return;

    setLoading(true);
    try {
      const res = await onImport(file);
      setResult(res);
      setStep(1);
    } catch (err) {
      message.error(localizedError(err, t, "common:error"));
    } finally {
      setLoading(false);
    }
  }, [fileList, message, onImport, t]);

  const handleDone = useCallback(() => {
    setFileList([]);
    setStep(0);
    setResult(null);
    onClose();
  }, [onClose]);

  const beforeUpload = useCallback(() => false, []);

  return (
    <Modal
      open={open}
      title={title ?? t("batchImport.title")}
      onCancel={handleDone}
      footer={null}
      width={520}
      destroyOnHidden
    >
      <Steps
        current={step}
        size="small"
        style={{ marginBottom: 24 }}
        items={[{ title: t("batchImport.upload") }, { title: t("batchImport.result") }]}
      />

      {step === 0 && (
        <>
          {templateHint && (
            <Alert type="info" showIcon title={templateHint} style={{ marginBottom: 16 }} />
          )}
          <Upload
            beforeUpload={beforeUpload}
            fileList={fileList}
            onChange={({ fileList: list }) => setFileList(list.slice(-1))}
            accept={accept}
            maxCount={1}
          >
            <Button icon={<UploadOutlined />}>{t("batchImport.selectFile")}</Button>
          </Upload>
          <div style={{ marginTop: 24, textAlign: "right" }}>
            <Space>
              <Button onClick={handleDone}>{t("action.cancel")}</Button>
              <Button
                type="primary"
                loading={loading}
                disabled={fileList.length === 0}
                onClick={handleImport}
              >
                {t("action.import")}
              </Button>
            </Space>
          </div>
        </>
      )}

      {step === 1 && result && (
        <>
          <div style={{ textAlign: "center", marginBottom: 16 }}>
            <Progress
              type="circle"
              percent={result.total > 0 ? Math.round((result.succeeded / result.total) * 100) : 0}
              format={() => `${result.succeeded}/${result.total}`}
            />
          </div>
          <Space orientation="vertical" style={{ width: "100%" }}>
            <Typography.Text>
              {t("batchImport.total")}: {result.total}
            </Typography.Text>
            <Typography.Text type="success">
              {t("batchImport.succeeded")}: {result.succeeded}
            </Typography.Text>
            <Typography.Text type="danger">
              {t("batchImport.failed")}: {result.failed}
            </Typography.Text>
          </Space>
          {result.errors && result.errors.length > 0 && (
            <Alert
              type="error"
              style={{ marginTop: 12 }}
              title={t("batchImport.errorDetails")}
              description={
                <ul style={{ margin: 0, paddingLeft: 20 }}>
                  {result.errors.slice(0, 10).map((err, idx) => (
                    <li key={idx}>{err}</li>
                  ))}
                  {result.errors.length > 10 && <li>...{result.errors.length - 10} more</li>}
                </ul>
              }
            />
          )}
          <div style={{ marginTop: 24, textAlign: "right" }}>
            <Button type="primary" onClick={handleDone}>
              {t("action.done")}
            </Button>
          </div>
        </>
      )}
    </Modal>
  );
};
