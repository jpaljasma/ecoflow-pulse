function trimTrailingSlashes(value: string): string {
  return value.replace(/\/+$/, '');
}

function normalizeRequestPath(path: string): string {
  return path.startsWith('/') ? path : `/${path}`;
}

function basePathEndsWithApi(base: string): boolean {
  try {
    const parsed = new URL(base);
    return trimTrailingSlashes(parsed.pathname).endsWith('/api');
  } catch {
    return trimTrailingSlashes(base).endsWith('/api');
  }
}

export function buildApiRequestUrl(base: string, path: string): string {
  if (/^https?:\/\//i.test(path)) {
    return path;
  }

  const normalizedBase = trimTrailingSlashes(base);
  const requestPath = normalizeRequestPath(path);
  const pathWithoutDuplicateApi =
    basePathEndsWithApi(normalizedBase) &&
    (requestPath === '/api' || requestPath.startsWith('/api/'))
      ? requestPath.slice('/api'.length) || ''
      : requestPath;

  return `${normalizedBase}${pathWithoutDuplicateApi}`;
}
