import { describe, expect, it } from 'vitest';
import {
  canAccessPulseLogs,
  filterPulsePrimaryNavItems,
  isPulseGlobalAdmin,
  pulsePrimaryNavItems,
  resolvePulsePrimaryNavPressTarget,
  resolvePulsePrimaryNavKey
} from '@/shared/ui/pulsePrimaryNav';

describe('pulse primary nav', () => {
  it('keeps Search routable while hiding it from visible primary navigation', () => {
    expect(pulsePrimaryNavItems.map((item) => item.key)).toEqual(['devices', 'energy', 'energy-calendar', 'logs', 'settings', 'search', 'about']);
    expect(pulsePrimaryNavItems.find((item) => item.key === 'search')?.hiddenFromNav).toBe(true);

    expect(filterPulsePrimaryNavItems({ roles: ['admin'], deviceCount: 0 }).map((item) => item.key)).toEqual([
      'devices',
      'energy',
      'energy-calendar',
      'logs',
      'settings',
      'about'
    ]);
  });

  it('resolves the energy calendar route as the calendar nav item', () => {
    expect(resolvePulsePrimaryNavKey('energy-calendar')).toBe('energy-calendar');
  });

  it('shows Logs for global admins or users with devices', () => {
    expect(isPulseGlobalAdmin(['viewer', 'Admin'])).toBe(true);
    expect(canAccessPulseLogs({ roles: ['viewer'], deviceCount: 0 })).toBe(false);
    expect(canAccessPulseLogs({ roles: ['viewer'], deviceCount: 2 })).toBe(true);
    expect(canAccessPulseLogs({ roles: ['admin'], deviceCount: 0 })).toBe(true);
    expect(filterPulsePrimaryNavItems({ roles: ['viewer'], deviceCount: 0 }).map((item) => item.key)).not.toContain('logs');
    expect(filterPulsePrimaryNavItems({ roles: ['viewer'], deviceCount: 2 }).map((item) => item.key)).toContain('logs');
    expect(filterPulsePrimaryNavItems({ roles: ['admin'], deviceCount: 0 }).map((item) => item.key)).toContain('logs');
  });

  it('uses a web document navigation for Logs while keeping native router navigation', () => {
    const logsItem = pulsePrimaryNavItems.find((item) => item.key === 'logs');
    const devicesItem = pulsePrimaryNavItems.find((item) => item.key === 'devices');

    expect(logsItem).toBeDefined();
    expect(devicesItem).toBeDefined();
    expect(resolvePulsePrimaryNavPressTarget(logsItem!, 'web')).toEqual({ mode: 'document', href: '/logs' });
    expect(resolvePulsePrimaryNavPressTarget(logsItem!, 'ios')).toEqual({ mode: 'router', href: '/(tabs)/logs' });
    expect(resolvePulsePrimaryNavPressTarget(devicesItem!, 'web')).toEqual({ mode: 'router', href: '/(tabs)/devices' });
  });
});
