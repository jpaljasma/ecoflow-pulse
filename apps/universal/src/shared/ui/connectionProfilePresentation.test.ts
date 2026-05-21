import { describe, expect, it } from 'vitest';
import { describeConnectionProfileForUi } from '@/shared/ui/connectionProfilePresentation';
import type { ConnectionProfileConfig } from '@/shared/config/env';

const baseProfile = {
  apiUrl: 'https://localhost',
  apiUrlExplicit: true,
  wsUrl: 'wss://localhost/ws',
  wsUrlExplicit: true,
  oidcIssuerUrl: 'https://localhost/realms/pulse',
  oidcClientId: 'pulse-universal-app',
  oidcAudience: '',
  oidcScopes: 'openid profile email offline_access',
  configured: true
} satisfies Omit<ConnectionProfileConfig, 'id' | 'label' | 'edge' | 'dataPlane'>;

describe('connection profile presentation', () => {
  it('names local-edge cloud-data mode without calling it hosted cloud', () => {
    const presentation = describeConnectionProfileForUi({
      ...baseProfile,
      id: 'cloud',
      label: 'Local Edge',
      edge: 'local',
      dataPlane: 'cloud'
    });

    expect(presentation.title).toBe('Local Edge');
    expect(presentation.statusDescription).toBe('Cloud data');
    expect(presentation.detailedDescription).toContain('local HTTPS edge');
  });
});
