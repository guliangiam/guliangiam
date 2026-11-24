/**
 * 组织基础信息分区 —— 创建页与详情页共用同一套字段实现。
 */
import { Col, Form, Input, InputNumber, Row, Select } from "antd";
import { useTranslation } from "react-i18next";
import { GlFormRow } from "@/shared/ui/GlForm";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import type { DeploymentRegionOption } from "@/shared/api/iam/organization";
import type { OrganizationSectionMode } from "./types";

type OrganizationBasicFieldsProps = {
  mode: OrganizationSectionMode;
  currentOrganizationId?: string;
  regionOptions: DeploymentRegionOption[];
  regionsError?: boolean;
};

export function OrganizationBasicFields({
  mode,
  currentOrganizationId,
  regionOptions,
  regionsError = false,
}: OrganizationBasicFieldsProps) {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");

  return (
    <Row gutter={16}>
      <Col xs={24} md={12}>
        <Form.Item
          name="slug"
          rules={
            mode === "create"
              ? [
                  {
                    required: true,
                    message: tr("organization.validation.slugRequired", {}),
                  },
                  {
                    validator: (_rule, value: string | undefined) =>
                      !value || /^[a-z0-9](?:[a-z0-9-]*[a-z0-9])$/.test(value)
                        ? Promise.resolve()
                        : Promise.reject(new Error(tr("organization.validation.slugInvalid", {}))),
                  },
                ]
              : undefined
          }
        >
          <GlFormRow
            label={tc("org.slug")}
            required={mode === "create"}
            helpKey="organizations.slug"
          >
            <Input disabled={mode === "edit"} placeholder={tr("organization.placeholder.slug")} />
          </GlFormRow>
        </Form.Item>
      </Col>
      <Col xs={24} md={12}>
        <Form.Item name="parentId">
          <GlFormRow label={tr("organization.parentOrganization")}>
            <OrganizationSelect
              autoFillSessionOrg={false}
              allowClear
              excludeDescendantsOf={mode === "edit" ? currentOrganizationId : undefined}
              placeholder={tr("organization.parentOrganization")}
            />
          </GlFormRow>
        </Form.Item>
      </Col>
      <Col xs={24} md={12}>
        <Form.Item name="regionCode">
          <GlFormRow label={tr("organization.regionCode")}>
            <Select
              allowClear
              showSearch
              optionFilterProp="label"
              disabled={regionsError}
              placeholder={regionsError ? tc("loadFailed") : tr("organization.regionCode")}
              options={regionOptions.map((region) => ({
                label: region.name ? `${region.name} (${region.regionCode})` : region.regionCode,
                value: region.regionCode,
              }))}
            />
          </GlFormRow>
        </Form.Item>
      </Col>
      <Col xs={24} md={12}>
        <Form.Item
          name="displayName"
          rules={[
            {
              required: true,
              message: tr("organization.validation.displayNameRequired", {}),
            },
          ]}
        >
          <GlFormRow label={tc("displayName")} required helpKey="organizations.displayName">
            <Input placeholder={tr("organization.placeholder.displayName")} />
          </GlFormRow>
        </Form.Item>
      </Col>
      <Col xs={24} md={12}>
        <Form.Item
          name="websiteUrl"
          rules={[{ type: "url", message: tr("organization.validation.urlInvalid", {}) }]}
        >
          <GlFormRow label={tr("organization.websiteUrl")}>
            <Input placeholder={tr("organization.placeholder.websiteUrl")} />
          </GlFormRow>
        </Form.Item>
      </Col>
      <Col xs={24} md={12}>
        <Form.Item
          name="status"
          initialValue="active"
          rules={[{ required: true, message: tc("required") }]}
        >
          <GlFormRow label={tc("statusLabel")} required>
            {/* 设计闭集 active / suspended；历史值（normal/disabled/archived）
                由 normalizeOrganizationStatus 在读取时归一，保存写回新值。 */}
            <Select
              options={[
                { label: tc("enumValues.active"), value: "active" },
                { label: tc("enumValues.suspended"), value: "suspended" },
              ]}
            />
          </GlFormRow>
        </Form.Item>
      </Col>
      <Col xs={24} md={12}>
        <Form.Item name="initScore" initialValue={100}>
          <GlFormRow label={tr("organization.initScore")}>
            <InputNumber min={0} max={2147483647} precision={0} style={{ width: "100%" }} />
          </GlFormRow>
        </Form.Item>
      </Col>
    </Row>
  );
}
