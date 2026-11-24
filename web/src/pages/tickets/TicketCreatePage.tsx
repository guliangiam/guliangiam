import { useEffect, useRef, useState } from "react";
import {
  App,
  Button,
  Form,
  Input,
  Modal,
  Radio,
  Select,
  Upload,
  type UploadFile,
  type UploadProps,
} from "antd";
import { CameraOutlined, FilePdfOutlined, PictureOutlined } from "@ant-design/icons";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamTicket } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { requestBlob } from "@/shared/api/http";
import { UPLOAD_RESOURCE } from "@/shared/api/paths";
import { useSessionStore } from "@/features/session/session-store";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { normalizeSlug } from "@/pages/applications/application-helpers";
import { localizedError } from "@/shared/lib/server-error";
import { captureViewport, consumePendingCapture } from "@/features/feedback/capture";

type TicketFormValues = {
  organizationId?: string;
  name: string;
  displayName: string;
  type: string;
  priority?: string;
  assignee?: string;
  creator?: string;
  description?: string;
  attachment?: string;
};

/**
 * Attachment limits mirror the upload contract (手册 4.17.2): the backend only
 * accepts raster images (jpg/jpeg/png/gif/webp) and PDF, each extension bound to
 * a fixed MIME type. Anything else is refused server-side, so the picker offers
 * exactly that set rather than promising formats the API would reject.
 */
const MAX_ATTACHMENTS = 5;
const MAX_ATTACHMENT_MB = 5;
const ACCEPTED_EXTENSIONS = [".jpg", ".jpeg", ".png", ".gif", ".webp", ".pdf"];
const ACCEPT_ATTR = "image/jpeg,image/png,image/gif,image/webp,application/pdf";
const NAME_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

type FeedbackSubtype = "bug" | "feature";

function hasAcceptedExtension(name: string): boolean {
  const lower = name.toLowerCase();
  return ACCEPTED_EXTENSIONS.some((ext) => lower.endsWith(ext));
}

function isImageName(name: string): boolean {
  return /\.(jpe?g|png|gif|webp)$/i.test(name);
}

function renderAttachmentIcon(file: UploadFile) {
  return isImageName(file.name) ? <PictureOutlined /> : <FilePdfOutlined />;
}

