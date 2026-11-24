/** Form.Item 规则：非空时必须是合法 JSON，否则阻断提交（FE-D-08）。
 *  JsonEditor 自身只在视觉上标红，不阻止表单提交，故由调用方挂此规则。 */
export function jsonFieldRule(message: string) {
  return {
    validator: (_: unknown, value: string) => {
      if (!value || !value.trim()) return Promise.resolve();
      try {
        JSON.parse(value);
        return Promise.resolve();
      } catch {
        return Promise.reject(new Error(message));
      }
    },
  };
}

/** 更严格：非空时必须是合法 JSON 对象（非数组/标量），用于 customAttributes
 *  等只接受键值映射的字段（FE-B-10），避免非法输入被 payload 静默丢弃。 */
export function jsonObjectFieldRule(message: string) {
  return {
    validator: (_: unknown, value: string) => {
      if (!value || !value.trim()) return Promise.resolve();
      try {
        const parsed: unknown = JSON.parse(value);
        if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
          return Promise.resolve();
        }
        return Promise.reject(new Error(message));
      } catch {
        return Promise.reject(new Error(message));
      }
    },
  };
}
