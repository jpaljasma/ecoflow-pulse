function sanitizeMetricPathSegment(input: string): string {
  return input.trim().replaceAll('{', '_').replaceAll('}', '_').replaceAll(' ', '_');
}

function joinMetricPath(parent: string, child: string): string {
  const clean = sanitizeMetricPathSegment(child);
  if (!parent) {
    return clean;
  }
  if (!clean) {
    return parent;
  }
  return `${parent}.${clean}`;
}

function walkMetricValue(path: string, value: unknown, out: Record<string, number>): void {
  if (typeof value === 'number' && Number.isFinite(value)) {
    if (path) {
      out[path] = value;
    }
    return;
  }
  if (typeof value === 'boolean') {
    if (path) {
      out[path] = value ? 1 : 0;
    }
    return;
  }
  if (Array.isArray(value)) {
    value.forEach((child, index) => {
      walkMetricValue(joinMetricPath(path, String(index)), child, out);
    });
    return;
  }
  if (value && typeof value === 'object') {
    for (const [key, child] of Object.entries(value)) {
      walkMetricValue(joinMetricPath(path, key), child, out);
    }
  }
}

export function extractNumericMetrics(payload: Uint8Array | Buffer | string | undefined): Record<string, number> {
  if (!payload || payload.length === 0) {
    return {};
  }

  let parsed: unknown;
  try {
    const raw = typeof payload === 'string' ? payload : Buffer.from(payload).toString('utf8');
    parsed = JSON.parse(raw);
  } catch {
    return {};
  }

  const out: Record<string, number> = {};
  walkMetricValue('', parsed, out);
  return out;
}
