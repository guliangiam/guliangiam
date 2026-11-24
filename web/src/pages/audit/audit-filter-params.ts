import type { AuditFilterValue } from "./AuditFilterBar";

/** 把筛选值转换成接口参数（RFC3339）。 */
export function auditFilterParams(value: AuditFilterValue) {
  return {
    applicationId: value.applicationId || undefined,
    startTime: value.range?.[0]?.toISOString(),
    endTime: value.range?.[1]?.toISOString(),
  };
}
