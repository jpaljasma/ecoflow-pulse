import { env } from '@/shared/config/env';
import { maybeHandleMockRequest } from '@/shared/api/mockServer';

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

export async function requestJson<T>(
  path: string,
  { method = 'GET', token, body, signal }: RequestOptions = {}
): Promise<T> {
  if (env.apiUrl.startsWith('mock://')) {
    const mock = await maybeHandleMockRequest(path);
    if (!mock) {
      throw new ApiError(`No mock route for ${path}`, 404);
    }
    if (mock.status >= 400) {
      throw new ApiError(`Mock request failed (${mock.status}) for ${path}`, mock.status, mock.body);
    }
    return mock.body as T;
  }

  const url = path.startsWith('http') ? path : `${env.apiUrl}${path}`;
  const headers: Record<string, string> = {
    Accept: 'application/json'
  };

  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
  }

  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const res = await fetch(url, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal
  });

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
