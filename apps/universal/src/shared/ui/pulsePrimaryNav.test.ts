import { describe, expect, it } from 'vitest';
import { pulsePrimaryNavItems, resolvePulsePrimaryNavKey } from '@/shared/ui/pulsePrimaryNav';

describe('pulse primary nav', () => {
  it('places energy calendar directly after energy', () => {
    expect(pulsePrimaryNavItems.map((item) => item.key)).toEqual(['devices', 'energy', 'energy-calendar', 'settings', 'search', 'about']);
  });

  it('resolves the energy calendar route as the calendar nav item', () => {
    expect(resolvePulsePrimaryNavKey('energy-calendar')).toBe('energy-calendar');
  });
});
