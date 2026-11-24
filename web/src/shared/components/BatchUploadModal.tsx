import { useState } from "react";
import { Modal, Upload, Button, Table, Alert, type UploadFile } from "antd";
import { UploadOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";

type BulkImportResult = {
  total: number;
  succeeded: number;
  failed: number;
  errors?: string[];
};

export type BatchUploadTemplate = {
  /** 下载文件名（如 groups-import-template.csv） */
  fileName: string;
  /** 模板列头，需与后端导入解析的列顺序一致 */
  columns: string[];
  /** 可选示例行 */
  sample?: string[];
};

type BatchUploadModalProps = {
  open: boolean;
  onClose: () => void;
  onUpload: (file: File) => Promise<BulkImportResult>;
  title: string;
  accept?: string;
  /** 外部已选好的文件（例如单入口导入按扩展名分流后交给本弹窗） */
  initialFile?: File | null;
  /** 提供后展示「下载模板」按钮（前端静态生成 CSV） */
  template?: BatchUploadTemplate;
};

export function BatchUploadModal({
  open,
  onClose,
  onUpload,
  title,
  accept = ".xlsx,.xls",
  initialFile = null,
  template,
}: BatchUploadModalProps) {
  const { t: tc } = useTranslation("common");

  const downloadTemplate = () => {
    if (!template) return;
    const rows = [template.columns, ...(template.sample ? [template.sample] : [])];
    // BOM 保证 Excel 打开 UTF-8 CSV 不乱码；填完后另存为 .xlsx 再上传。
    const csv = "\uFEFF" + rows.map((row) => row.join(",")).join("\r\n");
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = template.fileName;
    anchor.click();
    URL.revokeObjectURL(url);
  };
  // null 表示用户尚未在弹窗内改选文件，此时沿用外部传入的 initialFile。
  const [selectedFiles, setSelectedFiles] = useState<UploadFile[] | null>(null);
  const [uploading, setUploading] = useState(false);
  const [result, setResult] = useState<BulkImportResult | null>(null);

  const fileList: UploadFile[] =
    selectedFiles ??
    (initialFile
      ? [
          {
            uid: "initial",
            name: initialFile.name,
            status: "done",
            originFileObj: initialFile as UploadFile["originFileObj"],
          },
        ]
      : []);

  const handleUpload = async () => {
    if (fileList.length === 0) return;
    setUploading(true);
    setResult(null);
    try {
      const res = await onUpload(fileList[0].originFileObj as File);
      setResult(res);
    } catch {
      setResult({ total: 0, succeeded: 0, failed: 1, errors: ["Upload failed"] });
    } finally {
      setUploading(false);
    }
  };

  const handleClose = () => {
    setSelectedFiles(null);
    setResult(null);
    onClose();
  };

  return (
    <Modal
      title={title}
      open={open}
      onCancel={handleClose}
      footer={[
        <Button key="cancel" onClick={handleClose}>
          {tc("cancel")}
        </Button>,
        <Button
          key="upload"
          type="primary"
          loading={uploading}
          disabled={fileList.length === 0}
          onClick={handleUpload}
        >
          {tc("import")}
        </Button>,
      ]}
    >
      <Upload
        accept={accept}
        maxCount={1}
        beforeUpload={() => false}
        fileList={fileList}
        onChange={({ fileList: newFileList }) => setSelectedFiles(newFileList)}
      >
        <Button icon={<UploadOutlined />}>{tc("selectFile")}</Button>
      </Upload>

      {template ? (
        <div style={{ marginTop: 8 }}>
          <Button type="link" size="small" style={{ paddingLeft: 0 }} onClick={downloadTemplate}>
            {tc("downloadTemplate")}
          </Button>
          <div style={{ fontSize: 12, color: "rgba(0,0,0,0.45)" }}>{tc("templateCsvHint")}</div>
        </div>
      ) : null}

      {result && (
        <div style={{ marginTop: 16 }}>
          <Alert
            type={result.failed === 0 ? "success" : "warning"}
            title={result.failed === 0 ? tc("importSuccess") : tc("importPartial")}
            description={
              <Table
                size="small"
                pagination={false}
                dataSource={[
                  { key: "total", label: tc("total"), value: result.total },
                  { key: "succeeded", label: tc("succeeded"), value: result.succeeded },
                  { key: "failed", label: tc("failed"), value: result.failed },
                ]}
                columns={[
                  { dataIndex: "label", key: "label" },
                  { dataIndex: "value", key: "value" },
                ]}
              />
            }
          />
          {result.errors && result.errors.length > 0 && (
            <Alert
              style={{ marginTop: 8 }}
              type="error"
              title={tc("importErrors")}
              description={
                <ul style={{ margin: 0, paddingLeft: 16 }}>
                  {result.errors.map((e, i) => (
                    <li key={i}>{e}</li>
                  ))}
                </ul>
              }
            />
          )}
        </div>
      )}
    </Modal>
  );
}
