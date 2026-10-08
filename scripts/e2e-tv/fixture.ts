import { test as base } from 'e2e';
import { createAgentDeviceClient, type AgentDeviceClient } from 'agent-device';
import { createTvActions } from './actions.mjs';
import { readControl } from './control.mjs';
import { tvSelection, withTvSession, type TVSelection } from './tv.mjs';
export const test = base.extend<{ tv: { client: AgentDeviceClient; selection: TVSelection; actions: ReturnType<typeof createTvActions>; run: ReturnType<typeof readControl> } }>({
  tv: async ({}, use) => {
    const run = readControl();
    const selection = tvSelection(run.identity.platform === 'ios'
      ? { platform: 'ios', target: 'tv', udid: run.identity.device }
      : { platform: 'android', target: 'tv', serial: run.identity.device });
    const actions = createTvActions();
    const client = createAgentDeviceClient({ session: `kino-tv-${run.identity.run}`, cwd: process.cwd() });
    await withTvSession(client, selection, async () => { await use({ client, selection, run, actions }); }, run.appPath, actions.step);
  },
});
