import { Col, Form, Input, Row, Select } from "antd";
import type { FormInstance } from "antd";
import { useTranslation } from "react-i18next";
import RedirectUriTable from "@/shared/ui/RedirectUriTable";
import type { RedirectUri } from "@/shared/ui/RedirectUriTable";
import { FormSection } from "@/shared/ui/FormSection";
import type { ApplicationConfigFormValues } from "./types";

/**
 * Mirrors the write-side rules in internal/app/applications/validation.go
 * (validateApplicationReverseProxyWrite). Empty values always pass; non-empty
 * values keep the shape the server accepts. The browser checks stay at or below
 * the server's strictness so a value the API accepts is never blocked here.
 *
 * Cross-field rules (R1/R2/R3/R4) mirror the server's combination checks, but are
 * warning-only: the server decides whether a rule applies by comparing the request
 * with the stored row, and the browser cannot know that. Blocking here would make
 * applications whose stored combination predates the rules unsaveable, while the
 * API would still accept them.
 */
const MAX_REVERSE_PROXY_TEXT_LENGTH = 255;

/** Target sslMode values, kept in sync with the server's allowedSslModes. */
const SSL_MODE_OPTIONS = ["", "HTTP", "HTTPS and HTTP", "HTTPS Only"] as const;
const TLS_SSL_MODES = new Set(["HTTPS Only", "HTTPS and HTTP"]);
/** Historical Casdoor wording mapped to its target value (read-only in the UI). */
const LEGACY_SSL_MODE_ALIASES: Record<string, string> = {
  Auto: "",
  Enable: "HTTPS Only",
  Disable: "HTTP",
};