export default function TicketCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [searchParams] = useSearchParams();
  const [form] = Form.useForm<TicketFormValues>();

  const account = useSessionStore((state) => state.account);
  const sessionOrganization = useSessionStore((state) => state.organization);
  // 平台管理员判定与会话组织一致：built-in 组织的成员可代其他组织提交工单。
  const isPlatformAdmin = account?.owner === "built-in";

  // 反馈悬浮按钮带 ?type=vendor_feedback 进入时直接预选厂商反馈。
  const initialType =
    searchParams.get("type") === "vendor_feedback" ? "vendor_feedback" : "org_request";

  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [subtype, setSubtype] = useState<FeedbackSubtype | undefined>(undefined);
  const [capturing, setCapturing] = useState(false);
  // 记录最近一次自动注入的模板；用户改动过描述就不再覆盖。
  const lastTemplateRef = useRef("");

  const watchedType = Form.useWatch("type", form);
  const watchedDisplayName = Form.useWatch("displayName", form);
  const watchedName = Form.useWatch("name", form);
  const lastAutoNameRef = useRef("");

  // name 由 displayName slug 化自动生成（与 applications 创建页同一联动模式），保留手改。
  useEffect(() => {
    const next = normalizeSlug(watchedDisplayName);
    const current = (watchedName ?? "").trim();
    const shouldAutofill = current === "" || current === lastAutoNameRef.current;
    if (!shouldAutofill || next === "" || next === current) return;
    lastAutoNameRef.current = next;
    form.setFieldValue("name", next);
  }, [form, watchedDisplayName, watchedName]);

  // 离开 vendor_feedback 时，未被用户编辑过的模板要撤掉，已输入的内容保留。
  const previousTypeRef = useRef<string | undefined>(undefined);
  useEffect(() => {
    const previous = previousTypeRef.current;
    previousTypeRef.current = watchedType;
    if (previous === undefined || previous === watchedType) return;
    if (previous === "vendor_feedback" && watchedType !== "vendor_feedback") {
      const current = (form.getFieldValue("description") as string | undefined) ?? "";
      if (current === lastTemplateRef.current) {
        form.setFieldValue("description", "");
        lastTemplateRef.current = "";
      }
      setSubtype(undefined);
    }
  }, [form, watchedType]);

  // 附件 URL 列表始终由 fileList 派生，避免上传/截图/删除各路径手动同步漏项。
  useEffect(() => {
    form.setFieldValue(
      "attachment",
      fileList
        .filter((file) => file.url)
        .map((file) => file.url)
        .join(","),
    );
  }, [fileList, form]);

  const uploadAttachment = async (file: File): Promise<string> => {
    const formData = new FormData();
    formData.append("file", file);
    const resp = await requestBlob(
      `${UPLOAD_RESOURCE}?owner=${encodeURIComponent(account?.owner ?? "")}&user=${encodeURIComponent(account?.name ?? "")}&tag=ticket`,
      { method: "POST", body: formData },
    );
    const result = await resp.json();
    const url =
      typeof result === "string"
        ? result
        : (((result as Record<string, unknown>)?.data as string) ?? "");
    if (!url) throw new Error("upload response has no url");
    return url;
  };

  // 把一张截图（或任意附件 File）走与 Upload 相同的上传管线，落到 fileList。
  const addAttachmentFile = async (file: File) => {
    const uid = `capture-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const pending: UploadFile = {
      uid,
      name: file.name,
      size: file.size,
      type: file.type,
      status: "uploading",
    };
    setFileList((prev) => [...prev, pending]);
    try {
      const url = await uploadAttachment(file);
      setFileList((prev) =>
        prev.map((f) => (f.uid === uid ? { ...f, status: "done", url, response: { url } } : f)),
      );
      message.success(tc("uploadSuccess"));
    } catch {
      setFileList((prev) => prev.map((f) => (f.uid === uid ? { ...f, status: "error" } : f)));
      message.error(tc("uploadFailed"));
    }
  };

  // 消费悬浮按钮在跳转前捕获的当前屏幕截图（一次性）。
  const captureConsumedRef = useRef(false);
  useEffect(() => {
    if (captureConsumedRef.current) return;
    const file = consumePendingCapture();
    if (!file) return;
    captureConsumedRef.current = true;
    void addAttachmentFile(file);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleCapture = async () => {
    if (capturing) return;
    if (fileList.length >= MAX_ATTACHMENTS) {
      message.error(tr("ticket.tooManyFiles", { max: MAX_ATTACHMENTS }));
      return;
    }
    setCapturing(true);
    try {
      const file = await captureViewport();
      await addAttachmentFile(file);
    } catch {
      message.warning(tc("feedback.captureFailed"));
    } finally {
      setCapturing(false);
    }
  };

  const handleSubtypeChange = (next?: FeedbackSubtype) => {
    setSubtype(next);
    const template =
      next === "bug"
        ? tr("ticket.templateBug")
        : next === "feature"
          ? tr("ticket.templateFeature")
          : "";
    const current = (form.getFieldValue("description") as string | undefined) ?? "";
    if (current === "" || current === lastTemplateRef.current) {
      form.setFieldValue("description", template);
      lastTemplateRef.current = template;
    }
  };

  const uploadProps: UploadProps = {
    multiple: true,
    accept: ACCEPT_ATTR,
    listType: "picture-card",
    fileList,
    iconRender: renderAttachmentIcon,
    beforeUpload: (file) => {
      if (!hasAcceptedExtension(file.name)) {
        message.error(tr("ticket.fileTypeInvalid"));
        return Upload.LIST_IGNORE;
      }
      if (file.size > MAX_ATTACHMENT_MB * 1024 * 1024) {
        message.error(tr("ticket.fileTooLarge", { max: MAX_ATTACHMENT_MB }));
        return Upload.LIST_IGNORE;
      }
      if (fileList.length >= MAX_ATTACHMENTS) {
        message.error(tr("ticket.tooManyFiles", { max: MAX_ATTACHMENTS }));
        return Upload.LIST_IGNORE;
      }
      return true;
    },
    customRequest: async ({ file, onSuccess: onUploaded, onError: onFailed }) => {
      try {
        const url = await uploadAttachment(file as File);
        onUploaded?.({ url });
        message.success(tc("uploadSuccess"));
      } catch (error) {
        message.error(tc("uploadFailed"));
        onFailed?.(error as Error);
      }
    },
    onChange: (info) => {
      const merged = info.fileList.slice(0, MAX_ATTACHMENTS).map((f) => {
        const response = f.response as { url?: string } | undefined;
        return response?.url ? { ...f, url: response.url } : f;
      });
      setFileList(merged);
    },
    onPreview: (file) => {
      if (!file.url) return;
      if (isImageName(file.name)) {
        setPreviewUrl(file.url);
      } else {
        window.open(file.url, "_blank", "noopener,noreferrer");
      }
    },
  };

  const mutation = useMutation({
    mutationFn: async (values: TicketFormValues) => {
      return createIamTicket({
        // 普通成员不带 organizationId：租户由服务端可信 scope 裁决；平台管理员代提时带所选组织。
        organizationId: isPlatformAdmin ? values.organizationId : undefined,
        name: values.name,
        displayName: values.displayName,
        type: values.type,
        priority: values.priority,
        assignee: values.assignee,
        creator: values.creator,
        description: values.description,
        attachment: values.attachment,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.tickets.all });
      message.success(tr("ticket.created"));
      navigate("/tickets");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "ticket.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("ticket.createTitle")}
      description={tr("ticket.createDesc")}
      saving={mutation.isPending}
      form={form}
      retryable
      onCancel={() => navigate("/tickets")}
      onFinish={async (values: TicketFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item name="type" initialValue={initialType}>
        <GlFormRow label={tr("ticket.target")} required>
          <Radio.Group>
            <Radio value="org_request">
              <div>
                <div>{tr("ticket.targetOrgRequest")}</div>
                <div style={{ fontSize: 12, color: "var(--gl-color-text-tertiary)" }}>
                  {tr("ticket.targetOrgRequestDesc")}
                </div>
              </div>
            </Radio>
            <Radio value="vendor_feedback">
              <div>
                <div>{tr("ticket.targetVendorFeedback")}</div>
                <div style={{ fontSize: 12, color: "var(--gl-color-text-tertiary)" }}>
                  {tr("ticket.targetVendorFeedbackDesc")}
                </div>
              </div>
            </Radio>
          </Radio.Group>
        </GlFormRow>
      </Form.Item>

      {isPlatformAdmin ? (
        <Form.Item
          name="organizationId"
          rules={[{ required: true, message: tc("validation.organizationIdRequired") }]}
        >
          <GlFormRow label={tc("organizationId")} required>
            <OrganizationSelect ariaLabel={tc("organizationId")} style={{ width: "100%" }} />
          </GlFormRow>
        </Form.Item>
      ) : (
        // 普通成员：租户由服务端可信 scope 决定，这里只读展示当前组织名。
        // GlFormRow 消费 Form.Item.useStatus()，必须位于 Form.Item 之内（无 name 的纯展示项）。
        <Form.Item>
          <GlFormRow label={tc("organization")}>
            <Input
              disabled
              value={sessionOrganization?.displayName || sessionOrganization?.name || ""}
            />
          </GlFormRow>
        </Form.Item>
      )}

      <Form.Item
        name="displayName"
        rules={[{ required: true, whitespace: true, message: tr("ticket.displayNameRequired") }]}
      >
        <GlFormRow label={tc("displayName")} required>
          <Input maxLength={128} placeholder={tr("ticket.displayNamePlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="name"
        rules={[
          { required: true, message: tr("ticket.nameRequired") },
          { pattern: NAME_PATTERN, message: tr("ticket.nameInvalid") },
        ]}
      >
        <GlFormRow label={tc("name")} required>
          <Input maxLength={64} placeholder="e.g. ticket-1" />
        </GlFormRow>
      </Form.Item>

      {watchedType === "vendor_feedback" && (
        <Form.Item>
          <GlFormRow label={tr("ticket.subtype")}>
            <Select
              allowClear
              value={subtype}
              onChange={(value) => handleSubtypeChange(value as FeedbackSubtype | undefined)}
              placeholder={tr("ticket.subtypePlaceholder")}
              options={[
                { label: tr("ticket.subtypeBug"), value: "bug" },
                { label: tr("ticket.subtypeFeature"), value: "feature" },
              ]}
            />
          </GlFormRow>
        </Form.Item>
      )}

      <Form.Item name="priority" initialValue="Medium">
        <GlFormRow label={tr("ticket.priority")}>
          <Select placeholder={tr("ticket.priorityPlaceholder")}>
            <Select.Option value="Low">{tr("ticket.priorityLow")}</Select.Option>
            <Select.Option value="Medium">{tr("ticket.priorityMedium")}</Select.Option>
            <Select.Option value="High">{tr("ticket.priorityHigh")}</Select.Option>
            <Select.Option value="Urgent">{tr("ticket.priorityUrgent")}</Select.Option>
          </Select>
        </GlFormRow>
      </Form.Item>
      <Form.Item name="assignee">
        <GlFormRow label={tr("ticket.assignee")}>
          <Input placeholder={tr("ticket.assigneePlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="creator" initialValue={account?.displayName || account?.name || undefined}>
        <GlFormRow label={tr("ticket.creator")}>
          <Input disabled />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="description"
        rules={
          watchedType === "vendor_feedback"
            ? [{ required: true, whitespace: true, message: tr("ticket.descriptionRequired") }]
            : []
        }
      >
        <GlFormRow label={tc("description")} required={watchedType === "vendor_feedback"}>
          <Input.TextArea rows={5} placeholder={tc("description")} />
        </GlFormRow>
      </Form.Item>

      <Form.Item name="attachment" hidden>
        <Input />
      </Form.Item>
      <Form.Item>
        <GlFormRow label={tr("ticket.attachments")} extra={tr("ticket.attachmentsHint")}>
          <div>
            <div style={{ marginBottom: "var(--gl-space-3, 12px)" }}>
              <Button
                icon={<CameraOutlined />}
                loading={capturing}
                onClick={handleCapture}
                data-feedback-ui="true"
              >
                {tr("ticket.captureScreenshot")}
              </Button>
            </div>
            <Upload {...uploadProps}>
              <div>
                <PictureOutlined />
                <div style={{ marginTop: 8 }}>{tr("ticket.upload")}</div>
              </div>
            </Upload>
            <Modal
              open={!!previewUrl}
              footer={null}
              onCancel={() => setPreviewUrl(null)}
              width={640}
            >
              {previewUrl && <img src={previewUrl} alt="" style={{ width: "100%" }} />}
            </Modal>
          </div>
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
