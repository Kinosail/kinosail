import type { E2EConfig } from 'e2e';
import { readControl } from './control.mjs';
const run = readControl();
export default {
  tests: 'tests/**/*.e2e.ts', targets: [{ name: `player-${run.identity.profile}`, platform: run.identity.profile,
    app: { environment:'test', command:{ executable:'node', args:['fixture-start.mjs',run.identity.port], log:'.e2e/sdk/app.command.log', shutdownTimeout:15000, startupTimeout:120000 }, readyUrl:`http://127.0.0.1:${run.identity.port}/healthz` } }],
  workers:1, retries:0, cache:'off', trace:'off', output:'.e2e/sdk', reporters:['list'], timeout:120000, assertionTimeout:15000,
} satisfies E2EConfig;
