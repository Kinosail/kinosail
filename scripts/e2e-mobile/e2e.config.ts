import type { E2EConfig } from 'e2e';
import { mobile } from '@e2e-dev/mobile';
import { readControl } from './control.mjs';
const run = readControl();
const { platform, device, port } = run.identity;
export default {
  tests: 'tests/**/*.e2e.ts',
  targets: [{
    name: `player-${platform}`,
    engine: mobile({ platform, device, session: `kino-${run.identity.run}`, snapshot: 'full' }),
    app: {
      bundleId: platform === 'ios' ? 'com.kinosail.player' : 'com.kinosail.player.dev',
      appPath: run.appPath,
      environment: 'test',
      command: { executable: 'node', args: ['fixture-start.mjs', port], log: '.e2e/sdk/app.command.log', shutdownTimeout: 15000, startupTimeout: 120000 },
      readyUrl: `http://127.0.0.1:${port}/healthz`,
    },
  }],
  workers: 1, retries: 0, cache: 'off', trace: 'off',
  output: '.e2e/sdk', reporters: ['list'], timeout: 120000, assertionTimeout: 15000,
} satisfies E2EConfig;
