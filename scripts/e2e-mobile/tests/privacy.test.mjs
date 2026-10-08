import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, existsSync, symlinkSync, chmodSync, linkSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { sanitize } from '../privacy.mjs';
function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'kino-mobile-privacy-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  mkdirSync(join(root, 'sdk'), { mode: 0o700 });
  writeFileSync(join(root, 'secrets.json'), JSON.stringify({ secrets: ['839271', 'session-private-token'], stage: 'paired' }), { mode: 0o600 });
  return root;
}
test('published evidence redacts contiguous, spaced, QR URL and credential text; omits all pixels and semantic dumps', t => {
  const root = fixture(t);
  writeFileSync(join(root, 'sdk', 'report.json'), JSON.stringify({ error: '839271 8 3 9 2 7 1 /quick-connect?code=839271 session-private-token', artifacts: [{ path: 'failure.png' }] }));
  mkdirSync(join(root, 'sdk', 'artifacts'));
  writeFileSync(join(root, 'sdk', 'artifacts', 'screen.txt'), 'private semantic QR 839271');
  writeFileSync(join(root, 'sdk', 'failure.png'), 'private pixels');
  writeFileSync(join(root, 'sdk', 'app.command.log'), 'a token session-private-token');
  const result = sanitize(root, { result: 1, revision: 'a'.repeat(40) });
  assert.equal(result.result, 1);
  const report = readFileSync(join(root, 'published', 'report.json'), 'utf8');
  assert.ok(!report.includes('839271') && !report.includes('8 3 9 2 7 1') && !report.includes('session-private-token'));
  assert.deepEqual(JSON.parse(report).artifacts, []);
  assert.ok(!existsSync(join(root, 'published', 'failure.png')));
  assert.ok(!existsSync(join(root, 'published', 'artifacts')));
  assert.match(readFileSync(join(root, 'published', 'SHA256SUMS'), 'utf8'), /^[a-f0-9]{64}  receipt.json/m);
});
for (const scenario of ['missing-sidecar', 'loose-sidecar', 'sidecar-link', 'nested-link', 'hardlink', 'unknown-file', 'oversized', 'existing-publication']) {
  test(`fails closed on ${scenario}, preserves unsafe/non-owned input, creates no publication`, t => {
    const root = fixture(t);
    const outsider = join(root, 'outside.txt');
    writeFileSync(outsider, 'DO NOT CHANGE');
    writeFileSync(join(root, 'sdk', 'report.json'), '{}');
    if (scenario === 'missing-sidecar') rmSync(join(root, 'secrets.json'));
    if (scenario === 'loose-sidecar') chmodSync(join(root, 'secrets.json'), 0o644);
    if (scenario === 'sidecar-link') { rmSync(join(root, 'secrets.json')); symlinkSync(outsider, join(root, 'secrets.json')); }
    if (scenario === 'nested-link') symlinkSync(outsider, join(root, 'sdk', 'link.txt'));
    if (scenario === 'hardlink') linkSync(outsider, join(root, 'sdk', 'hard.txt'));
    if (scenario === 'unknown-file') writeFileSync(join(root, 'sdk', 'private.exe'), 'private');
    if (scenario === 'oversized') writeFileSync(join(root, 'sdk', 'report.json'), 'x'.repeat(2 * 1024 * 1024 + 1));
    if (scenario === 'existing-publication') { mkdirSync(join(root, 'published')); writeFileSync(join(root, 'published', 'sentinel'), 'PRESERVE'); }
    assert.throws(() => sanitize(root, { result: 1 }));
    assert.equal(readFileSync(outsider, 'utf8'), 'DO NOT CHANGE');
    assert.equal(existsSync(join(root, 'published', 'receipt.json')), false);
    if (scenario === 'existing-publication') assert.equal(readFileSync(join(root, 'published', 'sentinel'), 'utf8'), 'PRESERVE');
  });
}
test('malformed and oversized secret sidecars cannot publish', t => {
  const root = fixture(t);
  for (const body of ['{}', '{', JSON.stringify({ secrets: [''], stage: 'paired' }), JSON.stringify({ secrets: ['x'.repeat(4097)], stage: 'paired' })]) {
    writeFileSync(join(root, 'secrets.json'), body, { mode: 0o600 });
    assert.throws(() => sanitize(root, { result: 1 }));
    assert.equal(existsSync(join(root, 'published')), false);
  }
});
