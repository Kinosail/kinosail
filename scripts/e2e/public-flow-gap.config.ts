import type { E2EConfig } from 'e2e';
import { playerDeepEngine } from './public-flow-gap-engine.ts';
import { readGapLaunch } from './gap-launch.mjs';
readGapLaunch(process.env.KINOSAIL_E2E_GAP_RUN);
export default {
  tests: ['tests/owner.setup.e2e.ts', 'deep-tests/public-flow-gap.e2e.ts'],
  targets: [{ name: 'player', engine: playerDeepEngine,
    app: { url: 'http://127.0.0.1:0', readyUrl: 'http://127.0.0.1:{port}/healthz', environment: 'test',
      command: { executable: 'node', args: ['fixture.mjs', 'player', '{port}'],
        env: { ...(process.env.KINOSAIL_E2E_PLAYER_BINARY ? { KINOSAIL_E2E_PLAYER_BINARY: process.env.KINOSAIL_E2E_PLAYER_BINARY } : {}) },
        log: '.e2e/logs/player-public-flow-gap.log' } } }],
  workers: 1, retries: 0, cache: 'off', trace: 'off',
  reporters: ['list', 'markdown', 'junit'], assertionTimeout: 15000,
} satisfies E2EConfig;
