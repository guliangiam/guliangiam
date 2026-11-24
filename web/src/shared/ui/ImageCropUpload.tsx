import { useCallback, useRef, useState } from "react";
import { App, Avatar, Button, Input, Modal, Space, Tooltip, Upload, type UploadProps } from "antd";
import {
  ColumnWidthOutlined,
  LinkOutlined,
  RedoOutlined,
  RotateLeftOutlined,
  RotateRightOutlined,
  UploadOutlined,
  UserOutlined,
  ZoomInOutlined,
  ZoomOutOutlined,
} from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import Cropper from "cropperjs";
import "cropperjs/dist/cropper.css";
import { requestBlob } from "@/shared/api/http";

import { UPLOAD_RESOURCE } from "@/shared/api/paths";

interface ImageCropUploadProps {
  value?: string;
  onChange?: (url: string) => void;
  label?: string;
  tag?: string;
  owner?: string;
  name?: string;
  aspectRatio?: number;
  shape?: "circle" | "square";
  maxSize?: number;
  disabled?: boolean;
  /** Show CDN URL input toggle alongside upload button */
  allowUrlInput?: boolean;
}

const OUTPUT_TYPES = /^image\/(png|jpeg|webp)$/;

export function ImageCropUpload({
  value,
  onChange,
  label,
  tag = "avatar",
  owner = "",
  name = "",
  aspectRatio = 1,
  shape = "circle",
  maxSize = 5,
  disabled = false,
  allowUrlInput = false,
}: ImageCropUploadProps) {
  const { t } = useTranslation();
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const [previewOpen, setPreviewOpen] = useState(false);
  const [previewImage, setPreviewImage] = useState("");

  const [cropOpen, setCropOpen] = useState(false);
  const [cropSrc, setCropSrc] = useState("");
  const [freeAspect, setFreeAspect] = useState(true);
  const [uploading, setUploading] = useState(false);
  const imgRef = useRef<HTMLImageElement | null>(null);
  const cropperRef = useRef<Cropper | null>(null);
  const pendingFileRef = useRef<{ name: string; type: string } | null>(null);

  const handleUpload = useCallback(
    async (file: File) => {
      if (file.size > maxSize * 1024 * 1024) {
        message.error(t("common:fileTooLarge", `File must be under ${maxSize}MB`));
        return;
      }

      const formData = new FormData();
      formData.append("file", file);

      try {
        const resp = await requestBlob(
          `${UPLOAD_RESOURCE}?owner=${encodeURIComponent(owner)}&user=${encodeURIComponent(name)}&tag=${encodeURIComponent(tag)}`,
          { method: "POST", body: formData },
        );
        const result = await resp.json();
        const url =
          typeof result === "string"
            ? result
            : (((result as Record<string, unknown>)?.data as string) ?? "");
        onChange?.(url);
        message.success(t("common:uploadSuccess", "Upload successful"));
      } catch {
        message.error(t("common:uploadFailed", "Upload failed"));
      }
    },
    [tag, owner, name, maxSize, onChange, message, t],
  );

  // The crop box is freely resizable (eight drag handles), the image pans by drag,
  // zooms on the wheel, and rotates via the toolbar — no sliders. Default aspect is
  // free (NaN); the toggle re-locks it to the caller's `aspectRatio`.
  //
  // cropperjs must attach only once the <img> is mounted and decoded. antd Modal
  // mounts its portal content a tick after `open` flips, so an effect keyed on `open`
  // sees a null ref and never builds the cropper. Attach from the image's onLoad
  // (primary) and the Modal's afterOpenChange (fallback); both are idempotent.
  const destroyCropper = useCallback(() => {
    cropperRef.current?.destroy();
    cropperRef.current = null;
  }, []);

  const initCropper = useCallback(() => {
    const img = imgRef.current;
    if (!img || cropperRef.current) return;
    cropperRef.current = new Cropper(img, {
      viewMode: 1,
      dragMode: "move",
      autoCropArea: 0.8,
      background: false,
      rotatable: true,
      zoomable: true,
      zoomOnWheel: true,
      cropBoxResizable: true,
      cropBoxMovable: true,
      aspectRatio: NaN,
    });
  }, []);

  const closeCrop = useCallback(() => {
    destroyCropper();
    setCropOpen(false);
    setCropSrc("");
    setFreeAspect(true);
  }, [destroyCropper]);

  const rotateBy = (degree: number) => cropperRef.current?.rotate(degree);
  const zoomBy = (ratio: number) => cropperRef.current?.zoom(ratio);
  const resetCrop = () => cropperRef.current?.reset();
  const toggleAspect = () => {
    const next = !freeAspect;
    setFreeAspect(next);
    cropperRef.current?.setAspectRatio(next ? NaN : aspectRatio);
  };

  const handleCropOk = () => {
    const cropper = cropperRef.current;
    if (!cropper) return;
    const meta = pendingFileRef.current;
    const outType = meta?.type && OUTPUT_TYPES.test(meta.type) ? meta.type : "image/png";
    const canvas = cropper.getCroppedCanvas({
      maxWidth: 2048,
      maxHeight: 2048,
      imageSmoothingQuality: "high",
    });
    canvas.toBlob(
      (blob) => {
        if (!blob) {
          message.error(t("common:uploadFailed", "Upload failed"));
          return;
        }
        const file = new File([blob], meta?.name || "cropped.png", { type: outType });
        setUploading(true);
        void handleUpload(file)
          .then(() => closeCrop())
          .finally(() => setUploading(false));
      },
      outType,
      0.92,
    );
  };

  const uploadProps: UploadProps = {
    name: "file",
    showUploadList: false,
    accept: "image/*",
    beforeUpload: (file) => {
      const reader = new FileReader();
      reader.onload = () => {
        pendingFileRef.current = { name: file.name, type: file.type };
        setCropSrc(String(reader.result));
        setFreeAspect(true);
        setCropOpen(true);
      };
      reader.readAsDataURL(file);
      return false;
    },
  };

  const avatarSize = 96;
  const isImage = value && /\.(jpg|jpeg|png|gif|webp|svg|bmp)$/i.test(value);

  return (
    <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
      {value && isImage ? (
        <button
          type="button"
          style={{ cursor: "pointer", padding: 0, border: "none", background: "none" }}
          onClick={() => {
            setPreviewImage(value);
            setPreviewOpen(true);
          }}
          /* WCAG 4.1.2 — a native button provides the preview trigger semantics (A-04) */
          aria-label={
            label
              ? t("common:previewImage", `Preview ${label}`)
              : t("common:previewImage", "Preview image")
          }
        >
          <Avatar
            src={value}
            size={avatarSize}
            shape={shape === "circle" ? "circle" : "square"}
            alt={label}
          />
        </button>
      ) : (
        <Avatar
          size={avatarSize}
          shape={shape === "circle" ? "circle" : "square"}
          icon={<UserOutlined />}
          style={{ backgroundColor: "var(--ant-color-bg-container)" }}
        />
      )}

      <Space.Compact size="small" style={{ maxWidth: 320 }}>
        <Upload {...uploadProps} disabled={disabled}>
          <Button icon={<UploadOutlined />} disabled={disabled}>
            {label || t("common:upload", "Upload")}
          </Button>
        </Upload>

        {allowUrlInput && (
          <Tooltip title="Paste CDN image URL">
            <Input
              placeholder="https://..."
              prefix={<LinkOutlined />}
              value={value ?? ""}
              disabled={disabled}
              onChange={(e) => onChange?.(e.target.value.trim())}
              style={{ width: 180 }}
            />
          </Tooltip>
        )}
      </Space.Compact>

      <Modal
        open={cropOpen}
        title={tc("cropImage", "Crop Image")}
        okText={t("common:confirm", "Confirm")}
        cancelText={t("common:cancel", "Cancel")}
        onOk={handleCropOk}
        onCancel={closeCrop}
        afterOpenChange={(open) => (open ? initCropper() : destroyCropper())}
        confirmLoading={uploading}
        width={760}
        destroyOnHidden
      >
        <Space wrap style={{ marginBottom: 12 }}>
          <Tooltip title={tc("rotateLeft", "Rotate left")}>
            <Button icon={<RotateLeftOutlined />} onClick={() => rotateBy(-90)} />
          </Tooltip>
          <Tooltip title={tc("rotateRight", "Rotate right")}>
            <Button icon={<RotateRightOutlined />} onClick={() => rotateBy(90)} />
          </Tooltip>
          <Tooltip title={tc("zoomIn", "Zoom in")}>
            <Button icon={<ZoomInOutlined />} onClick={() => zoomBy(0.1)} />
          </Tooltip>
          <Tooltip title={tc("zoomOut", "Zoom out")}>
            <Button icon={<ZoomOutOutlined />} onClick={() => zoomBy(-0.1)} />
          </Tooltip>
          <Tooltip title={tc("reset", "Reset")}>
            <Button icon={<RedoOutlined />} onClick={resetCrop} />
          </Tooltip>
          <Button
            icon={<ColumnWidthOutlined />}
            type={freeAspect ? "default" : "primary"}
            onClick={toggleAspect}
          >
            {freeAspect ? tc("aspectFree", "Free aspect") : tc("aspectLocked", "Fixed aspect")}
          </Button>
        </Space>
        <div style={{ height: 420 }}>
          <img
            ref={imgRef}
            src={cropSrc}
            alt={tc("cropImage", "Crop Image")}
            onLoad={initCropper}
            style={{ display: "block", maxWidth: "100%" }}
          />
        </div>
        <div style={{ marginTop: 8, fontSize: 12, color: "var(--ant-color-text-tertiary)" }}>
          {tc(
            "cropHint",
            "Drag the handles to resize the crop area, scroll to zoom, and use the buttons to rotate.",
          )}
        </div>
      </Modal>

      <Modal
        open={previewOpen}
        title={label || t("common:preview", "Preview")}
        footer={null}
        onCancel={() => setPreviewOpen(false)}
      >
        <img
          src={previewImage}
          alt="Preview"
          style={{ width: "100%", borderRadius: shape === "circle" ? "50%" : 8 }}
        />
      </Modal>
    </div>
  );
}
