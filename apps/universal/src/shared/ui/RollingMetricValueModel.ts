export type RollingMetricDirection = 'up' | 'down' | 'none';

export type RollingMetricToken =
  | { kind: 'digit'; value: string }
  | { kind: 'static'; value: string };

const NUMBER_PATTERN = /[-+]?\d[\d,]*(?:\.\d+)?/;

export function tokenizeRollingMetricValue(value: string): RollingMetricToken[] {
  const tokens: RollingMetricToken[] = [];
  let staticBuffer = '';

  for (const char of value) {
    if (/\d/.test(char)) {
      if (staticBuffer) {
        tokens.push({ kind: 'static', value: staticBuffer });
        staticBuffer = '';
      }
      tokens.push({ kind: 'digit', value: char });
    } else {
      staticBuffer += char;
    }
  }

  if (staticBuffer) {
    tokens.push({ kind: 'static', value: staticBuffer });
  }

  return tokens.length > 0 ? tokens : [{ kind: 'static', value }];
}

export function parseRollingMetricNumber(value: string): number | null {
  const match = value.match(NUMBER_PATTERN);
  if (!match) return null;
  const parsed = Number.parseFloat(match[0].replace(/,/g, ''));
  return Number.isFinite(parsed) ? parsed : null;
}

export function getRollingMetricDirection(previousValue: string | undefined, nextValue: string): RollingMetricDirection {
  if (!previousValue || previousValue === nextValue) return 'none';
  const previousNumber = parseRollingMetricNumber(previousValue);
  const nextNumber = parseRollingMetricNumber(nextValue);
  if (previousNumber === null || nextNumber === null || previousNumber === nextNumber) return 'none';
  return nextNumber > previousNumber ? 'up' : 'down';
}
