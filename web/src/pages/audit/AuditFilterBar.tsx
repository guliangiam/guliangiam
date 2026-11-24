import { DatePicker, Select, Space } from "antd";
import type { Dayjs } from "dayjs";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { getApplicationOptions } from "@/shared/api/iam/application";

export type AuditFilterValue = {
  applicationId?: string;
  range?: [Dayjs, Dayjs];
};

type AuditFilterBarProps = {
  value: AuditFilterValue;
  onChange: (value: AuditFilterValue) => void;
  /** 登录日志默认展示应用筛选；其他分类同样可用。 */
  showApplication?: boolean;
};
/** 审计中心共用筛选条：应用 + 时间范围。
 *
 * 三个日志页共用，避免各写一套筛选控件与时间格式化逻辑。
 */
export function AuditFilterBar({ value, onChange, showApplication = true }: AuditFilterBarProps) {
  const { t } = useTranslation("resource");
  const { data: applications = [] } = useQuery({
    queryKey: ["applications", "options", "audit"],
    queryFn: () => getApplicationOptions(),
    staleTime: 60_000,
  });
  const applicationOptions = applications.map((application) => ({
    value: application.id,
    label: application.displayName || application.slug,
  }));

  return (
    <Space wrap size={8}>
      {showApplication ? (
        <Select
          allowClear
          showSearch
          aria-label={t("audit.filters.application")}
          style={{ minWidth: 220 }}
          placeholder={t("audit.filters.application")}
          value={value.applicationId}
          options={applicationOptions}
          filterOption={(input, option) =>
            String(option?.label ?? "")
              .toLowerCase()
              .includes(input.toLowerCase())
          }
          onChange={(applicationId?: string) => onChange({ ...value, applicationId })}
        />
      ) : null}
      <DatePicker.RangePicker
        showTime
        value={value.range}
        onChange={(range) =>
          onChange({ ...value, range: (range as [Dayjs, Dayjs] | null) ?? undefined })
        }
        placeholder={[t("audit.filters.startTime"), t("audit.filters.endTime")]}
      />
    </Space>
  );
}
