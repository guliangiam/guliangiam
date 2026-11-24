import type { CreateChannelConfigBody } from "@/shared/api/payments-admin";

/** Mirrors app/payments.maxChannelConfigBytes; the backend rejects oversize plaintext. */
export const MAX_CHANNEL_CONFIG_BYTES = 64 << 10;

export type ChannelConfigFormValues = {
  channel: string;
  environment: string;
  config: string;
};

export type ChannelConfigIssue = "required" | "tooLarge" | "invalid";

/**
 * Validate the plaintext editor before submit. The backend is authoritative:
 * it accepts a non-empty JSON object, canonicalizes it, derives configSet from
 * its top-level keys and seals it with the field-encryption envelope. This
 * client check exists only to keep obviously invalid text from being sent.
 */
export function validateChannelConfigText(value: string | undefined): ChannelConfigIssue | null {
  const trimmed = (value ?? "").trim();
  if (!trimmed) {
    return "required";
  }
  if (new TextEncoder().encode(trimmed).length > MAX_CHANNEL_CONFIG_BYTES) {
    return "tooLarge";
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return "invalid";
  }
  if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
    return "invalid";
  }
  if (Object.keys(parsed as Record<string, unknown>).length === 0) {
    return "invalid";
  }
  return null;
}

/**
 * The request carries a JSON object under `config`, never a pre-sealed blob:
 * the server owns encryption. `configSet` is omitted because the server derives
 * it from the plaintext keys.
 */
export function buildChannelConfigPayload(
  values: ChannelConfigFormValues,
): Pick<CreateChannelConfigBody, "channel" | "environment" | "config"> {
  const issue = validateChannelConfigText(values.config);
  if (issue) {
    throw new Error(`invalid channel config: ${issue}`);
  }
  return {
    channel: values.channel,
    environment: values.environment,
    config: JSON.parse(values.config.trim()) as Record<string, unknown>,
  };
}
