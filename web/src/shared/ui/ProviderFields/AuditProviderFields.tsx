import { Form, Input, InputNumber, Select } from "antd";

const SYSLOG_FACILITIES = [
  "auth",
  "authpriv",
  "daemon",
  "kern",
  "local0",
  "local1",
  "local2",
  "local3",
  "local4",
  "local5",
  "local6",
  "local7",
  "security",
  "user",
].map((value) => ({ label: value, value }));

/**
 * Audit provider fields.
 *
 * The column mapping is deliberate: Host and Port carry the syslog address,
 * SubType carries UDP/TCP/TLS, Template carries the facility and ContentType
 * carries the RFC format. These are the values the outbox observer reads when
 * it builds the sink, so each control has exactly one destination column.
 */
export function AuditProviderFields() {
  return (
    <>
      <Form.Item
        label="Syslog Host"
        name="host"
        rules={[{ required: true, message: "Syslog host is required" }]}
      >
        <Input placeholder="syslog.example.com" />
      </Form.Item>
      <Form.Item
        label="Syslog Port"
        name="port"
        rules={[{ required: true, message: "Syslog port is required" }]}
      >
        <InputNumber min={1} max={65535} style={{ width: "100%" }} placeholder="514" />
      </Form.Item>
      <Form.Item label="Protocol" name="subType" initialValue="UDP">
        <Select
          options={[
            { label: "UDP", value: "UDP" },
            { label: "TCP", value: "TCP" },
            { label: "TLS", value: "TLS" },
          ]}
        />
      </Form.Item>
      <Form.Item label="Facility" name="template" initialValue="auth">
        <Select options={SYSLOG_FACILITIES} />
      </Form.Item>
      <Form.Item label="Format" name="contentType" initialValue="RFC 5424">
        <Select
          options={[
            { label: "RFC 5424", value: "RFC 5424" },
            { label: "RFC 3164", value: "RFC 3164" },
          ]}
        />
      </Form.Item>
    </>
  );
}
