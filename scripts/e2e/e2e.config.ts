import type { E2EConfig } from 'e2e';
import { web } from '@e2e-dev/web';

export default {
  tests: 'tests/**/*.e2e.ts',
  targets: ['player', 'subtitles'].map(name => ({
    name,
    engine: web({ browser: 'chromium', viewport: { width: 1440, height: 900 } }),
    app: {
      url: 'http://127.0.0.1:0',
      readyUrl: 'http://127.0.0.1:{port}/healthz',
      environment: 'test' as const,
      command: {
        executable: 'node', args: ['fixture.mjs', name, '{port}'],
        env: {
          ...(process.env.KINOSAIL_E2E_PLAYER_BINARY ? { KINOSAIL_E2E_PLAYER_BINARY: process.env.KINOSAIL_E2E_PLAYER_BINARY } : {}),
          ...(process.env.KINOSAIL_E2E_SUBTITLES_BINARY ? { KINOSAIL_E2E_SUBTITLES_BINARY: process.env.KINOSAIL_E2E_SUBTITLES_BINARY } : {}),
        },
        log: `.e2e/logs/${name}.log`,
      },
    },
  })),
  workers: 1, retries: 0, cache: 'off', trace: 'off',
  reporters: ['list', 'markdown', 'junit'], assertionTimeout: 15000,
} satisfies E2EConfig;