const upstreamHostNamePattern = /^[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?$/;
const upstreamHostPortPattern = /^[0-9]{1,5}$/;
const sslCertIDPattern = /^[A-Za-z0-9._:/-]+$/;
const ipv4Pattern =
  /^(?:(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)\.){3}(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)$/;
// Absolute http(s) URL with a non-empty authority. Deliberately looser than
// net/url (e.g. it tolerates an out-of-range port) — the server still rejects
// those, but this must not reject anything the server accepts.
const absoluteHttpUrlPattern = /^https?:\/\/[^\s/?#]+/i;

/** Count code points, matching the server's utf8.RuneCountInString. */
function characterLength(value: string): number {
  return [...value].length;
}

function isValidUpstreamPort(value: string): boolean {
  if (!upstreamHostPortPattern.test(value)) return false;
  const port = Number(value);
  return port >= 1 && port <= 65535;
}

/**
 * net.ParseIP has no direct browser equivalent. Accepting hex digits, dots and
 * at least two colons covers every literal the server accepts; it may let a few
 * malformed values through, but never the other way round.
 */
function isIPv6Literal(value: string): boolean {
  if (!value.includes(":") || !/^[0-9A-Fa-f:.]+$/.test(value)) return false;
  return (value.match(/:/g)?.length ?? 0) >= 2;
}

function isIPLiteral(value: string): boolean {
  return ipv4Pattern.test(value) || isIPv6Literal(value);
}

function isValidUpstreamHost(value: string): boolean {
  if (value.includes("://")) return false;
  if (value.startsWith("[")) {
    const end = value.lastIndexOf("]");
    if (end < 0 || !isIPLiteral(value.slice(1, end))) return false;
    const rest = value.slice(end + 1);
    return rest === "" || (rest.startsWith(":") && isValidUpstreamPort(rest.slice(1)));
  }
  if (isIPLiteral(value)) return true;
  const separator = value.lastIndexOf(":");
  const host = separator >= 0 ? value.slice(0, separator) : value;
  if (separator >= 0 && !isValidUpstreamPort(value.slice(separator + 1))) return false;
  return upstreamHostNamePattern.test(host);
}

function upstreamHostErrorKey(value: string | undefined): string | undefined {
  const trimmed = value?.trim() ?? "";
  if (trimmed === "") return undefined;
  if (characterLength(trimmed) > MAX_REVERSE_PROXY_TEXT_LENGTH) {
    return "application.validation.upstreamHostTooLong";
  }
  return isValidUpstreamHost(trimmed) ? undefined : "application.validation.upstreamHostInvalid";
}

function sslCertErrorKey(value: string | undefined): string | undefined {
  const trimmed = value?.trim() ?? "";
  if (trimmed === "") return undefined;
  if (characterLength(trimmed) > MAX_REVERSE_PROXY_TEXT_LENGTH) {
    return "application.validation.sslCertTooLong";
  }
  return sslCertIDPattern.test(trimmed) ? undefined : "application.validation.sslCertInvalid";
}

/** Returns the first offending row (1-based) so the message names it. */
function otherDomainsIssue(value: unknown): { key: string; row: number } | undefined {
  if (!Array.isArray(value)) return undefined;
  for (let index = 0; index < value.length; index += 1) {
    const entry = value[index];
    const trimmed = typeof entry === "string" ? entry.trim() : "";
    if (trimmed === "") continue;
    if (characterLength(trimmed) > MAX_REVERSE_PROXY_TEXT_LENGTH) {
      return { key: "application.validation.otherDomainTooLong", row: index + 1 };
    }
    if (!absoluteHttpUrlPattern.test(trimmed)) {
      return { key: "application.validation.otherDomainInvalid", row: index + 1 };
    }
  }
  return undefined;
}

/** Legacy value → target value; undefined means "not a known sslMode". */
function canonicalSslMode(value: string | undefined): string | undefined {
  const trimmed = value?.trim() ?? "";
  if ((SSL_MODE_OPTIONS as readonly string[]).includes(trimmed)) return trimmed;
  return LEGACY_SSL_MODE_ALIASES[trimmed];
}

/**
 * Mirrors the server's NormalizeReverseProxyHost: lower-case host[:port] with
 * scheme, path/query/fragment and the trailing dot removed; the port is kept.
 */
function normalizeHost(raw: string | undefined): string {
  const trimmed = raw?.trim() ?? "";
  if (trimmed === "") return "";
  let candidate = trimmed;
  const schemeIndex = candidate.indexOf("://");
  if (schemeIndex >= 0) candidate = candidate.slice(schemeIndex + 3);
  const boundary = candidate.search(/[/?#]/);
  if (boundary >= 0) candidate = candidate.slice(0, boundary);
  candidate = candidate.toLowerCase();
  const separator = candidate.lastIndexOf(":");
  if (
    separator >= 0 &&
    !candidate.slice(separator + 1).includes(":") &&
    !candidate.startsWith("[")
  ) {
    const host = candidate.slice(0, separator).replace(/\.$/, "");
    return host + candidate.slice(separator);
  }
  return candidate.replace(/\.$/, "");
}

/** Drops the port; a self-loop stays a self-loop on any port. */
function hostOnly(normalized: string): string {
  if (normalized === "") return "";
  if (normalized.startsWith("[")) {
    const end = normalized.lastIndexOf("]");
    return end > 0 ? normalized.slice(0, end + 1) : normalized;
  }
  if ((normalized.match(/:/g)?.length ?? 0) > 1) return normalized;
  const separator = normalized.lastIndexOf(":");
  return separator >= 0 ? normalized.slice(0, separator) : normalized;
}

/** R1: HTTPS termination declared without a certificate. */
function sslCertRequiredKey(
  mode: string | undefined,
  cert: string | undefined,
): string | undefined {
  const canonical = canonicalSslMode(mode);
  if (canonical === undefined || !TLS_SSL_MODES.has(canonical)) return undefined;
  return (cert?.trim() ?? "") === "" ? "application.validation.sslCertRequiredForHttps" : undefined;
}

/** R1: certificate set on a mode that does not terminate TLS. */
function sslCertUnusedKey(mode: string | undefined, cert: string | undefined): string | undefined {
  const canonical = canonicalSslMode(mode);
  if (canonical === undefined || TLS_SSL_MODES.has(canonical)) return undefined;
  return (cert?.trim() ?? "") !== ""
    ? "application.validation.sslCertRequiresHttpsMode"
    : undefined;
}

/** R2 + R4: upstream required when proxying, and it must not point at itself. */
function upstreamHostCombinationKey(values: {
  sslMode?: string;
  sslCert?: string;
  upstreamHost?: string;
  domain?: string;
  otherDomains?: string[];
}): string | undefined {
  const upstreamHost = values.upstreamHost?.trim() ?? "";
  const mode = values.sslMode?.trim() ?? "";
  const cert = values.sslCert?.trim() ?? "";
  if (upstreamHost === "") {
    return mode !== "" || cert !== ""
      ? "application.validation.upstreamHostRequiredForProxy"
      : undefined;
  }
  const ownHosts = new Set<string>();
  const domainHost = hostOnly(normalizeHost(values.domain));
  if (domainHost !== "") ownHosts.add(domainHost);
  for (const entry of values.otherDomains ?? []) {
    const host = hostOnly(normalizeHost(entry));
    if (host !== "") ownHosts.add(host);
  }
  return ownHosts.has(hostOnly(normalizeHost(upstreamHost)))
    ? "application.validation.upstreamHostSelfLoop"
    : undefined;
}

/** R3: otherDomains must not repeat the primary domain or another row's host. */
function otherDomainsHostIssue(
  domain: string | undefined,
  value: unknown,
): { key: string; row: number } | undefined {
  if (!Array.isArray(value)) return undefined;
  const primaryHost = normalizeHost(domain);
  const seen = new Set<string>();
  for (let index = 0; index < value.length; index += 1) {
    const entry = value[index];
    const host = normalizeHost(typeof entry === "string" ? entry : "");
    if (host === "") continue;
    if (primaryHost !== "" && host === primaryHost) {
      return { key: "application.validation.otherDomainEqualsDomain", row: index + 1 };
    }
    if (seen.has(host)) {
      return { key: "application.validation.otherDomainDuplicate", row: index + 1 };
    }
    seen.add(host);
  }
  return undefined;
}

/** Bridge: converts string[] ↔ RedirectUri[] for otherDomains Form.Item integration */
function OtherDomainsTableInput({
  value,
  onChange,
}: {
  value?: string[];
  onChange?: (v: string[]) => void;
}) {
  const items: RedirectUri[] = (value ?? []).map((uri) => ({ uri }));
  return <RedirectUriTable value={items} onChange={(v) => onChange?.(v.map((r) => r.uri))} />;
}

type ReverseProxyTabProps = {
  form: FormInstance<ApplicationConfigFormValues>;
};

export function ReverseProxyTab({ form }: ReverseProxyTabProps) {
  const { t: tr } = useTranslation("resource");
  const sslMode = Form.useWatch("sslMode", form);
  const legacySslMode =
    typeof sslMode === "string" &&
    sslMode !== "" &&
    !(SSL_MODE_OPTIONS as readonly string[]).includes(sslMode)
      ? sslMode
      : undefined;

  return (
    <>
      <FormSection
        title={tr("application.reverseProxyConfigSection", "Reverse Proxy Configuration")}
      >
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item
              label={tr("application.sslMode", "Protocol")}
              name="sslMode"
              dependencies={["sslCert"]}
              rules={[
                {
                  warningOnly: true,
                  validator: (_, value: string | undefined) => {
                    const key = sslCertUnusedKey(value, form.getFieldValue("sslCert"));
                    return key ? Promise.reject(new Error(tr(key))) : Promise.resolve();
                  },
                },
              ]}
            >
              <Select
                placeholder={tr("application.placeholder.selectProtocol", "Select protocol")}
                options={[
                  ...(legacySslMode
                    ? [
                        {
                          label: tr("application.sslModeOption.legacy", {
                            value: legacySslMode,
                            defaultValue: "{{value}} (legacy)",
                          }),
                          value: legacySslMode,
                        },
                      ]
                    : []),
                  {
                    label: tr("application.sslModeOption.none", "None (not declared)"),
                    value: "",
                  },
                  { label: tr("application.sslModeOption.http", "HTTP"), value: "HTTP" },
                  {
                    label: tr("application.sslModeOption.httpsAndHttp", "HTTPS and HTTP"),
                    value: "HTTPS and HTTP",
                  },
                  {
                    label: tr("application.sslModeOption.httpsOnly", "HTTPS only"),
                    value: "HTTPS Only",
                  },
                ]}
              />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              label={tr("application.upstreamHost", "Upstream Host")}
              name="upstreamHost"
              dependencies={["sslMode", "sslCert", "domain", "otherDomains"]}
              rules={[
                {
                  validator: (_, value: string | undefined) => {
                    const key = upstreamHostErrorKey(value);
                    return key ? Promise.reject(new Error(tr(key))) : Promise.resolve();
                  },
                },
                {
                  warningOnly: true,
                  validator: (_, value: string | undefined) => {
                    const key = upstreamHostCombinationKey({
                      sslMode: form.getFieldValue("sslMode"),
                      sslCert: form.getFieldValue("sslCert"),
                      upstreamHost: value,
                      domain: form.getFieldValue("domain"),
                      otherDomains: form.getFieldValue("otherDomains"),
                    });
                    return key ? Promise.reject(new Error(tr(key))) : Promise.resolve();
                  },
                },
              ]}
            >
              <Input autoComplete="off" spellCheck={false} placeholder="internal-server:8080" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              label={tr("application.sslCert", "SSL Certificate")}
              name="sslCert"
              dependencies={["sslMode"]}
              rules={[
                {
                  validator: (_, value: string | undefined) => {
                    const key = sslCertErrorKey(value);
                    return key ? Promise.reject(new Error(tr(key))) : Promise.resolve();
                  },
                },
                {
                  warningOnly: true,
                  validator: (_, value: string | undefined) => {
                    const key = sslCertRequiredKey(form.getFieldValue("sslMode"), value);
                    return key ? Promise.reject(new Error(tr(key))) : Promise.resolve();
                  },
                },
              ]}
            >
              <Input
                autoComplete="off"
                spellCheck={false}
                placeholder={tr(
                  "application.placeholder.sslCert",
                  "Certificate ID for SSL termination",
                )}
              />
            </Form.Item>
          </Col>
        </Row>
      </FormSection>

      <FormSection title={tr("application.domainConfigSection", "Domain Configuration")}>
        <Form.Item
          label={tr("application.otherDomains", "Other Domains")}
          name="otherDomains"
          dependencies={["domain"]}
          rules={[
            {
              validator: (_, value: string[] | undefined) => {
                const issue = otherDomainsIssue(value);
                return issue
                  ? Promise.reject(new Error(tr(issue.key, { row: issue.row })))
                  : Promise.resolve();
              },
            },
            {
              warningOnly: true,
              validator: (_, value: string[] | undefined) => {
                const issue = otherDomainsHostIssue(form.getFieldValue("domain"), value);
                return issue
                  ? Promise.reject(new Error(tr(issue.key, { row: issue.row })))
                  : Promise.resolve();
              },
            },
          ]}
        >
          <OtherDomainsTableInput />
        </Form.Item>
      </FormSection>
    </>
  );
}
