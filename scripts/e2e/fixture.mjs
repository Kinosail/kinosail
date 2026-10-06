// Disposable host fixture: real app binaries, synthetic media, no containers.
import { spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync, lstatSync, openSync, fstatSync, readSync, closeSync, constants, renameSync, linkSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
const [app, port] = process.argv.slice(2);
if (process.argv.length !== 4 || !['player', 'subtitles'].includes(app) || !/^\d{1,5}$/.test(port) || +port < 1024 || +port > 65535) throw new Error('invalid fixture app/port');
const runtime = Object.fromEntries(['PATH', 'TMPDIR', 'TMP', 'TEMP', 'LANG', 'LC_ALL', 'LC_CTYPE', 'TZ', 'SystemRoot', 'WINDIR'].filter(key => process.env[key] !== undefined).map(key => [key, process.env[key]]));
const root = mkdtempSync(join(tmpdir(), `kinosail-e2e-${app}-`));
for (const dir of ['media/Movies', 'data', 'cache', 'backups']) mkdirSync(join(root, dir), { recursive: true });
const media = join(root, 'media/Movies');
const generated = spawnSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-y', '-f', 'lavfi', '-i', 'testsrc2=size=320x180:rate=24', '-f', 'lavfi', '-i', 'sine=frequency=220:sample_rate=48000', '-t', '16', '-c:v', 'libx264', '-preset', 'ultrafast', '-crf', '30', '-pix_fmt', 'yuv420p', '-c:a', 'aac', '-movflags', '+faststart', join(media, 'Example Movie.mp4')], { stdio: 'inherit', env: runtime });
if (generated.status !== 0) { rmSync(root, { recursive: true, force: true }); throw new Error('FFmpeg fixture failed'); }
writeFileSync(join(media, 'Example Movie.en.srt'), '1\n00:00:00,000 --> 00:00:03,000\nExample dialogue.\n\n2\n00:00:03,500 --> 00:00:07,000\nA second line.\n');
if (app === 'player') {
  const extra = spawnSync('python3', [fileURLToPath(new URL('./media-fixture.py', import.meta.url)), media], { stdio: 'inherit', env: runtime });
  if (extra.status !== 0) { rmSync(root, { recursive: true, force: true }); throw new Error('reader/audio fixture failed'); }
}
const binary = process.env[`KINOSAIL_E2E_${app.toUpperCase()}_BINARY`] ?? join(process.cwd(), '.e2e/bin', app);
const stateDirectory = join(process.cwd(), '.e2e/fixtures');
const statePath = join(stateDirectory, port + '.json');
const pendingPath = statePath + '.pending';
const controlPath = join(stateDirectory, port + '.restart');
let child, stopping = false, restarting = false, generation = 0, timer, controlTimer;
let pendingFD, pendingIdentity, receiptIdentity;
const childEnv = { ...runtime, KINOSAIL_LISTEN: `127.0.0.1:${port}`, KINOSAIL_TLS_ENABLED: 'false', KINOSAIL_MEDIA_DIR: join(root, 'media'), KINOSAIL_DATA_DIR: join(root, 'data'), KINOSAIL_CACHE_DIR: join(root, 'cache'), KINOSAIL_BACKUP_DIR: join(root, 'backups'), KINOSAIL_LIBRARIES: '["Movies"]', KINOSAIL_SCAN_INTERVAL: '24h', KINOSAIL_BACKUP_INTERVAL: '24h', KINOSAIL_SERVER_NAME: 'Kinosail E2E Fixture' };
function owns(path, identity) {
  if (!identity) return false;
  try { const current = lstatSync(path); return current.dev === identity.dev && current.ino === identity.ino; }
  catch (error) { if (error.code !== 'ENOENT') throw error; return false; }
}
function cleanup() {
  clearInterval(controlTimer);
  if (pendingFD !== undefined) { closeSync(pendingFD); pendingFD = undefined; }
  if (owns(pendingPath, pendingIdentity)) rmSync(pendingPath);
  if (owns(statePath, receiptIdentity)) {
    rmSync(controlPath, { force: true });
    rmSync(statePath);
  }
  rmSync(root, { recursive: true, force: true });
}
try {
  mkdirSync(stateDirectory, { recursive: true });
  if (lstatSync(stateDirectory).isSymbolicLink()) throw new Error('invalid fixture receipt directory');
  try { lstatSync(controlPath); throw new Error('fixture control already exists'); } catch (error) { if (error.code !== 'ENOENT') throw error; }
  try { lstatSync(statePath); throw new Error('fixture receipt already exists'); } catch (error) { if (error.code !== 'ENOENT') throw error; }
  pendingFD = openSync(pendingPath, 'wx', 0o600);
  pendingIdentity = fstatSync(pendingFD);
} catch (error) { rmSync(root, { recursive: true, force: true }); throw error; }
function launch() {
  if (pendingFD === undefined) {
    try {
      pendingFD = openSync(pendingPath, 'wx', 0o600);
      pendingIdentity = fstatSync(pendingFD);
    } catch (error) { cleanup(); throw error; }
  }
  child = spawn(binary, [], { stdio: 'inherit', env: childEnv });
  child.on('error', error => { cleanup(); throw error; });
  child.on('exit', code => {
    clearTimeout(timer);
    if (restarting && !stopping) { restarting = false; generation++; launch(); }
    else { cleanup(); process.exitCode ??= code ?? 0; }
  });
  if (!Number.isInteger(child.pid)) return; // The error event owns failed-spawn cleanup.
  try {
    writeFileSync(pendingFD, JSON.stringify({ app, port, supervisorPID: process.pid, childPID: child.pid, generation }));
    closeSync(pendingFD); pendingFD = undefined;
    if (!owns(pendingPath, pendingIdentity)) throw new Error('fixture pending receipt ownership lost');
    if (generation === 0) linkSync(pendingPath, statePath);
    else {
      if (!owns(statePath, receiptIdentity)) throw new Error('fixture receipt ownership lost');
      renameSync(pendingPath, statePath);
    }
    receiptIdentity = pendingIdentity;
    if (generation === 0) rmSync(pendingPath);
    pendingIdentity = undefined;
  } catch (error) {
    clearInterval(controlTimer);
    stopping = true;
    process.exitCode = 1;
    console.error('fixture receipt publication failed', app, generation, error.code === 'EEXIST' ? 'conflict' : 'publication');
    stopChild('SIGTERM'); // Keep the supervisor alive until its owned child exits.
  }
}
function stopChild(signal) {
  child.kill(signal);
  timer = setTimeout(() => child.kill('SIGKILL'), 5000);
  timer.unref();
}
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => {
  stopping = true;
  stopChild(signal);
});
// A request can select only this supervisor's current child, never a caller PID.
controlTimer = setInterval(() => {
  let fd;
  try {
    fd = openSync(controlPath, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
    const control = fstatSync(fd);
    if (!control.isFile() || control.size > 128) return;
    const body = Buffer.alloc(129);
    const size = readSync(fd, body, 0, body.length, 0);
    if (size > 128) return;
    const request = JSON.parse(body.subarray(0, size).toString('utf8'));
    if (!request || body.subarray(0, size).toString('utf8') !== JSON.stringify(request) || Object.keys(request).sort().join(',') !== 'childPID,generation' || request.childPID !== child.pid || request.generation !== generation || stopping || restarting || generation >= 3) return;
    restarting = true;
    stopChild('SIGTERM');
  } catch (error) { if (error.code !== 'ENOENT') console.error('invalid fixture restart request'); }
  finally { if (fd !== undefined) closeSync(fd); rmSync(controlPath, { force: true }); }
}, 100);
controlTimer.unref();
launch();
