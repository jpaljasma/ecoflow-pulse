import { describe, expect, it } from 'vitest';

import { loadConfig } from '../src/config.js';

describe('pulse-platform config', () => {
  it('defaults local BFF port to 18081 to avoid Expo web collisions', () => {
    const config = loadConfig({});

    expect(config.port).toBe(18081);
  });

  it('supports optional noop dev subject override', () => {
    const config = loadConfig({
      PULSE_PLATFORM_DEV_SUBJECT: 'jpaljasma@gmail.com'
    });

    expect(config.devUserSubject).toBe('jpaljasma@gmail.com');
  });
});
