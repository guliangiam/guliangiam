// ─── Form value types ──────────────────────────────────────────

/** Values collected by the permission detail form. */
export type PermissionFormValues = {
  displayName: string;
  description: string;
  model: string;
  adapter: string;
  resourceType: string;
  resources: string[];
  actions: string[];
  effect: string;
  domains: string[];
  users: string[];
  groups: string[];
  roles: string[];
  sorter: string;
  application: string;
  state: string;
  isEnabled: boolean;
};
