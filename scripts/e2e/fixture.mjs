// Disposable host fixture: real app binaries, synthetic media, no containers.
import { createHash, randomBytes } from 'node:crypto';
import { decodeRecoveryRequest } from './recovery-control.mjs';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync, lstatSync, openSync, fstatSync, readSync, closeSync, constants, renameSync, linkSync, realpathSync, opendirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, isAbsolute, join } from 'node:path';
import { fileURLToPath } from 'node:url';
const [app, port] = process.argv.slice(2);
if (process.argv.length !== 4 || !['player', 'subtitles'].includes(app) || !/^\d{1,5}$/.test(port) || +port < 1024 || +port > 65535) throw new Error('invalid fixture app/port');
const runtime = Object.fromEntries(['PATH', 'TMPDIR', 'TMP', 'TEMP', 'LANG', 'LC_ALL', 'LC_CTYPE', 'TZ', 'SystemRoot', 'WINDIR'].filter(key => process.env[key] !== undefined).map(key => [key, process.env[key]]));
const root = realpathSync(mkdtempSync(join(tmpdir(), `kinosail-e2e-${app}-`)));
const rootIdentity = lstatSync(root);
const rootParent = dirname(root), rootParentIdentity = lstatSync(rootParent);
for (const dir of ['media/Movies', 'data', 'cache', 'backups']) mkdirSync(join(root, dir), { recursive: true });
const media = join(root, 'media/Movies');
const generated = spawnSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-y', '-f', 'lavfi', '-i', 'testsrc2=size=320x180:rate=24', '-f', 'lavfi', '-i', 'sine=frequency=220:sample_rate=48000', '-t', '16', '-c:v', 'libx264', '-preset', 'ultrafast', '-crf', '30', '-pix_fmt', 'yuv420p', '-c:a', 'aac', '-movflags', '+faststart', join(media, 'Example Movie.mp4')], { stdio: 'inherit', env: runtime });
if (generated.status !== 0) { cleanupRoot(); throw new Error('FFmpeg fixture failed'); }
writeFileSync(join(media, 'Example Movie.en.srt'), '1\n00:00:00,000 --> 00:00:03,000\nExample dialogue.\n\n2\n00:00:03,500 --> 00:00:07,000\nA second line.\n');
if (app === 'player') {
  const extra = spawnSync('python3', [fileURLToPath(new URL('./media-fixture.py', import.meta.url)), media], { stdio: 'inherit', env: runtime });
  if (extra.status !== 0) { cleanupRoot(); throw new Error('reader/audio fixture failed'); }
}
const binary = process.env[`KINOSAIL_E2E_${app.toUpperCase()}_BINARY`] ?? join(process.cwd(), '.e2e/bin', app);
const stateDirectory = join(process.cwd(), '.e2e/fixtures');
const statePath = join(stateDirectory, port + '.json');
const pendingPath = statePath + '.pending';
const controlPath = join(stateDirectory, port + '.restart');
let child, stopping = false, restarting = false, generation = 0, timer, controlTimer;
let pendingFD, pendingIdentity, receiptIdentity, controlParents;
let recoveryOperation, recoveryReceipt, recoveryChild, recoveryArchiveIdentity;
const recoveryKey = randomBytes(32).toString('hex');
let binaryIdentity;
const childEnv = { ...runtime, KINOSAIL_LISTEN: `127.0.0.1:${port}`, KINOSAIL_TLS_ENABLED: 'false', KINOSAIL_MEDIA_DIR: join(root, 'media'), KINOSAIL_DATA_DIR: join(root, 'data'), KINOSAIL_CACHE_DIR: join(root, 'cache'), KINOSAIL_BACKUP_DIR: join(root, 'backups'), KINOSAIL_LIBRARIES: '["Movies"]', KINOSAIL_SCAN_INTERVAL: '24h', KINOSAIL_BACKUP_INTERVAL: '24h', KINOSAIL_SERVER_NAME: 'Kinosail E2E Fixture' };
function owns(path, identity) {
  if (!identity) return false;
  try { const current = lstatSync(path); return current.dev === identity.dev && current.ino === identity.ino; }
  catch (error) { if (error.code !== 'ENOENT') throw error; return false; }
}
function cleanupRoot() {
  if (owns(root, rootIdentity)) rmSync(root, { recursive: true, force: true });
}
function cleanup() {
  clearInterval(controlTimer);
  if (pendingFD !== undefined) { closeSync(pendingFD); pendingFD = undefined; }
  if (controlParents && controlOwned() && owns(pendingPath, pendingIdentity)) rmSync(pendingPath);
  if (controlParents && controlOwned() && owns(statePath, receiptIdentity)) {
    rmSync(statePath);
  }
  cleanupRoot();
}
try {
  mkdirSync(stateDirectory, { recursive: true });
  if (lstatSync(stateDirectory).isSymbolicLink()) throw new Error('invalid fixture receipt directory');
  controlParents = [dirname(stateDirectory), stateDirectory].map(path => ({ path, identity: lstatSync(path) }));
  try { lstatSync(controlPath); throw new Error('fixture control already exists'); } catch (error) { if (error.code !== 'ENOENT') throw error; }
  try { lstatSync(statePath); throw new Error('fixture receipt already exists'); } catch (error) { if (error.code !== 'ENOENT') throw error; }
  pendingFD = openSync(pendingPath, 'wx', 0o600);
  pendingIdentity = fstatSync(pendingFD);
} catch (error) { cleanupRoot(); throw error; }
// This capability stays private to the supervisor that allocated and spawned it.
function controlOwned() {
  return controlParents.every(({path, identity}) => owns(path, identity) && !lstatSync(path).isSymbolicLink());
}
function recoveryOwned() {
  return owns(root, rootIdentity) && owns(rootParent, rootParentIdentity)
    && controlOwned()
    && !lstatSync(root).isSymbolicLink() && !lstatSync(binary).isSymbolicLink()
    && lstatSync(binary).isFile() && owns(binary, binaryIdentity);
}
function dataDigest() {
  const hash = createHash('sha256'); let count = 0, bytes = 0;
  function walk(path, prefix) {
    const before = lstatSync(path);
    if (!before.isDirectory() || before.isSymbolicLink()) throw new Error('invalid owned recovery data');
    const directory = opendirSync(path); const entries = [];
    try {
      for (let entry; (entry = directory.readSync());) {
        if (++count > 256) throw new Error('owned recovery data oversized');
        entries.push(entry.name);
      }
    } finally { directory.closeSync(); }
    for (const name of entries.sort()) {
      const current = join(path, name), stat = lstatSync(current);
      if (stat.isSymbolicLink()) throw new Error('invalid owned recovery data');
      if (stat.isDirectory()) walk(current, prefix + name + '/');
      else if (stat.isFile()) {
        if ((bytes += stat.size) > 33554432) throw new Error('owned recovery data oversized');
        const fd = openSync(current, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
        try {
          const opened = fstatSync(fd);
          if (!opened.isFile() || opened.dev !== stat.dev || opened.ino !== stat.ino || opened.size !== stat.size) throw new Error('owned recovery data changed');
          hash.update(prefix + name + '\0'); const buffer = Buffer.alloc(65536); let offset = 0, size;
          while ((size = readSync(fd, buffer, 0, buffer.length, offset))) {
            offset += size; if (offset > stat.size) throw new Error('owned recovery data changed');
            hash.update(buffer.subarray(0, size));
          }
          if (offset !== stat.size || !owns(current, stat)) throw new Error('owned recovery data changed');
          hash.update('\0');
        } finally { closeSync(fd); }
      } else throw new Error('invalid owned recovery data');
    }
    if (!owns(path, before)) throw new Error('owned recovery directory changed');
  }
  walk('data', ''); return hash.digest('hex');
}
function runRecovery() {
  if (app !== 'player' || !recoveryChild || child !== recoveryChild
      || !Number.isInteger(child.pid) || child.exitCode === null && child.signalCode === null
      || ![0, 1].includes(generation) || recoveryOperation !== (generation === 0 ? 'backup' : 'restore')
      || !isAbsolute(binary) || binary.length > 4096 || binary.includes('\0')
      || !recoveryOwned()) throw new Error('invalid owned recovery operation');
  const previous = process.cwd(), previousIdentity = lstatSync(previous);
  const rootFD = openSync(root, constants.O_RDONLY | constants.O_DIRECTORY | constants.O_NOFOLLOW);
  let entered = false;
  try {
    const opened = fstatSync(rootFD);
    if (opened.dev !== rootIdentity.dev || opened.ino !== rootIdentity.ino || !recoveryOwned()) throw new Error('owned recovery root changed');
    process.chdir(root); entered = true;
    if (!owns('.', opened) || !recoveryOwned()) throw new Error('owned recovery root changed');
    for (const name of ['data', 'cache', 'backups', 'media']) {
      const stat = lstatSync(name); if (!stat.isDirectory() || stat.isSymbolicLink()) throw new Error('invalid owned recovery directory');
    }
    const env = { ...childEnv, KINOSAIL_DATA_DIR: './data', KINOSAIL_CACHE_DIR: './cache',
      KINOSAIL_BACKUP_DIR: './backups', KINOSAIL_MEDIA_DIR: './media', KINOSAIL_BACKUP_KEY: recoveryKey };
    function command(args, input) {
      if (!recoveryOwned() || !owns('.', opened)) throw new Error('owned recovery authority changed');
      const result = spawnSync(binary, args, { cwd: '.', env, input, timeout: 30000, killSignal: 'SIGKILL',
        stdio: [input ? 'pipe' : 'ignore', 'pipe', 'ignore'], maxBuffer: 33554432 });
      if (!recoveryOwned() || !owns('.', opened) || result.error || result.signal || result.status === null) throw new Error('owned recovery CLI failed');
      return result;
    }
    if (recoveryOperation === 'backup') {
      const result = command(['backup']);
      if (result.status !== 0 || !result.stdout?.length) throw new Error('owned recovery backup failed');
      const fd = openSync('recovery.backup', constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, 0o600);
      try { recoveryArchiveIdentity = fstatSync(fd); writeFileSync(fd, result.stdout); } finally { closeSync(fd); }
    }
    const fd = openSync('recovery.backup', constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
    let bytes;
    try {
      const stat = fstatSync(fd);
      if (!stat.isFile() || stat.size < 18 || stat.size > 33554432 || !recoveryArchiveIdentity || recoveryArchiveIdentity.dev !== stat.dev || recoveryArchiveIdentity.ino !== stat.ino) throw new Error('invalid owned recovery archive');
      bytes = Buffer.alloc(stat.size + 1); const size = readSync(fd, bytes, 0, bytes.length, 0);
      if (size !== stat.size || !owns('recovery.backup', stat)) throw new Error('owned recovery archive changed');
      bytes = bytes.subarray(0, size);
    } finally { closeSync(fd); }
    if (recoveryOperation === 'restore' && (!recoveryReceipt || bytes.length !== recoveryReceipt.archiveBytes || createHash('sha256').update(bytes).digest('hex') !== recoveryReceipt.archiveSHA256)) throw new Error('owned recovery archive changed');
    if (!bytes.subarray(0, 18).equals(Buffer.from('KINOSAIL-BACKUP-1\n')) || command(['backup', 'verify'], bytes).status !== 0) throw new Error('owned encrypted recovery verification failed');
    const result = { operation: recoveryOperation, verified: true,
      archiveSHA256: createHash('sha256').update(bytes).digest('hex'), archiveBytes: bytes.length };
    if (recoveryOperation === 'restore') {
      const before = dataDigest();
      if (command(['restore'], Buffer.from('deliberately corrupt owned recovery archive')).status === 0 || dataDigest() !== before) throw new Error('corrupt recovery did not preserve owned data');
      if (command(['restore'], bytes).status !== 0) throw new Error('owned recovery restore failed');
      Object.assign(result, { corruptRejected: true, corruptDataUnchanged: true, restored: true });
    }
    return result;
  } finally {
    try { if (entered) { if (!owns(previous, previousIdentity)) throw new Error('fixture cwd changed'); process.chdir(previous); } }
    finally { closeSync(rootFD); }
  }
}
function launch() {
  if (pendingFD === undefined) {
    try {
      pendingFD = openSync(pendingPath, 'wx', 0o600);
      pendingIdentity = fstatSync(pendingFD);
    } catch (error) { cleanup(); throw error; }
  }
  child = spawn(binary, [], { stdio: 'inherit', env: childEnv });
  if (binaryIdentity === undefined && Number.isInteger(child.pid)) binaryIdentity = lstatSync(binary);
  child.on('error', error => { cleanup(); throw error; });
  child.on('exit', code => {
    clearTimeout(timer);
    if (restarting && !stopping) {
      if (recoveryOperation) {
        try { recoveryReceipt = runRecovery(); }
        catch { stopping = true; cleanup(); process.exitCode = 1; console.error('owned fixture recovery failed'); return; }
        recoveryOperation = undefined; recoveryChild = undefined;
      }
      restarting = false; generation++; launch();
    }
    else { cleanup(); process.exitCode ??= code ?? 0; }
  });
  if (!Number.isInteger(child.pid)) return; // The error event owns failed-spawn cleanup.
  try {
    writeFileSync(pendingFD, JSON.stringify({ app, port, supervisorPID: process.pid, childPID: child.pid, generation, ...(recoveryReceipt ? { recovery: recoveryReceipt } : {}) }));
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
  let fd, controlIdentity;
  try {
    if (!controlOwned()) return;
    fd = openSync(controlPath, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
    const control = fstatSync(fd);
    if (!control.isFile()) return;
    controlIdentity = control;
    if (control.size > 192) return;
    const body = Buffer.alloc(193);
    const size = readSync(fd, body, 0, body.length, 0);
    if (size > 192) return;
    const request = JSON.parse(body.subarray(0, size).toString('utf8'));
    if (request?.operation !== undefined) {
      if (app !== 'player' || stopping || restarting || !recoveryOwned()) return;
      recoveryOperation = decodeRecoveryRequest(body.subarray(0, size), child.pid, generation);
      recoveryChild = child;
    } else if (!request || body.subarray(0, size).toString('utf8') !== JSON.stringify(request) || Object.keys(request).sort().join(',') !== 'childPID,generation' || request.childPID !== child.pid || request.generation !== generation || stopping || restarting || generation >= 3) return;
    restarting = true;
    stopChild('SIGTERM');
  } catch (error) { if (error.code !== 'ENOENT') console.error('invalid fixture restart request'); }
  finally {
    try {
      if (controlOwned() && owns(controlPath, controlIdentity)) rmSync(controlPath);
    } catch { console.error('fixture restart cleanup failed'); }
    finally { if (fd !== undefined) closeSync(fd); }
  }
}, 100);
controlTimer.unref();
launch();
