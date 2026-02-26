export function splitScopes(raw: string): string[] {
  return raw
    .split(/[,\s]+/)
    .map((scope) => scope.trim())
    .filter(Boolean);
}
