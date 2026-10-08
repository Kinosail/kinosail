import { describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';
import { api, movie, requireFixtureURL } from './helpers';

// Production-process credentials and durable curation, beyond handler fixtures.
describe('access and curation', { session: 'owner' }, () => {
  test('scoped automation key cannot mutate and revocation stops its access', async ({ app, browser }) => {
    requireFixtureURL(app.baseUrl);
    await app.open('/settings');
    const before = await api(browser, app.baseUrl, '/api/v1/api-keys');
    expect(before.status).toBe(200);
    for (const input of [{ name: '', scopes: 'library' }, { name: 'x'.repeat(81), scopes: 'library' }, { name: 'Rejected', scopes: 'unknown' }, { name: 'Rejected', scopes: 'library,unknown' }, { name: 'Rejected', scopes: 'x'.repeat(257) }, { name: 'Rejected', scopes: 'library', unknown: true }]) {
      expect((await api(browser, app.baseUrl, '/api/v1/api-keys', 'POST', input)).status).toBe(400);
      expect((await api(browser, app.baseUrl, '/api/v1/api-keys')).data).toEqual(before.data);
    }
    const name = `Automation ${Date.now()}`;
    const issued = await api(browser, app.baseUrl, '/api/v1/api-keys', 'POST', { name, scopes: 'library' });
    expect(issued.status).toBe(201);
    expect(issued.data.secret).toMatch(/^ks_[A-Z2-7]+$/);
    const keys = await api(browser, app.baseUrl, '/api/v1/api-keys');
    expect(JSON.stringify(keys.data)).not.toContain(issued.data.secret);
    const key = keys.data.keys.find((value: { name: string }) => value.name === name);
    expect(key).toBeDefined();
    const bearer = async (path: string, method = 'GET', body?: object) => fetch(new URL(path, app.baseUrl), {
      method, headers: { Authorization: `Bearer ${issued.data.secret}`, 'Content-Type': 'application/json' },
      ...(body ? { body: JSON.stringify(body) } : {}),
    });
    const item = await movie(browser, app.baseUrl);
    const original = await api(browser, app.baseUrl, `/api/v1/items/${item.id}`);
    try {
      expect((await bearer('/api/v1/library')).status).toBe(200);
      expect((await bearer(`/api/v1/items/${item.id}/list`, 'PUT', { listed: !original.data.listed })).status).toBe(403);
      expect((await api(browser, app.baseUrl, `/api/v1/items/${item.id}`)).data).toEqual(original.data);
    } finally {
      expect((await api(browser, app.baseUrl, `/api/v1/api-keys/${key.id}`, 'DELETE')).status).toBe(204);
    }
    expect((await bearer('/api/v1/library')).status).toBe(401);
  });

  test('My List and collection membership survive reload and rejected mutations', async ({ app, browser }) => {
    requireFixtureURL(app.baseUrl);
    await app.open('/settings');
    const item = await movie(browser, app.baseUrl);
    const path = `/api/v1/items/${item.id}`;
    const original = await api(browser, app.baseUrl, path);
    expect((await api(browser, app.baseUrl, `${path}/list`, 'PUT', { listed: true })).status).toBe(200);
    const name = `Collection ${Date.now()}`;
    const collection = `/api/v1/collections/${encodeURIComponent(name)}`;
    try {
      expect((await api(browser, app.baseUrl, '/api/v1/collections', 'POST', { name })).status).toBe(201);
      expect((await api(browser, app.baseUrl, `${collection}/items/${item.id}`, 'PUT', { included: true })).status).toBe(200);
      await browser.reload();
      expect((await api(browser, app.baseUrl, path)).data.listed).toBe(true);
      const saved = await api(browser, app.baseUrl, collection);
      expect(saved.status).toBe(200);
      expect(saved.data.items.map((value: { id: string }) => value.id)).toContain(item.id);
      for (const body of [{ included: 'yes' }, { included: false, unknown: true }]) {
        expect((await api(browser, app.baseUrl, `${collection}/items/${item.id}`, 'PUT', body)).status).toBe(400);
        expect((await api(browser, app.baseUrl, collection)).data).toEqual(saved.data);
      }
      expect((await api(browser, app.baseUrl, `${path}/list`, 'PUT', { listed: 'yes' })).status).toBe(400);
      expect((await api(browser, app.baseUrl, path)).data.listed).toBe(true);
    } finally {
      expect((await api(browser, app.baseUrl, collection, 'DELETE')).status).toBe(204);
      expect((await api(browser, app.baseUrl, `${path}/list`, 'PUT', { listed: original.data.listed })).status).toBe(200);
    }
    expect((await api(browser, app.baseUrl, collection)).status).toBe(404);
  });
});
