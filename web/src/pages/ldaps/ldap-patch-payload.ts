import type { IamPatchLdapPayload } from "@/shared/api/iam/ldap";

export type LdapPatchFormValues = {
  displayName: string;
  serverName: string;
  type: string;
  serverHost: string;
  port: number;
  enableSsl: boolean;
  allowSelfSignedCert: boolean;
  baseDn: string;
  filter: string;
  filterFields: string[];
  admin: string;
  password: string;
  passwordType: string;
  attributeName: string;
  defaultGroups: string[];
  enableGroups: boolean;
  customAttributes: string;
  autoSync: boolean;
  syncInterval: number;
  isEnabled: boolean;
};

/**
 * Build the PATCH payload for an LDAP server.
 *
 * The read API never returns the bind password, so the form field starts
 * empty. An empty field means "keep the stored credential": the key is omitted
 * from the payload. Sending `bindPassword: ""` would be treated by the PATCH
 * handler as a replacement and wipe the stored bind password on every save.
 */
export function buildLdapPatchPayload(
  values: LdapPatchFormValues,
  revision: number,
): IamPatchLdapPayload {
  let customAttributes: Record<string, string> | undefined;
  if (values.customAttributes?.trim()) {
    try {
      const parsed = JSON.parse(values.customAttributes) as unknown;
      if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
        customAttributes = parsed as Record<string, string>;
      }
    } catch {
      // Invalid JSON is ignored, matching the previous inline behaviour.
    }
  }

  const hasNewBindPassword = Boolean(values.password?.trim());
  return {
    displayName: values.displayName,
    serverName: values.serverName,
    type: values.type,
    host: values.serverHost,
    port: values.port,
    enableSsl: values.enableSsl,
    allowSelfSignedCert: values.allowSelfSignedCert,
    baseDn: values.baseDn,
    filter: values.filter,
    filterFields: values.filterFields,
    admin: values.admin,
    ...(hasNewBindPassword ? { bindPassword: values.password } : {}),
    passwordType: values.passwordType,
    attributeName: values.attributeName,
    defaultGroups: values.defaultGroups,
    enableGroups: values.enableGroups,
    customAttributes,
    syncInterval: values.autoSync ? values.syncInterval : 0,
    isEnabled: values.isEnabled,
    revision,
  };
}
