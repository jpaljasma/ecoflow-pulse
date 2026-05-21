type ActionErrorBody = {
  message?: unknown;
  details?: unknown;
  error?: unknown;
  issues?: unknown;
};

type ActionErrorLike = {
  body?: unknown;
  message?: unknown;
};

export function formatAvailableDeviceActionError(action: string, error: unknown): string {
  const detail = actionErrorDetail(error);
  return detail ? `${action} failed: ${detail}` : `${action} failed. Try again in a moment.`;
}

function actionErrorDetail(error: unknown): string | undefined {
  if (hasResponseBody(error)) {
    return detailFromBody(error.body) ?? stringDetail(error.message);
  }
  if (error instanceof Error) {
    return error.message;
  }
  if (typeof error === 'string') {
    return error.trim() || undefined;
  }
  return undefined;
}

function hasResponseBody(error: unknown): error is ActionErrorLike {
  return Boolean(error && typeof error === 'object' && 'body' in error);
}

function detailFromBody(body: unknown): string | undefined {
  if (!body || typeof body !== 'object') {
    return undefined;
  }
  const record = body as ActionErrorBody;
  return (
    stringDetail(record.message) ??
    stringDetail(record.details) ??
    stringDetail(record.error) ??
    issueDetail(record.issues)
  );
}

function stringDetail(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value.trim() : undefined;
}

function issueDetail(value: unknown): string | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  for (const issue of value) {
    if (issue && typeof issue === 'object') {
      const message = stringDetail((issue as { message?: unknown }).message);
      if (message) {
        return message;
      }
    }
  }
  return undefined;
}
