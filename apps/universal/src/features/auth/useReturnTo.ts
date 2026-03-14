export function sanitizeReturnTo(value: string | string[] | undefined): string | null {
  const raw = Array.isArray(value) ? value[0] : value;
  const trimmed = typeof raw === 'string' ? raw.trim() : '';
  if (!trimmed) {
    return null;
  }
  if (!trimmed.startsWith('/')) {
    return null;
  }
  if (trimmed.startsWith('//') || trimmed.includes('://')) {
    return null;
  }
  if (trimmed === '/' || trimmed.startsWith('/login')) {
    return null;
  }
  return trimmed;
}

export function resolvePostLoginTarget(
  returnTo: string | null,
  deviceCount: number | undefined
): string {
  if (returnTo) {
    return returnTo;
  }
  if ((deviceCount ?? 0) > 0) {
    return '/devices';
  }
  return '/onboarding';
}

export function buildReturnTo(pathname: string, params: Record<string, string | string[] | undefined>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (key === 'returnTo') {
      continue;
    }
    if (typeof value === 'string' && value.trim()) {
      search.set(key, value);
      continue;
    }
    if (Array.isArray(value)) {
      for (const item of value) {
        if (typeof item === 'string' && item.trim()) {
          search.append(key, item);
        }
      }
    }
  }
  const query = search.toString();
  return query ? `${pathname}?${query}` : pathname;
}
