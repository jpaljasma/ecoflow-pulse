import { describe, expect, it } from 'vitest';
import {
  resolveCenteredPageMaxWidth,
  resolvePageHorizontalPadding,
  resolvePageHorizontalPaddingPx,
  resolvePageMaxWidth
} from '@/shared/ui/navigationShellMetrics';

describe('navigation shell layout metrics', () => {
  it('uses the shared Pulse page gutter scale across phone tablet and desktop', () => {
    expect(resolvePageHorizontalPadding(390)).toBe('$4');
    expect(resolvePageHorizontalPadding(768)).toBe('$5');
    expect(resolvePageHorizontalPadding(1120)).toBe('$6');
    expect(resolvePageHorizontalPaddingPx(390)).toBe(16);
    expect(resolvePageHorizontalPaddingPx(768)).toBe(20);
    expect(resolvePageHorizontalPaddingPx(1120)).toBe(24);
  });

  it('keeps operating pages wider than centered settings and info pages', () => {
    expect(resolvePageMaxWidth(1440)).toBe(1180);
    expect(resolveCenteredPageMaxWidth(1440)).toBe(1040);

    expect(resolvePageMaxWidth(900)).toBe(980);
    expect(resolveCenteredPageMaxWidth(900)).toBe(920);
  });
});
