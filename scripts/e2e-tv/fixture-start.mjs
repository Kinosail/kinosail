import { createServer } from 'node:net';
import { spawn, execFileSync } from 'node:child_process';
import { writeFileSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
const [port] = process.argv.slice(2);
if (process.argv.length !== 3 || !/^\d{4,5}$/.test(port) || +port < 1024 || +port > 65535) throw new Error('invalid fixture port');
// A live listener is a collision, never a reusable fixture.
const probe = createServer();
await new Promise((done, reject) => { probe.once('error', () => reject(new Error('fixture port collision'))); probe.listen(+port, '127.0.0.1', done); });
await new Promise(done => probe.close(done));
const startTime = pid => execFileSync('ps', ['-p', String(pid), '-o', 'lstart='], { encoding: 'utf8' }).trim();
const processes = [{ pid: process.pid, start: startTime(process.pid) }];
const ownedPath = '.e2e/process-owned.json';
writeFileSync(ownedPath, JSON.stringify({ processes }), { mode: 0o600, flag: 'wx' });
const child = spawn(process.execPath, ['../e2e/fixture.mjs', 'player', port], {
  stdio: 'inherit', env: { PATH: process.env.PATH, ...(process.env.TMPDIR ? { TMPDIR: process.env.TMPDIR } : {}), KINOSAIL_E2E_PLAYER_BINARY: resolve('.e2e/bin/player') },
});
if (child.pid) {
  processes.push({ pid: child.pid, start: startTime(child.pid) });
  writeFileSync(ownedPath, JSON.stringify({ processes }), { mode: 0o600 });
}
let timer;
// Record the real Go child when the shared fixture publishes its owned receipt.
const witness = setInterval(() => {
  try {
    const receipt = JSON.parse(readFileSync(`.e2e/fixtures/${port}.json`, 'utf8'));
    if (receipt.supervisorPID === child.pid && Number.isInteger(receipt.childPID) && receipt.childPID > 1 && !processes.some(p => p.pid === receipt.childPID)) {
      processes.push({ pid: receipt.childPID, start: startTime(receipt.childPID) });
      writeFileSync(ownedPath, JSON.stringify({ processes }), { mode: 0o600 });
    }
  } catch {}
}, 100);
witness.unref();
for (const signal of ['SIGTERM', 'SIGINT']) process.on(signal, () => { if (child.exitCode === null) { child.kill(signal); timer = setTimeout(() => child.kill('SIGKILL'), 10000); timer.unref(); } });
child.on('error', () => { process.exitCode = 1; });
child.on('exit', code => { clearInterval(witness); clearTimeout(timer); process.exitCode = code ?? 1; });
