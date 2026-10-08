import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';
import { api, movie, requireFixtureURL } from './helpers';
import { requestRestart } from '../restart-control.mjs';

// app.restart() recreates only a browser context. The disposable supervisor
// validates a local request against its own child before replacing that process.
describe('real process durability', { session: 'owner' }, () => {
  test('Owner session, curation and progress survive a bounded Go process restart', async ({ app, browser }) => {
    requireFixtureURL(app.baseUrl);
    await app.open('/settings');
    const item = await movie(browser, app.baseUrl);
    const path = '/api/v1/items/' + item.id;
    expect((await api(browser, app.baseUrl, path + '/progress', 'PUT', { seconds: 3 })).status).toBe(200);
    expect((await api(browser, app.baseUrl, path + '/list', 'PUT', { listed: true })).status).toBe(200);
    const saved = await api(browser, app.baseUrl, path + '/watch-progress');
    const port = new URL(app.baseUrl!).port;
    const receiptPath = join(process.cwd(), '.e2e/fixtures', port + '.json');
    const receipt = () => JSON.parse(readFileSync(receiptPath, 'utf8'));
    const before = receipt();
    expect(before.port).toBe(port);
    expect(before.supervisorPID).toBeGreaterThan(1);
    expect(before.childPID).toBeGreaterThan(1);
    expect(before.generation).toBe(0);
    requestRestart(port, { generation: before.generation, childPID: before.childPID });
    await expect.poll(() => receipt().generation).toBe(1);
    expect(receipt().childPID).not.toBe(before.childPID);
    await expect.poll(async () => {
      try { return (await fetch(new URL('/healthz', app.baseUrl))).status; }
      catch { return 0; }
    }).toBe(200);
    await browser.reload();
    expect((await api(browser, app.baseUrl, path + '/watch-progress')).data).toEqual(saved.data);
    expect((await api(browser, app.baseUrl, path)).data.listed).toBe(true);
    expect((await api(browser, app.baseUrl, '/api/v1/profiles')).status).toBe(200);
    for (const body of [{ seconds: -1 }, { seconds: 1, unknown: true }]) {
      expect((await api(browser, app.baseUrl, path + '/progress', 'PUT', body)).status).toBe(400);
      expect((await api(browser, app.baseUrl, path + '/watch-progress')).data).toEqual(saved.data);
    }
    expect((await api(browser, app.baseUrl, path + '/list', 'PUT', { listed: false })).status).toBe(200);
    await app.screenshot('durable-session-after-process-restart');
  });
});
