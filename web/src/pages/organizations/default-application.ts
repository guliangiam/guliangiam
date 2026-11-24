// Organization list renders the default application as a name when the batch
// application-options map has it; otherwise it falls back to the raw id with a
// title so the admin can still identify/copy it.
export function defaultApplicationLabel(
  applicationId: string | undefined,
  nameById: ReadonlyMap<string, string>,
): { label: string; title: string } | null {
  const id = applicationId?.trim();
  if (!id) return null;
  return { label: nameById.get(id) || id, title: id };
}
