// Detail routes are keyed by organization slug, not display name. Both the
// edit action and impersonation must resolve the slug from the target user's
// organization, otherwise cross-organization targets land in the wrong org.
export function userRouteSlug(
  organizationId: string | undefined,
  orgSlugById: ReadonlyMap<string, string>,
  fallbackSlug: string,
): string {
  return (organizationId && orgSlugById.get(organizationId)) || fallbackSlug;
}

export function userDetailPath(
  organizationId: string | undefined,
  username: string,
  orgSlugById: ReadonlyMap<string, string>,
  fallbackSlug: string,
): string {
  return `/users/${userRouteSlug(organizationId, orgSlugById, fallbackSlug)}/${username}`;
}
