import type { Dayjs } from "dayjs";
import type { IamPatchKeyPayload } from "@/shared/api/iam";

/** 与后端 PatchKeyRequest 完全一致的字段集合。 */
export type KeyFormValues = {
  keyType: string;
  algorithm: string;
  publicKey: string;
  privateKey: string;
  displayName: string;
  expiresAt: Dayjs | null;
  isEnabled: boolean;
};

/**
 * BE-B-6: the detail endpoint never returns the stored private key, so the
 * form starts blank. PATCH semantics are "omit = keep, empty string = clear":
 * submitting the blank init value on every save would wipe the stored key.
 * Only send privateKey when the operator actually typed a replacement.
 */
export function buildKeyPatchPayload(values: KeyFormValues, revision: number): IamPatchKeyPayload {
  const payload: IamPatchKeyPayload = {
    displayName: values.displayName,
    algorithm: values.algorithm,
    publicKey: values.publicKey,
    ...(values.expiresAt ? { expiresAt: values.expiresAt.toISOString() } : {}),
    isEnabled: values.isEnabled,
    revision,
  };
  if ((values.privateKey ?? "").trim() !== "") {
    payload.privateKey = values.privateKey;
  }
  return payload;
}
