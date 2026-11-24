import { useMemo } from "react";
import { Alert, Descriptions, Typography } from "antd";

type ParsedMetadata = {
  entityId: string;
  ssoUrls: { binding: string; location: string }[];
  sloUrls: { binding: string; location: string }[];
  certificates: string[];
  errors: string[];
};

function parseSamlMetadata(xml: string): ParsedMetadata {
  const result: ParsedMetadata = {
    entityId: "",
    ssoUrls: [],
    sloUrls: [],
    certificates: [],
    errors: [],
  };

  try {
    const parser = new DOMParser();
    const doc = parser.parseFromString(xml, "application/xml");

    const parseError = doc.querySelector("parsererror");
    if (parseError) {
      result.errors.push(
        "Invalid XML: " + (parseError.textContent?.slice(0, 200) ?? "parse error"),
      );
      return result;
    }

    // Entity ID
    const entityDescriptor = doc.querySelector("EntityDescriptor, md\\:EntityDescriptor");
    result.entityId = entityDescriptor?.getAttribute("entityID") ?? "";

    // SSO URLs
    const ssoServices = doc.querySelectorAll("SingleSignOnService, md\\:SingleSignOnService");
    ssoServices.forEach((el) => {
      const binding = el.getAttribute("Binding") ?? "";
      const location = el.getAttribute("Location") ?? "";
      if (location) {
        result.ssoUrls.push({ binding: binding.split("/").pop() ?? binding, location });
      }
    });

    // SLO URLs
    const sloServices = doc.querySelectorAll("SingleLogoutService, md\\:SingleLogoutService");
    sloServices.forEach((el) => {
      const binding = el.getAttribute("Binding") ?? "";
      const location = el.getAttribute("Location") ?? "";
      if (location) {
        result.sloUrls.push({ binding: binding.split("/").pop() ?? binding, location });
      }
    });

    // X509 Certificates
    const certs = doc.querySelectorAll("X509Certificate, ds\\:X509Certificate");
    certs.forEach((el) => {
      const certText = el.textContent?.trim() ?? "";
      if (certText) {
        // Show first 40 chars + "..." for display
        result.certificates.push(certText.length > 60 ? certText.slice(0, 60) + "..." : certText);
      }
    });

    if (!result.entityId && result.ssoUrls.length === 0 && result.sloUrls.length === 0) {
      result.errors.push("No SAML entity ID or endpoints found in the metadata");
    }
  } catch (e) {
    result.errors.push(e instanceof Error ? e.message : "Unknown parse error");
  }

  return result;
}

type SamlMetadataViewerProps = {
  xml: string;
};

export function SamlMetadataViewer({ xml }: SamlMetadataViewerProps) {
  const parsed = useMemo(() => (xml.trim() ? parseSamlMetadata(xml) : null), [xml]);

  if (!parsed) return null;

  if (parsed.errors.length > 0) {
    return (
      <Alert
        type="warning"
        showIcon
        style={{ marginTop: 8 }}
        title="SAML Metadata Parse Issues"
        description={
          <ul style={{ margin: 0, paddingLeft: 20 }}>
            {parsed.errors.map((err, i) => (
              <li key={i}>{err}</li>
            ))}
          </ul>
        }
      />
    );
  }

  const hasData = parsed.entityId || parsed.ssoUrls.length > 0 || parsed.sloUrls.length > 0;

  if (!hasData) return null;

  return (
    <div style={{ marginTop: 12 }}>
      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        Parsed from metadata XML:
      </Typography.Text>
      <Descriptions
        bordered
        size="small"
        column={1}
        style={{ marginTop: 4 }}
        labelStyle={{ width: 140, fontWeight: 500 }}
      >
        {parsed.entityId && (
          <Descriptions.Item label="Entity ID">
            <Typography.Text copyable code>
              {parsed.entityId}
            </Typography.Text>
          </Descriptions.Item>
        )}
        {parsed.ssoUrls.map((sso, i) => (
          <Descriptions.Item
            key={i}
            label={`SSO URL${parsed.ssoUrls.length > 1 ? ` (${sso.binding})` : ""}`}
          >
            <Typography.Text copyable>{sso.location}</Typography.Text>
          </Descriptions.Item>
        ))}
        {parsed.sloUrls.map((slo, i) => (
          <Descriptions.Item
            key={i}
            label={`SLO URL${parsed.sloUrls.length > 1 ? ` (${slo.binding})` : ""}`}
          >
            <Typography.Text copyable>{slo.location}</Typography.Text>
          </Descriptions.Item>
        ))}
        {parsed.certificates.map((cert, i) => (
          <Descriptions.Item
            key={i}
            label={`Certificate${parsed.certificates.length > 1 ? ` ${i + 1}` : ""}`}
          >
            <Typography.Text code>{cert}</Typography.Text>
          </Descriptions.Item>
        ))}
      </Descriptions>
    </div>
  );
}
