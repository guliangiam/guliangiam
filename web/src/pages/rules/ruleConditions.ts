import type { IamRuleCondition } from "@/shared/api/iam";

// IP and WAF rules are stored as generic `conditions` rows. These helpers keep
// the detail form's typed inputs (`ipRange`, `wafPattern`) and the generic
// condition editor from overwriting each other: derived rows are excluded from
// the editor and merged back on save.

const IP_OPERATORS = new Set(["eq", "cidr", "range"]);

export function isIPCondition(condition: IamRuleCondition): boolean {
  return (
    condition.field === "ip" &&
    typeof condition.operator === "string" &&
    IP_OPERATORS.has(condition.operator)
  );
}

export function isWAFPatternCondition(condition: IamRuleCondition): boolean {
  return typeof condition.pattern === "string" && condition.pattern.trim() !== "";
}

function splitLines(raw: string | undefined): string[] {
  return (raw ?? "")
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

function splitIPValues(raw: string | undefined): string[] {
  return splitLines(raw)
    .flatMap((line) => line.split(","))
    .map((value) => value.trim())
    .filter(Boolean);
}

export function deriveIPRange(conditions: IamRuleCondition[]): string {
  return conditions
    .filter(isIPCondition)
    .map((condition) => condition.value?.trim())
    .filter((value): value is string => Boolean(value))
    .join(", ");
}

export function deriveWAFPattern(conditions: IamRuleCondition[]): string {
  return conditions
    .filter(isWAFPatternCondition)
    .map((condition) => condition.pattern!.trim())
    .join("\n");
}

/** Conditions the generic field/operator/value editor may show. */
export function editableConditions(conditions: IamRuleCondition[]): IamRuleCondition[] {
  return conditions.filter(
    (condition) => !isIPCondition(condition) && !isWAFPatternCondition(condition),
  );
}

function buildIPConditions(raw: string | undefined): IamRuleCondition[] {
  return splitIPValues(raw).map((value) => ({
    field: "ip",
    operator: value.includes("-") ? "range" : value.includes("/") ? "cidr" : "eq",
    value,
  }));
}

function buildWAFConditions(
  raw: string | undefined,
  existing: IamRuleCondition[],
): IamRuleCondition[] {
  // `target` is not edited by the typed input; keep it when the pattern is
  // unchanged so an advanced WAF target survives a save.
  const targetByPattern = new Map<string, string>();
  for (const condition of existing.filter(isWAFPatternCondition)) {
    const pattern = condition.pattern!.trim();
    if (condition.target) targetByPattern.set(pattern, condition.target);
  }
  return splitLines(raw).map((pattern) => {
    const target = targetByPattern.get(pattern);
    return target ? { pattern, target } : { pattern };
  });
}

export function mergeRuleConditions(
  userConditions: IamRuleCondition[] | undefined,
  ipRange: string | undefined,
  wafPattern: string | undefined,
  existingConditions: IamRuleCondition[],
): IamRuleCondition[] {
  return [
    ...editableConditions(userConditions ?? []),
    ...buildIPConditions(ipRange),
    ...buildWAFConditions(wafPattern, existingConditions),
  ];
}
