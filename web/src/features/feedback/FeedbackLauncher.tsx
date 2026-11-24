import { useState } from "react";
import { App, Button, Tooltip } from "antd";
import { CommentOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { captureViewport, setPendingCapture } from "./capture";
import "./feedback.css";

/**
 * Persistent bottom-right feedback launcher, mounted once inside the
 * authenticated app shell (both top-nav and sidebar layouts share it).
 *
 * Clicking it captures the current viewport, hands the screenshot to the ticket
 * form via the capture singleton, and opens the vendor-feedback submission. The
 * capture is best-effort: if rasterisation fails the form still opens so the
 * user can describe the issue and attach files manually.
 */
export function FeedbackLauncher() {
  const { t } = useTranslation("common");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [capturing, setCapturing] = useState(false);

  const openFeedback = async () => {
    if (capturing) return;
    setCapturing(true);
    try {
      setPendingCapture(await captureViewport());
    } catch {
      message.warning(t("feedback.captureFailed"));
    } finally {
      setCapturing(false);
    }
    navigate("/tickets/new?type=vendor_feedback");
  };

  return (
    <div className="gl-feedback-launcher" data-feedback-ui="true">
      <Tooltip title={t("feedback.tooltip")} placement="left">
        <Button
          type="primary"
          size="large"
          className="gl-feedback-launcher__btn"
          icon={<CommentOutlined />}
          loading={capturing}
          onClick={openFeedback}
          aria-label={t("feedback.launch")}
        >
          {t("feedback.launch")}
        </Button>
      </Tooltip>
    </div>
  );
}
