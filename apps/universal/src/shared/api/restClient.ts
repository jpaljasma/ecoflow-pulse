import { env } from '@/shared/config/env';

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly body?: unknown
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

type RequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  token?: string;
  body?: unknown;
  signal?: AbortSignal;
};

function buildApiBaseCandidates(primaryBase: string): string[] {
  if (env.isWeb || env.apiUrlExplicit) {
    return [primaryBase];
  }

  let parsed: URL;
  try {
    parsed = new URL(primaryBase);
  } catch {
    return [primaryBase];
  }

  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return [primaryBase];
  }

  const hostHints = Array.isArray((env as { nativeHostHints?: unknown }).nativeHostHints)
    ? (((env as { nativeHostHints?: unknown }).nativeHostHints as unknown[]) ?? [])
        .filter((value): value is string => typeof value === 'string' && value.length > 0)
    : [];
  const hosts = [parsed.hostname, ...hostHints, '127.0.0.1', 'localhost'];
  const seenBases = new Set<string>();
  const bases: string[] = [];
  const rawPathname = parsed.pathname || '';
  const pathname = rawPathname === '/' ? '' : rawPathname.replace(/\/+$/, '');
  const ports = (() => {
    const list: string[] = [parsed.port];
    if (parsed.protocol === 'http:') {
      if (parsed.port === '18081') list.push('');
      if (!parsed.port) list.push('18081');
    }
    return Array.from(new Set(list));
  })();

  for (const port of ports) {
    for (const hostname of hosts) {
      if (!hostname) continue;
      const host = port ? `${hostname}:${port}` : hostname;
      const base = `${parsed.protocol}//${host}${pathname}`;
      if (seenBases.has(base)) continue;
      seenBases.add(base);
      bases.push(base);
    }
  }

  return bases.length > 0 ? bases : [primaryBase];
}

function isRetryableNetworkError(error: unknown): boolean {
  return error instanceof TypeError;
}

export async function requestJson<T>(
  path: string,
  { method = 'GET', token, body, signal }: RequestOptions = {}
): Promise<T> {
  const urlCandidates = path.startsWith('http')
    ? [path]
    : buildApiBaseCandidates(env.apiUrl).map((base) => `${base}${path}`);
  const attemptedUrls: string[] = [];
  const headers: Record<string, string> = {
    Accept: 'application/json'
  };

  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
  }

  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  for (const [index, url] of urlCandidates.entries()) {
    attemptedUrls.push(url);
    const hasNextCandidate = index < urlCandidates.length - 1;
    let res: Response;
    try {
      res = await fetch(url, {
        method,
        headers,
        body: body !== undefined ? JSON.stringify(body) : undefined,
        signal
      });
    } catch (error) {
      if (hasNextCandidate && isRetryableNetworkError(error)) {
        continue;
      }
      if (isRetryableNetworkError(error)) {
        const cause = error instanceof Error ? error.message : String(error);
        throw new ApiError(
          `Network request failed for ${method} ${path}. Tried: ${attemptedUrls.join(', ')}`,
          0,
          { cause }
        );
      }
      throw error;
    }

    const contentType = res.headers.get('content-type') ?? '';
    const parsedBody = contentType.includes('application/json')
      ? await res.json().catch(() => undefined)
      : await res.text().catch(() => undefined);

    if (!res.ok) {
      throw new ApiError(
        `Request failed (${res.status}) for ${method} ${path}`,
        res.status,
        parsedBody
      );
    }

    if (!contentType.includes('application/json')) {
      const preview =
        typeof parsedBody === 'string'
          ? parsedBody.slice(0, 160).replace(/\s+/g, ' ').trim()
          : undefined;
      throw new ApiError(
        `Expected JSON response for ${method} ${path}, received ${contentType || 'unknown content-type'}${preview ? `: ${preview}` : ''}`,
        res.status,
        parsedBody
      );
    }

    return parsedBody as T;
  }

  throw new ApiError(
    `Request failed before receiving response for ${method} ${path}. Tried: ${attemptedUrls.join(', ')}`,
    0
  );
}
