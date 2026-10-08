import { test, describe } from '@e2e-dev/web';
import { expect } from 'e2e';
import { api, movie, requireFixtureURL } from './helpers';

// These exercise the production process + durable store, unlike handler fixtures.
describe('populated public contracts', { session: 'owner' }, () => {
  test('deep paging, search and rejected queries preserve the catalog', async ({ app, browser }) => {
    requireFixtureURL(app.baseUrl);
    await app.open('/settings');
    const original = await api(browser, app.baseUrl, '/api/v1/library?view=movies');
    expect(original.status).toBe(200);
    expect(original.data.items.map((item: { title: string }) => item.title)).toContain('Example Movie');
    const page = await api(browser, app.baseUrl, '/api/v1/library?view=movies&offset=1&limit=1');
    expect(page.status).toBe(200);
    expect(page.data.items).toHaveLength(0);
    const search = await api(browser, app.baseUrl, '/api/v1/library?q=Example&limit=1');
    expect(search.status).toBe(200);
    expect(search.data.items).toHaveLength(1);
    for (const query of ['limit=0', 'limit=201', 'limit=NaN', 'limit=1&limit=2', 'offset=-1', 'view=unknown', 'q=' + 'x'.repeat(513), 'letter=E&q=Example']) {
      expect((await api(browser, app.baseUrl, `/api/v1/library?${query}`)).status, query).toBe(400);
    }
    expect((await api(browser, app.baseUrl, '/api/v1/library?view=movies')).data).toEqual(original.data);
  });

  test('progress survives navigation and invalid writes have no effect', async ({ app, browser }) => {
    requireFixtureURL(app.baseUrl);
    await app.open('/settings');
    const item = await movie(browser, app.baseUrl);
    const path = `/api/v1/items/${item.id}/progress`;
    expect((await api(browser, app.baseUrl, path, 'PUT', { seconds: 3 })).status).toBe(200);
    await browser.reload();
    const saved = await api(browser, app.baseUrl, `/api/v1/items/${item.id}/watch-progress`);
    expect(saved.status).toBe(200);
    expect(saved.data.seconds).toBe(3);
    const cases = [
      ['omitted', '{}'], ['null body', 'null'], ['null seconds', '{"seconds":null}'],
      ['missing seconds with watched', '{"watched":true}'], ['wrong type', '{"seconds":"3"}'],
      ['boolean', '{"seconds":true}'], ['negative', '{"seconds":-1}'],
      ['too large', '{"seconds":1000000001}'], ['nonfinite', '{"seconds":1e999}'],
      ['NaN encoding', '{"seconds":NaN}'], ['Infinity encoding', '{"seconds":Infinity}'],
      ['unknown', '{"seconds":1,"unknown":true}'], ['duplicate', '{"seconds":1,"seconds":2}'],
      ['conflicting case', '{"seconds":1,"Seconds":2}'], ['malformed', '{'], ['array', '[]'],
      ['trailing JSON', '{"seconds":1}{"seconds":2}'],
      ['oversized', ' '.repeat(1024 * 1024) + '{"seconds":1}'],
    ];
    const results = [];
    for (const [name, body] of cases) {
      expect((await api(browser, app.baseUrl, path, 'PUT', { seconds: 3, watched: false })).status).toBe(200);
      const baseline = (await api(browser, app.baseUrl, path.replace(/\/progress$/, ''))).data.item.progress;
      expect(baseline.updated).toMatch(/^\d{4}-\d\d-\d\dT/);
      const status = await browser.evaluate(async ({ path, body }) => {
        const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')!.content;
        return (await fetch(path, { method: 'PUT', headers: { 'Content-Type': 'application/json', 'X-Kinosail-CSRF': csrf }, body })).status;
      }, { path, body });
      const after = (await api(browser, app.baseUrl, path.replace(/\/progress$/, ''))).data.item.progress;
      results.push({ name, status, unchanged: JSON.stringify(after) === JSON.stringify(baseline) });
    }
    expect(results).toEqual(cases.map(([name]) => ({ name, status: name === 'oversized' ? 413 : 400, unchanged: true })));
    expect((await api(browser, app.baseUrl, path, 'PUT', { seconds: 0 })).status).toBe(200);
    expect((await api(browser, app.baseUrl, path.replace(/\/progress$/, '/watch-progress'))).data.seconds).toBe(0);

  });

  test('invalid Viewer creation cannot change profiles', async ({ app, browser }) => {
    requireFixtureURL(app.baseUrl);
    await app.open('/settings');
    const before = await api(browser, app.baseUrl, '/api/v1/profiles');
    expect(before.status).toBe(200);
    for (const input of [{}, { name: '', password: 'synthetic-viewer-password' }, { name: 'x'.repeat(65), password: 'synthetic-viewer-password' }, { name: 'Viewer', password: 'short' }, { name: 'Viewer', password: 'synthetic-viewer-password', rating: 'unknown', libraries: ['all'] }, { name: 'Viewer', password: 'synthetic-viewer-password', rating: 'all', libraries: ['all'], unknown: true }]) {
      expect((await api(browser, app.baseUrl, '/api/v1/profiles', 'POST', input)).status, JSON.stringify(input)).toBe(400);
      expect((await api(browser, app.baseUrl, '/api/v1/profiles')).data).toEqual(before.data);
    }
  });

  test('Viewer lifecycle persists and deleted credentials stop working', async ({ app, browser }) => {
    requireFixtureURL(app.baseUrl);
    await app.open('/settings');
    const name = `Viewer ${Date.now()}`;
    const created = await api(browser, app.baseUrl, '/api/v1/profiles', 'POST', { name, password: 'synthetic-viewer-password', rating: 'all', libraries: ['all'] });
    expect(created.status).toBe(201);
    const id = created.data.id;
    try {
      await browser.reload();
      const before = await api(browser, app.baseUrl, '/api/v1/profiles');
      expect((await api(browser, app.baseUrl, `/api/v1/profiles/${id}`, 'PUT', { rating: 'unknown', libraries: ['all'] })).status).toBe(400);
      expect((await api(browser, app.baseUrl, '/api/v1/profiles')).data).toEqual(before.data);
      expect((await api(browser, app.baseUrl, '/api/v1/profiles')).data.profiles).toEqual(expect.arrayContaining([expect.objectContaining({ id, name })]));
    } finally {
      expect((await api(browser, app.baseUrl, `/api/v1/profiles/${id}`, 'DELETE')).status).toBe(204);
    }
    const denied = await fetch(new URL('/api/v1/session', app.baseUrl), { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, password: 'synthetic-viewer-password', device: 'Deleted viewer' }) });
    expect(denied.status).toBe(401);
    expect((await api(browser, app.baseUrl, '/api/v1/profiles')).data.profiles.some((profile: { id: string }) => profile.id === id)).toBe(false);
  });

  test('session timeout validation preserves the saved policy', async ({ app, browser }) => {
    requireFixtureURL(app.baseUrl);
    await app.open('/settings');
    const before = await api(browser, app.baseUrl, '/api/v1/settings');
    expect(before.status).toBe(200);
    for (const body of [{}, { inactiveHours: '24', absoluteHours: 168 }, { inactiveHours: 0, absoluteHours: 168 }, { inactiveHours: 48, absoluteHours: 24 }, { inactiveHours: 24, absoluteHours: 8761 }, { inactiveHours: 24, absoluteHours: 168, unknown: true }]) {
      expect((await api(browser, app.baseUrl, '/api/v1/settings/session-timeouts', 'PUT', body)).status).toBe(400);
      expect((await api(browser, app.baseUrl, '/api/v1/settings')).data).toEqual(before.data);
    }
  });

  test('phone and desktop render real media and reachable account settings', async ({ app, browser, screen }) => {
    requireFixtureURL(app.baseUrl);
    await app.open('/settings');
    const subtitles = (await browser.title()).includes('Subtitles');
    for (const width of [390, 1440]) {
      await browser.setViewport({ width, height: 900 });
      await app.open(subtitles ? '/?view=library' : '/?view=movies');
      await expect(subtitles ? screen.getByText('Example Movie', { exact: true }) : screen.getByRole('heading', /Example Movie/)).toBeVisible();
      expect(await browser.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await app.screenshot(`movies-${width}`);
      await app.open('/account');
      await expect(screen.getByRole('heading', 'Ways to sign in', { exact: true })).toBeVisible();
      await app.open('/settings/configuration');
      await expect(browser).toHaveURL('/settings/configuration');
      await expect(screen.getByRole('main')).toBeVisible();
    }
  });
});

test('anonymous access cannot read Owner data or mutate configuration', async ({ app, browser }) => {
    requireFixtureURL(app.baseUrl);
  await app.open('/login');
  for (const [path, method, body] of [['/api/v1/profiles', 'GET', undefined], ['/api/v1/settings', 'GET', undefined], ['/api/v1/settings/server', 'PUT', { name: 'Unauthorized change' }]] as const) {
    const response = await api(browser, app.baseUrl, path, method, body);
    expect(response.status).toBe(401);
    expect(JSON.stringify(response.data)).not.toContain('synthetic-e2e-password');
  }
});
