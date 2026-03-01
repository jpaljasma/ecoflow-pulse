import { describe, expect, it } from 'vitest';

import { loadConfig } from '../src/config.js';

describe('pulse-platform config', () => {
  it('defaults local BFF port to 18081 to avoid Expo web collisions', () => {
    const config = loadConfig({});

    expect(config.port).toBe(18081);
  });
});
