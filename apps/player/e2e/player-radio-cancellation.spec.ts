import {expect, test, type Page, type TestInfo} from '@playwright/test';
import {installRadioCancellation, radioSourceHash, type RadioWindow} from './player-radio-cancellation-fixture';

// Failures: unfinished direct restoration owns the cancelled source, native
// replacement projects the saved position without another setter, and stale
// ownership suppresses its first native seek. Automatic replacement must never
// write progress. Existing raw replacement tests cannot reach the radio adapter.
async function proof(page: Page, info: TestInfo, state: Awaited<ReturnType<typeof installRadioCancellation>>, stage: string) {
  const decoder = await page.evaluate(() => (window as RadioWindow).radioDecoder.snapshot());
  const publicProgress = await page.evaluate(async () => (await (await fetch('/api/v1/items/movie', {cache: 'no-store'})).json()).item.progress);
  await info.attach(`${stage}.json`, {body: JSON.stringify({sourceHash: radioSourceHash, stage,
    boundary: 'Controlled decoder, production radio/source projection and intercepted public HTTP; no actual media/device/storage proof',
    decoder, publicProgress, writes: [...state.writes], errors: [...state.errors], adapterRequests: [...state.adapterRequests]}, null, 2),
    contentType: 'application/json'});
  expect(state.errors).toEqual([]);
  expect(state.adapterRequests).toEqual([]);
  expect(decoder.paused).toBe(true);
  expect(decoder.playCalls).toBe(0);
  return {decoder, publicProgress};
}

for (const raw of [undefined, 0, 15]) {
  test(`public Compatibility choice after cancelled direct restore ${raw === undefined ? 'stays unplayed' : `saves native projected ${20 + raw}`}`, async ({page}, info) => {
    const state = await installRadioCancellation(page);
    const pending = await proof(page, info, state, 'pending-direct-restore');
    expect(pending.decoder.raw).toBe(20);
    expect(pending.decoder.seeking).toBe(true);
    expect(state.writes).toEqual([]);
    await page.getByLabel('Compatibility', {exact: true}).check();
    await expect(page.locator('video')).toHaveJSProperty('readyState', 0);
    await expect(page.locator('video')).toHaveJSProperty('currentTime', 20);
    await page.evaluate(() => (window as RadioWindow).radioDecoder.metadata(0, 80));
    const replacement = await proof(page, info, state, 'native-replacement-metadata');
    expect(replacement.decoder.raw).toBe(0);
    expect(replacement.decoder.projected).toBe(20);
    expect(replacement.decoder.duration).toBe(100);
    expect(replacement.decoder.setters).toBe(pending.decoder.setters);
    expect(replacement.decoder.seeking).toBe(false);
    expect(state.writes).toEqual([]);
    if (raw !== undefined) {
      // Decoder clock is independent of the production projected getter/setter.
      // Synthetic clip-zero is movie20, not trusted physical native-zero proof.
      await page.evaluate(value => (window as RadioWindow).radioDecoder.nativeSeek(value), raw);
      await expect.poll(() => state.writes.length).toBe(1);
      expect(state.writes[0]).toMatchObject({seconds: String(20 + raw), session: 'radio-test-session', revision: '1', watched: 'false'});
    } else {
      await page.getByRole('link', {name: 'Library'}).click();
      await expect(page.getByText('Library', {exact: true})).toBeVisible();
      expect(state.writes).toEqual([]);
    }
    if (raw !== undefined) await proof(page, info, state, 'public-checkpoint-readback');
    const publicProgress = await page.evaluate(async () => (await (await fetch('/api/v1/items/movie', {cache: 'no-store'})).json()).item.progress);
    await info.attach('public-readback.json', {body: JSON.stringify({sourceHash: radioSourceHash,
      publicProgress, writes: state.writes, errors: state.errors, adapterRequests: state.adapterRequests}), contentType: 'application/json'});
    expect(state.errors).toEqual([]);
    expect(state.adapterRequests).toEqual([]);
    expect(publicProgress).toEqual({seconds: 20 + (raw ?? 0), watched: false});
  });
}
