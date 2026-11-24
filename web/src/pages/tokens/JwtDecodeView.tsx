import { useState } from "react";
import { App, Button, Card, Input, Space, Typography } from "antd";
import { CopyOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";

type DecodedPart = {
  header: Record<string, unknown>;
  payload: Record<string, unknown>;
  signature: string;
};

function decodeJwt(token: string): DecodedPart | null {
  const parts = token.trim().split(".");
  if (parts.length !== 3) return null;

  try {
    const header = JSON.parse(atob(parts[0].replace(/-/g, "+").replace(/_/g, "/")));
    const payload = JSON.parse(atob(parts[1].replace(/-/g, "+").replace(/_/g, "/")));
    return { header, payload, signature: parts[2] };
  } catch {
    return null;
  }
}

function formatTimestamp(value: unknown): string | null {
  if (typeof value !== "number") return null;
  // Heuristic: if the number is > 1e12 it's likely milliseconds, otherwise seconds
  const ms = value > 1e12 ? value : value * 1000;
  const date = new Date(ms);
  if (Number.isNaN(date.getTime())) return null;
  return date.toLocaleString();
}

function JsonBlock({
  label,
  data,
  color,
}: {
  label: string;
  data: Record<string, unknown>;
  color: string;
}) {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const text = JSON.stringify(data, null, 2);
  const entries = Object.entries(data);

  return (
    <div style={{ marginBottom: "var(--gl-space-3)" }}>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "var(--gl-space-2)",
          marginBottom: "var(--gl-space-1)",
        }}
      >
        <span
          style={{
            display: "inline-block",
            width: 10,
            height: 10,
            borderRadius: "50%",
            backgroundColor: color,
          }}
        />
        <Typography.Text strong>{label}</Typography.Text>
        <Button
          type="text"
          size="small"
          icon={<CopyOutlined />}
          onClick={() => {
            void navigator.clipboard.writeText(text).then(
              () => message.success(tc("copySuccess")),
              () => message.error(tc("copyFailed")),
            );
          }}
        >
          {t("token.jwt.copy")}
        </Button>
      </div>
      <div
        style={{
          backgroundColor: "var(--gl-color-bg)",
          borderRadius: "var(--gl-radius-sm)",
          padding: "var(--gl-space-3)",
          fontFamily: "var(--gl-font-mono)",
          fontSize: "var(--gl-font-size-xs)",
          lineHeight: 1.6,
          whiteSpace: "pre-wrap",
          wordBreak: "break-all",
        }}
      >
        {entries.map(([key, value]) => (
          <div key={key}>
            <span style={{ color: "var(--gl-color-text-secondary)" }}>{key}</span>
            {": "}
            <span style={{ color: "var(--gl-color-text)" }}>
              {typeof value === "string" ? `"${value}"` : String(value)}
            </span>
            {formatTimestamp(value) && (
              <span
                style={{
                  color: "var(--gl-color-text-tertiary)",
                  marginLeft: "var(--gl-space-2)",
                }}
              >
                {/* timestamp annotation */}
                {"// "}
                {formatTimestamp(value)}
              </span>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

export default function JwtDecodeView() {
  const { t } = useTranslation("resource");
  const [tokenInput, setTokenInput] = useState("");
  const [decoded, setDecoded] = useState<DecodedPart | null>(null);
  const [error, setError] = useState("");

  const handleDecode = () => {
    const result = decodeJwt(tokenInput);
    if (result) {
      setDecoded(result);
      setError("");
    } else {
      setDecoded(null);
      setError(t("token.jwt.invalidFormat"));
    }
  };

  const handleClear = () => {
    setTokenInput("");
    setDecoded(null);
    setError("");
  };

  return (
    <Card
      className="gl-panel gl-resource-card"
      variant="borderless"
      style={{ marginTop: "var(--gl-space-4)" }}
      title={t("token.jwt.title")}
    >
      <Typography.Paragraph type="secondary">{t("token.jwt.description")}</Typography.Paragraph>

      <Input.TextArea
        rows={4}
        value={tokenInput}
        onChange={(e) => setTokenInput(e.target.value)}
        placeholder="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
        style={{
          fontFamily: "var(--gl-font-mono)",
          fontSize: "var(--gl-font-size-xs)",
        }}
      />

      <Space style={{ marginTop: "var(--gl-space-2)" }}>
        <Button type="primary" onClick={handleDecode} disabled={!tokenInput.trim()}>
          {t("token.jwt.decode")}
        </Button>
        <Button onClick={handleClear}>{t("token.jwt.clear")}</Button>
      </Space>

      {error && (
        <Typography.Text type="danger" style={{ display: "block", marginTop: "var(--gl-space-2)" }}>
          {error}
        </Typography.Text>
      )}

      {decoded && (
        <div style={{ marginTop: "var(--gl-space-4)" }}>
          <JsonBlock label="Header" data={decoded.header} color="var(--gl-color-error)" />
          <JsonBlock label="Payload" data={decoded.payload} color="var(--gl-color-primary)" />
          <div>
            <Typography.Text strong>Signature</Typography.Text>
            <div
              style={{
                backgroundColor: "var(--gl-color-bg)",
                borderRadius: "var(--gl-radius-sm)",
                padding: "var(--gl-space-3)",
                fontFamily: "var(--gl-font-mono)",
                fontSize: "var(--gl-font-size-xs)",
                wordBreak: "break-all",
                color: "var(--gl-color-success)",
              }}
            >
              {decoded.signature}
            </div>
          </div>
        </div>
      )}
    </Card>
  );
}
