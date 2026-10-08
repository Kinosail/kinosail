import { fixtureItem } from '../../e2e/fixture-response.mjs';
import { readFileSync, readdirSync, lstatSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { PNG } from 'pngjs';
import { test } from '@e2e-dev/mobile';
import { expect } from 'e2e';
import { parseCode } from '../contracts.mjs';
import { readControl } from '../control.mjs';
import { decodedMotion } from '../frames.mjs';
import { registerSecrets } from '../privacy.mjs';
import { owner } from '../owner.mjs';
const root = join(process.cwd(), '.e2e');
const run = readControl();
function frame(relative: string) {
  if (!/^screenshots\/\d{3}-frame-(?:first|second)\.png$/.test(relative)) throw new Error('unexpected frame path');
  const matches: string[] = [];
  function walk(path: string, depth = 0) {
    if (depth > 10) throw new Error('frame tree too deep');
    for (const name of readdirSync(path)) {
      const full = join(path, name), stat = lstatSync(full);
      if (stat.isSymbolicLink()) throw new Error('unsafe frame tree');
      if (stat.isDirectory()) walk(full, depth + 1);
      else if (full.endsWith('/' + relative)) { if (!stat.isFile() || stat.size > 2 * 1024 * 1024) throw new Error('unsafe frame'); matches.push(full); }
    }
  }
  walk(join(root, 'sdk', 'artifacts'));
  if (matches.length !== 1) throw new Error('frame evidence ambiguous');
  const bytes = readFileSync(matches[0]);
  // Bound decoded allocation using PNG's IHDR before the decoder runs.
  if (bytes.length < 24 || bytes.readUInt32BE(16) > 4096 || bytes.readUInt32BE(20) > 4096) throw new Error('oversized decoded frame');
  return PNG.sync.read(bytes);
}
test('phone approval, decoded movie, partial progress and saved connection survive relaunch', async ({ app, device, screen, platform }) => {
  const current = readControl();
  if (JSON.stringify(current) !== JSON.stringify(run) || platform !== run.identity.platform) throw new Error('native control changed before journey');
  const base = `http://127.0.0.1:${run.identity.port}`;
  const admin = await owner(base, root);
  let movie: any;
  await expect.poll(async () => {
    const library = await admin.read('/api/v1/library?view=movies');
    movie = fixtureItem(library, 'Example Movie', 'video');
    return !!movie;
  }).toBe(true);
  const playback = await admin.read(`/api/v1/items/${movie.id}/playback`);
  expect(Number.isFinite(playback.duration) && playback.duration >= 15 && playback.duration <= 17).toBe(true);
  expect(movie.progress?.seconds ?? 0).toBe(0);
  expect(movie.progress?.watched ?? false).toBe(false);
  await device.installApp(undefined, { reinstall: true });
  await app.open();
  const address = platform === 'android' ? '10.0.2.2' : '127.0.0.1';
  await screen.getByLabel('Server address', { exact: true }).fill(`http://${address}:${run.identity.port}`);
  await device.dismissKeyboard();
  await screen.getByRole('button', 'Connect', { exact: true }).tap();
  await expect(screen.getByText('Waiting for approval…', { exact: true })).toBeVisible();
  const values = platform === 'ios'
    ? await screen.getByLabel(/^Approval code [0-9 ]+$/).allTextContents()
    : await screen.getByText(/^\d{6}$/).allTextContents();
  const approval = parseCode(values);
  registerSecrets(root, [approval], 'started');
  await admin.approve(approval);
  await expect(screen.getByText('Waiting for approval…', { exact: true })).toBeHidden();
  const movies = screen.getByRole('tab', 'Movies', { exact: true });
  await movies.tap();
  const card = platform === 'ios' ? screen.getByRole('button', /^Example Movie(?:,|$)/) : screen.getByLabel('Example Movie', { exact: true });
  await expect(card).toBeVisible();
  await card.tap();
  await screen.getByRole('button', 'Play', { exact: true }).tap();
  const pause = screen.getByRole('button', 'Pause', { exact: true });
  await expect(pause).toBeVisible();
  registerSecrets(root, [], 'playback');
  const first = frame(await app.screenshot('frame-first'));
  await delay(700);
  const second = frame(await app.screenshot('frame-second'));
  expect(decodedMotion(first, second)).toBe(true);
  // Rendered changing fixture frames guard decoder startup; Pause alone also appears while buffering.
  if (platform === 'ios') {
    await expect.poll(() => screen.getByLabel('Playback position', { exact: true }).inputValue()).toMatch(/(?:0:0[1-9]|0:1[0-5])/);
  } else {
    await expect.poll(() => device.locator('id=com.kinosail.player.dev:id/exo_position').textContent()).toMatch(/(?:0:0[1-9]|0:1[0-5])/);
  }
  await pause.tap();
  await expect(screen.getByRole('button', 'Play', { exact: true })).toBeVisible();
  if (platform === 'ios') await screen.getByRole('button', 'Close player', { exact: true }).tap();
  else await device.back();
  let progress: any;
  await expect.poll(async () => {
    progress = (await admin.read(`/api/v1/items/${movie.id}`)).item.progress;
    return Number.isFinite(progress.seconds) && progress.seconds > 0 && progress.seconds < playback.duration * .8 && !progress.watched;
  }, { timeout: 15000 }).toBe(true);
  const saved = progress.seconds;
  await device.closeApp();
  await app.open();
  await expect(screen.getByRole('tab', 'Movies', { exact: true })).toBeVisible();
  await screen.getByRole('tab', 'Movies', { exact: true }).tap();
  await expect(card).toBeVisible();
  expect(await screen.getByLabel('Server address', { exact: true }).count()).toBe(0);
  progress = (await admin.read(`/api/v1/items/${movie.id}`)).item.progress;
  expect(progress.seconds).toBe(saved);
  expect(progress.watched ?? false).toBe(false);
  registerSecrets(root, [], 'complete');
  writeFileSync(join(root, 'journey.json'), JSON.stringify({ approval: 'public Owner API and actual dynamic UI code', decodedFrames: true, partialProgressSeconds: saved, watched: false, connectionRestored: true, progressPersisted: true }), { mode: 0o600, flag: 'wx' });
});
