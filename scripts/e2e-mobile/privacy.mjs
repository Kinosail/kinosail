// Raw SDK output is private: failure capture includes ordinary pairing codes and QR pixels.
import { constants, openSync, fstatSync, readFileSync, readSync, closeSync, lstatSync, readdirSync, mkdirSync, writeFileSync, renameSync, rmSync, ftruncateSync } from 'node:fs';
import { join } from 'node:path';
import { createHash } from 'node:crypto';
import { parseStrictJSON } from './receipt.mjs';
export function privateFile(path, limit, privateMode = false) {
  const fd = openSync(path, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const stat = fstatSync(fd);
    if (!stat.isFile() || stat.nlink !== 1 || stat.uid !== process.getuid() || stat.size > limit || (privateMode && (stat.mode & 0o777) !== 0o600)) throw new Error('unsafe evidence file');
    const body = Buffer.alloc(limit + 1);
    let size = 0, count;
    while (size <= limit && (count = readSync(fd, body, size, body.length - size, null)) > 0) size += count;
    if (size > limit) throw new Error('evidence file changed');
    return body.subarray(0, size);
  } finally { closeSync(fd); }
}
export function registerSecrets(root, values, stage) {
  const path = join(root, 'secrets.json');
  const record = parseStrictJSON(privateFile(path, 65536, true));
  if (!Array.isArray(values) || values.some(v => typeof v !== 'string' || !v.length || v.length > 4096)) throw new Error('invalid private values');
  record.secrets = [...new Set([...record.secrets, ...values])];
  if (record.secrets.length > 128) throw new Error('private ledger full');
  record.stage = stage;
  // Owned file only; never write a caller-controlled symlink or replace an unrelated file.
  const fd = openSync(path, constants.O_WRONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const stat = fstatSync(fd);
    if (!stat.isFile() || stat.nlink !== 1 || stat.uid !== process.getuid() || (stat.mode & 0o777) !== 0o600) throw new Error('unsafe private ledger');
    const body = JSON.stringify(record);
    ftruncateSync(fd, 0); writeFileSync(fd, body);
  } finally { closeSync(fd); }
}
export function sanitize(root, receipt, { stageOnly = false } = {}) {
  const rootStat = lstatSync(root), sdk = join(root, 'sdk');
  if (!rootStat.isDirectory() || rootStat.isSymbolicLink() || rootStat.uid !== process.getuid()) throw new Error('unsafe owned output root');
  const ledger = parseStrictJSON(privateFile(join(root, 'secrets.json'), 65536, true));
  if (!ledger || Object.keys(ledger).sort().join(',') !== 'secrets,stage' || !Array.isArray(ledger.secrets) || ledger.secrets.length > 128 || ledger.secrets.some(v => typeof v !== 'string' || !v.length || v.length > 4096) || !['started', 'paired', 'playback', 'complete'].includes(ledger.stage)) throw new Error('missing private ledger');
  let count = 0, bytes = 0;
  function inspect(path, depth = 0) {
    const stat = lstatSync(path);
    if (++count > 2048 || depth > 12 || stat.uid !== process.getuid() || stat.isSymbolicLink()) throw new Error('unsafe SDK tree');
    if (stat.isDirectory()) {
      for (const name of readdirSync(path)) {
        if (!/^[A-Za-z0-9_.-]{1,180}$/.test(name)) throw new Error('unsafe SDK filename');
        inspect(join(path, name), depth + 1);
      }
    } else {
      if (!stat.isFile() || stat.nlink !== 1 || stat.size > 2 * 1024 * 1024 || (bytes += stat.size) > 64 * 1024 * 1024 || !/\.(?:json|jsonl|log|txt|md|xml|png|jpg|jpeg|webp|mp4|webm|zip)$/.test(path)) throw new Error('unsafe SDK file');
    }
  }
  inspect(sdk);
  const final = join(root, 'published'), staging = join(root, 'sanitizing');
  for (const path of [final, staging]) {
    try { lstatSync(path); throw new Error('publication already exists'); } catch (error) { if (error.code !== 'ENOENT') throw error; }
  }
  function redact(text) {
    let safe = text;
    for (const value of ledger.secrets.sort((a, b) => b.length - a.length)) {
      safe = safe.replaceAll(value, '[REDACTED]');
      if (/^\d{6}$/.test(value)) safe = safe.replaceAll([...value].join(' '), '[REDACTED]');
    }
    // Also covers a failure before the code read/register step reaches the private ledger.
    return safe.replace(/\b\d(?:[ \t]?\d){5}\b/g, '[REDACTED]');
  }
  function clean(value, depth = 0, redactNumbers = true) {
    if (depth > 40) throw new Error('evidence nesting too deep');
    if (typeof value === 'string') return redact(value);
    if (typeof value === 'number' && redactNumbers && ledger.secrets.includes(String(value))) return '[REDACTED]';
    if (Array.isArray(value)) return value.map(v => clean(v, depth + 1, redactNumbers));
    if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([key,v]) => [key,clean(v, depth + 1, redactNumbers)]));
    return value;
  }
  const safeReceipt = { ...clean(receipt, 0, false), privacy: { stage: ledger.stage, rawPublished: false, pixelsPublished: false, semanticDumpsPublished: false, textRedacted: true } };
  mkdirSync(staging, { mode: 0o700 });
  try {
    const files = [];
    function save(name, text) {
      writeFileSync(join(staging, name), text, { mode: 0o600, flag: 'wx' });
      files.push(`${createHash('sha256').update(text).digest('hex')}  ${name}`);
    }
    // Only these text files may cross publication. No screen dumps, pixels, traces or archives.
    for (const name of ['report.json', 'app.command.log', 'runner.log']) {
      const path = join(sdk, name);
      let text;
      try { text = privateFile(path, 2 * 1024 * 1024).toString('utf8'); } catch (error) { if (error.code === 'ENOENT') continue; throw error; }
      if (name === 'report.json') {
        const report = parseStrictJSON(text);
        // Remove references/hashes to evidence deliberately withheld by this privacy boundary.
        const omit = new Set(['artifacts', 'evidence', 'screenshot', 'screen', 'trace', 'video']);
        text = JSON.stringify(clean(report), (key, value) => omit.has(key) ? (key === 'artifacts' ? [] : undefined) : value, 2);
      }
      save(name, name === 'report.json' ? text : redact(text));
    }
    save('receipt.json', JSON.stringify(safeReceipt, null, 2));
    writeFileSync(join(staging, 'SHA256SUMS'), files.sort().join('\n') + '\n', { mode: 0o600, flag: 'wx' });
    if (!stageOnly) renameSync(staging, final);
    return safeReceipt;
  } catch (error) { rmSync(staging, { recursive: true, force: true }); throw error; }
}
