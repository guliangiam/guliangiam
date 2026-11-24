// LDAP protocol/attribute identifiers are proper nouns; labels stay equal to the
// stored values so the form never renders a raw i18n key.
export const LDAP_TYPE_OPTIONS = [
  { label: "OpenLDAP", value: "OpenLDAP" },
  { label: "Active Directory", value: "ActiveDirectory" },
];

export const PASSWORD_TYPE_OPTIONS = [
  { label: "Plain", value: "Plain" },
  { label: "SSHA", value: "SSHA" },
  { label: "MD5", value: "MD5" },
];

export const FILTER_FIELD_OPTIONS = ["uid", "mail", "mobile", "sAMAccountName"].map((value) => ({
  label: value,
  value,
}));
