import { fixtureItem } from '../../e2e/fixture-response.mjs';
import { lstatSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import type { AgentDeviceClient, SnapshotNode } from 'agent-device';
import { PNG } from 'pngjs';
import { expect } from 'e2e';
import { test } from '../fixture.ts';
import { type TVSelection, enterServerAddress, focusAndSelect } from '../tv.mjs';
import { readControl } from '../control.mjs';
import { decodedMotion } from '../../e2e-mobile/frames.mjs';
import { parseCode } from '../../e2e-mobile/contracts.mjs';
import { privateFile, registerSecrets } from '../../e2e-mobile/privacy.mjs';
import { owner } from '../../e2e-mobile/owner.mjs';
const button = (node: SnapshotNode) => node.kind === 'button' || node.role === 'button' || /Button$/.test(node.type ?? '');
async function nodes(client: AgentDeviceClient, selection: TVSelection) {
  const snapshot = await client.capture.snapshot({ ...selection, signal: AbortSignal.timeout(15000) });
  if (!Array.isArray(snapshot.nodes) || snapshot.nodes.length > 10000 || snapshot.truncated) throw Error('invalid TV snapshot');
  for (const node of snapshot.nodes) {
    if (!node || typeof node !== 'object' || ['label','value','identifier','ref'].some(key => {
      const value = node[key as 'label']; return value !== undefined && (typeof value !== 'string' || value.length > 2048);
    }) || (node.focused !== undefined && typeof node.focused !== 'boolean')) throw Error('invalid TV node');
  }
  return snapshot.nodes;
}
async function visible(client: AgentDeviceClient, selection: TVSelection, predicate: (node: SnapshotNode) => boolean) {
  let found: SnapshotNode[] = [];
  await expect.poll(async () => { found = (await nodes(client, selection)).filter(predicate); return found.length > 0; }, { timeout: 15000 }).toBe(true);
  return found;
}
async function image(client: AgentDeviceClient, root: string, name: 'first' | 'second') {
  const path = join(root, 'sdk', `frame-${name}.png`);
  const result = await client.capture.screenshot({ path, signal: AbortSignal.timeout(15000) });
  if (result.path !== path) throw Error('foreign TV screenshot path');
  const bytes = privateFile(path, 2 * 1024 * 1024, false);
  const stat = lstatSync(path);
  if (stat.isSymbolicLink() || bytes.length < 24 || bytes.readUInt32BE(16) > 4096 || bytes.readUInt32BE(20) > 4096) throw Error('unsafe TV frame');
  return PNG.sync.read(bytes);
}
test('TV public approval, focused remote movie, decoded motion, pause, Menu and durable relaunch', async ({ tv }) => {
  const { client, selection, run, actions } = tv, root = join(process.cwd(), '.e2e');
  if (JSON.stringify(readControl()) !== JSON.stringify(run)) throw Error('TV control changed');
  const admin = await actions.step('owner', () => owner(`http://127.0.0.1:${run.identity.port}`, root));
  let movie: any;
  const playback = await actions.step('library', async () => {
    await expect.poll(async () => {
      const library = await admin.read('/api/v1/library?view=movies');
      movie = fixtureItem(library, 'Example Movie', 'video'); return !!movie;
    }).toBe(true);
    const playback = await admin.read(`/api/v1/items/${movie.id}/playback`);
    expect(Number.isFinite(playback.duration) && playback.duration >= 15 && playback.duration <= 17).toBe(true);
    expect(movie.progress?.seconds ?? 0).toBe(0); expect(movie.progress?.watched ?? false).toBe(false);
    return playback;
  });
  await actions.step('server_address', () => enterServerAddress(client, selection, run.identity.port));
  await actions.step('connect', () => focusAndSelect(client, selection, 'Connect', 'down'));
  await actions.step('approval', async () => {
    const approvalNodes = await visible(client, selection, n => selection.platform === 'ios' ? /^Approval code [0-9 ]+$/.test(n.label ?? '') : /^\d{6}$/.test(n.label ?? ''));
    const approval = parseCode(approvalNodes.map(n => n.label));
    registerSecrets(root, [approval], 'started'); await admin.approve(approval);
  });
  await actions.step('movies', async () => {
    await visible(client, selection, n => n.label === 'Movies' && button(n));
    await focusAndSelect(client, selection, 'Movies', 'down');
  });
  await actions.step('movie_focus', async () => {
    const cards = await visible(client, selection, n => button(n) && /^Example Movie(?:,|$)/.test(n.label ?? ''));
    if (cards.length !== 1 || !cards[0].label) throw Error('ambiguous fixture card');
    await focusAndSelect(client, selection, cards[0].label, 'down');
  });
  await actions.step('play', async () => {
    if (selection.platform === 'ios') {
      await visible(client, selection, n => n.identifier === `detail.play.${movie.id}`);
      await focusAndSelect(client, selection, `detail.play.${movie.id}`, 'up', 'identifier');
    } else {
      await visible(client, selection, n => n.label === 'Play' && button(n));
      await focusAndSelect(client, selection, 'Play', 'up');
    }
    await visible(client, selection, n => (n.label === 'Pause' && button(n)) || n.label === 'Playback options');
  });
  await actions.step('decoded_frames', async () => {
    registerSecrets(root, [], 'playback');
    const first = await image(client, root, 'first'); await delay(700);
    const second = await image(client, root, 'second'); expect(decodedMotion(first, second)).toBe(true);
  });
  await actions.step('pause', async () => {
    await client.command.tvRemote({ ...selection, button: 'up', signal: AbortSignal.timeout(15000) });
    await visible(client, selection, n => n.label === 'Pause' && button(n));
    await focusAndSelect(client, selection, 'Pause', 'left');
    await visible(client, selection, n => n.label === 'Play' && button(n));
  });
  await actions.step('menu', async () => {
    await client.command.tvRemote({ ...selection, button: selection.platform === 'ios' ? 'menu' : 'back', signal: AbortSignal.timeout(15000) });
    // Return to the real detail and observe its restored Play/Resume focus before leaving.
    await expect.poll(async () => (await nodes(client, selection)).some(n => n.focused === true && (selection.platform === 'ios'
      ? n.identifier === `detail.play.${movie.id}` : button(n) && ['Play','Resume'].includes(n.label ?? ''))), { timeout: 15000 }).toBe(true);
  });
  let progress: any;
  await actions.step('progress', async () => {
    await expect.poll(async () => { progress = (await admin.read(`/api/v1/items/${movie.id}`)).item.progress;
      return Number.isFinite(progress?.seconds) && progress.seconds > 0 && progress.seconds < playback.duration * .8 && progress.watched === false;
    }, { timeout: 15000 }).toBe(true);
  });
  const saved = progress.seconds;
  const app = selection.platform === 'ios' ? 'com.kinosail.player' : 'com.kinosail.player.dev';
  await actions.step('relaunch', async () => {
    await client.apps.close({ app, signal: AbortSignal.timeout(15000) });
    await client.apps.open({ ...selection, app, ...(selection.platform === 'android' ? { activity: 'com.kinosail.player.tv.TvActivity' } : {}), signal: AbortSignal.timeout(30000) });
    const foreground = await client.command.appState({ ...selection, signal: AbortSignal.timeout(15000) });
    if (selection.platform === 'android' && (foreground.platform !== 'android' || foreground.package !== app || !['com.kinosail.player.tv.TvActivity','.tv.TvActivity'].includes(foreground.activity))) throw Error('relaunch lost TV foreground');
    if (selection.platform === 'ios' && (foreground.platform !== 'ios' || foreground.appBundleId !== app)) throw Error('relaunch lost TV foreground');
  });
  await actions.step('restored_connection', async () => {
    await visible(client, selection, n => n.label === 'Movies' && button(n));
    expect((await nodes(client, selection)).some(n => n.label === 'Server address' && n.editable)).toBe(false);
    await focusAndSelect(client, selection, 'Movies', 'down');
    await visible(client, selection, n => button(n) && /^Example Movie(?:,|$)/.test(n.label ?? ''));
  });
  await actions.step('persisted_progress', async () => {
    progress = (await admin.read(`/api/v1/items/${movie.id}`)).item.progress;
    expect(progress.seconds).toBe(saved); expect(progress.watched).toBe(false);
    registerSecrets(root, [], 'complete');
    writeFileSync(join(root, 'journey.json'), JSON.stringify({ approval:'public Owner API and actual dynamic UI code', decodedFrames:true,
      partialProgressSeconds:saved, watched:false, connectionRestored:true, progressPersisted:true, remoteFocus:true, menuReturned:true, tvForeground:true }), { mode:0o600, flag:'wx' });
  });
});
