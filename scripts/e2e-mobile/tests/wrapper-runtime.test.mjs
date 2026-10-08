import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createServer } from 'node:net';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, existsSync, statSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';

const wrapper = readFileSync(process.env.KINOSAIL_WRAPPER_CONTRACT_SOURCE ?? resolve('fixture-start.mjs'), 'utf8');
// A real disposable Node subprocess replaces Go only for the wrapper's own lifecycle contract.
// It exposes an actual readiness endpoint, publishes its real child identity, and owns child teardown.
const fixture = `
import { spawn } from 'node:child_process';
import { mkdirSync, writeFileSync, rmSync } from 'node:fs';
const port = process.argv[3];
const path = '.e2e/fixtures/' + port + '.json';
const server = spawn(process.execPath, ['--input-type=module', '-e',
  "import { createServer } from 'node:http'; const service = createServer((_request, response) => response.end('ready')); service.listen(" + port + ", '127.0.0.1'); process.on('SIGTERM', () => service.close(() => process.exit(0)));"], { stdio: 'inherit' });
mkdirSync('.e2e/fixtures', {recursive:true, mode:0o700});
writeFileSync(path, JSON.stringify({supervisorPID:process.pid, childPID:server.pid}), {mode:0o600});
process.on('SIGTERM', () => server.kill('SIGTERM'));
server.on('exit', code => {rmSync(path); process.exit(code ?? 1);});
`;

async function exercise(t, source) {
  const root = mkdtempSync(join(tmpdir(), 'kino-wrapper-runtime-'));
  const cwd = join(root, 'scripts/e2e-mobile');
  mkdirSync(join(root, 'scripts/e2e'), { recursive: true });
  mkdirSync(join(cwd, '.e2e'), { recursive: true, mode: 0o700 });
  writeFileSync(join(root, 'scripts/e2e/fixture.mjs'), fixture);
  writeFileSync(join(cwd, 'fixture-start.mjs'), source);
  const probe = createServer();
  await new Promise(done => probe.listen(0, '127.0.0.1', done));
  const port = probe.address().port;
  await new Promise(done => probe.close(done));
  const child = spawn(process.execPath, ['fixture-start.mjs', String(port)], { cwd, detached: true, stdio: ['ignore', 'pipe', 'pipe'], env: { PATH: process.env.PATH } });
  let output = ''; child.stdout.on('data', b => { output += b; }); child.stderr.on('data', b => { output += b; });
  const exit = new Promise(done => child.on('exit', (code, signal) => done({code, signal})));
  t.after(async () => {
    // This exact group was created by this test; includes only its wrapper and disposable children.
    try { process.kill(-child.pid, 'SIGTERM'); } catch (error) { if (error.code !== 'ESRCH') throw error; }
    await Promise.race([exit, delay(2000)]);
    await delay(100);
    await assert.rejects(fetch(`http://127.0.0.1:${port}/healthz`, { signal: AbortSignal.timeout(250) }));
    rmSync(root, { recursive: true, force: true });
  });
  const deadline = Date.now() + 5000;
  while (Date.now() < deadline && child.exitCode === null) {
    try { if ((await fetch(`http://127.0.0.1:${port}/healthz`, { signal: AbortSignal.timeout(250) })).status === 200) break; } catch {}
    await delay(25);
  }
  if (child.exitCode !== null) return { startup: false, output, exit: await exit };
  const ledgerPath = join(cwd, '.e2e/process-owned.json');
  while (Date.now() < deadline && JSON.parse(readFileSync(ledgerPath, 'utf8')).processes.length !== 3) await delay(25);
  const ledger = JSON.parse(readFileSync(ledgerPath, 'utf8'));
  assert.equal(ledger.processes.length, 3, output);
  assert.equal(ledger.processes[0].pid, child.pid);
  assert.equal(statSync(ledgerPath).mode & 0o777, 0o600);
  for (const owned of ledger.processes) assert.ok(owned.pid > 1 && owned.start.length > 0);
  child.kill('SIGTERM');
  const stopped = await Promise.race([exit, delay(5000).then(() => { throw new Error('wrapper teardown timed out'); })]);
  assert.equal(stopped.code, 0, output);
  assert.equal(existsSync(join(cwd, '.e2e/fixtures', port + '.json')), false);
  for (const owned of ledger.processes) assert.throws(() => process.kill(owned.pid, 0), { code: 'ESRCH' });
  return { startup: true, output, exit: stopped };
}

test('wrapper reaches real child readiness, records all owned processes, and tears them down on SIGTERM', async t => {
  const result = await exercise(t, wrapper);
  assert.equal(result.startup, true, result.output);
  assert.equal(result.output, '');
});

test('negative control: exclusive second creation fails startup with EEXIST', async t => {
  const doubled = wrapper.replace("writeFileSync(ownedPath, JSON.stringify({ processes }), { mode: 0o600 });", "writeFileSync(ownedPath, JSON.stringify({ processes }), { mode: 0o600, flag: 'wx' });");
  const result = await exercise(t, doubled);
  assert.equal(result.startup, false);
  assert.match(result.output, /EEXIST/);
});
