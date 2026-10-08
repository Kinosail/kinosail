import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { request } from '../owner.mjs';
async function fixture(t, handler) {
  const server = createServer(handler); await new Promise(done => server.listen(0, '127.0.0.1', done));
  t.after(() => server.close()); return `http://127.0.0.1:${server.address().port}`;
}
test('API adapter refuses another origin before any request/credential side effect', async t => {
  let calls = 0;
  const base = await fixture(t, (_req, res) => { calls++; res.end('{}'); });
  for (const path of ['http://example.test/private', '//example.test/private', 'x'.repeat(4097)]) await assert.rejects(request(base, path, { token: 'private' }));
  assert.equal(calls, 0);
});
test('redirects never forward Owner credentials and HTTP errors with secret bodies stay generic', async t => {
  let leaked = 0;
  const outsider = await fixture(t, (_req, res) => { leaked++; res.end('{}'); });
  const base = await fixture(t, (req, res) => {
    if (req.url === '/redirect') { res.writeHead(302, { Location: outsider }); res.end(); }
    else { res.writeHead(403); res.end('private-token-should-never-appear'); }
  });
  await assert.rejects(request(base, '/redirect', { token: 'private' }));
  await assert.rejects(request(base, '/denied', { token: 'private' }), error => error.message === 'fixture API HTTP 403');
  assert.equal(leaked, 0);
});
test('oversized remote response is bounded before JSON parsing or caller writes', async t => {
  const base = await fixture(t, (_req, res) => { res.writeHead(200, { 'Content-Type': 'application/json' }); res.end('x'.repeat(1024 * 1024 + 1)); });
  await assert.rejects(request(base, '/too-large'), /fixture response too large/);
});
