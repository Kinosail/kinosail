import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync, spawn } from 'node:child_process';
import { createServer } from 'node:net';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, readdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
const script = resolve('hosted.py');
test('local and malformed requests reject before any build/device/output mutation', t => {
  const cwd = mkdtempSync(join(tmpdir(), 'kino-mobile-guard-'));
  t.after(() => rmSync(cwd, { recursive: true, force: true }));
  mkdirSync(join(cwd, '.e2e')); writeFileSync(join(cwd, '.e2e', 'sentinel'), 'PRESERVE');
  for (const args of [['ios'], ['tvos'], [], ['ios', 'unexpected']]) {
    const result = spawnSync('python3', [script, ...args], { cwd, env: { PATH: process.env.PATH, PYTHONDONTWRITEBYTECODE: '1' }, encoding: 'utf8' });
    assert.notEqual(result.status, 0);
    assert.equal(readFileSync(join(cwd, '.e2e', 'sentinel'), 'utf8'), 'PRESERVE');
    assert.deepEqual(readdirSync(cwd), ['.e2e']);
  }
});
test('occupied fixture port is rejected without starting any Go fixture', async t => {
  const server = createServer();
  await new Promise(done => server.listen(0, '127.0.0.1', done));
  t.after(() => server.close());
  const child = spawn(process.execPath, [resolve('fixture-start.mjs'), String(server.address().port)], { stdio: ['ignore', 'pipe', 'pipe'] });
  let output = ''; child.stderr.on('data', data => { output += data; });
  const code = await new Promise(done => child.on('exit', done));
  assert.notEqual(code, 0); assert.match(output, /fixture port collision/);
});
