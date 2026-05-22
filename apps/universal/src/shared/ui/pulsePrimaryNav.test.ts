import { describe, expect, it } from 'vitest';
import {
  filterPulsePrimaryNavItems,
  isPulseGlobalAdmin,
  pulsePrimaryNavItems,
  resolvePulsePrimaryNavKey
} from '@/shared/ui/pulsePrimaryNav';

describe('pulse primary nav', () => {
  it('places energy calendar directly after energy', () => {
    expect(pulsePrimaryNavItems.map((item) => item.key)).toEqual(['devices', 'energy', 'energy-calendar', 'logs', 'settings', 'search', 'about']);
  });

  it('resolves the energy calendar route as the calendar nav item', () => {
    expect(resolvePulsePrimaryNavKey('energy-calendar')).toBe('energy-calendar');
  });

  it('shows Logs only for global admins', () => {
    expect(isPulseGlobalAdmin(['viewer', 'Admin'])).toBe(true);
    expect(filterPulsePrimaryNavItems(['viewer']).map((item) => item.key)).not.toContain('logs');
    expect(filterPulsePrimaryNavItems(['admin']).map((item) => item.key)).toContain('logs');
  });
});
