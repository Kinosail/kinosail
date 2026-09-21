import assert from 'node:assert/strict';
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { Script } from 'node:vm';
import { browserScriptBundles } from '../quality/browser-script-bundles.mjs';
import { prepareCodeQLJavaScript } from './prepare-codeql-js.mjs';

const repo = fileURLToPath(new URL('../../', import.meta.url));
const manifests = ['packages/webassets/webassets.go', 'apps/player/internal/server/assets.go',
  'apps/subtitles/internal/server/assets.go'];
const files = browserScriptBundles(repo).find(bundle => bundle.name === 'subtitles.playerJS').files;
const start = 'apps/subtitles/internal/server/static/player_streaming_start.js';

function fixture(t) {
  const temp = mkdtempSync(path.join(os.tmpdir(), 'codeql-browser-'));
  const root = path.join(temp, 'repo');
  t.after(() => rmSync(temp, { recursive: true, force: true }));
  for (const file of [...manifests, ...files]) {
    mkdirSync(path.dirname(path.join(root, file)), { recursive: true });
    copyFileSync(path.join(repo, file), path.join(root, file));
  }
  return { root, destination: path.join(temp, 'generated'), temp };
}

test('CodeQL receives the exact complete script served by the Go bundle', t => {
  const { root, destination } = fixture(t);
  prepareCodeQLJavaScript(root, destination);
  const generated = readFileSync(path.join(destination, 'subtitles-player.js'), 'utf8');
  assert.equal(generated, files.map(file => readFileSync(path.join(repo, file), 'utf8')).join(''));
  assert.doesNotThrow(() => new Script(generated));
  assert.throws(() => new Script(readFileSync(path.join(root, start), 'utf8')));
  const config = readFileSync(path.join(repo, '.github/codeql-config.yml'), 'utf8');
  assert.deepEqual(config.split('\n').filter(line => line.startsWith('  - ')), [
    `  - ${start}`, '  - apps/subtitles/internal/server/static/player_streaming_end.js',
  ]);
});

for (const [name, mutate] of [
  ['missing source', ({ root }) => rmSync(path.join(root, start))],
  ['oversized source', ({ root }) => writeFileSync(path.join(root, start), 'x'.repeat(1024 * 1024 + 1))],
  ['invalid UTF-8', ({ root }) => writeFileSync(path.join(root, start), Buffer.from([255]))],
  ['invalid combined syntax', ({ root }) => writeFileSync(path.join(root, start), 'const = ;')],
  ['missing required fragment', ({ root }) => {
    const manifest = path.join(root, manifests[2]);
    writeFileSync(manifest, readFileSync(manifest, 'utf8').replace('playerStreamingStartJS, ', ''));
  }],
  ['unknown dependency', ({ root }) => {
    const manifest = path.join(root, manifests[2]);
    writeFileSync(manifest, readFileSync(manifest, 'utf8').replace('playerStreamingStartJS, ', 'unknownJS, '));
  }],
  ['too many fragments', ({ root }) => {
    const manifest = path.join(root, manifests[2]);
    writeFileSync(manifest, readFileSync(manifest, 'utf8').replace('playerStreamingStartJS, ', 'playerStreamingStartJS, '.repeat(33)));
  }],
  ['source outside repository', ({ root, temp }) => {
    const outside = path.join(temp, 'outside.js');
    writeFileSync(outside, 'const secret = "must not be copied";');
    rmSync(path.join(root, start));
    symlinkSync(outside, path.join(root, start));
  }],
]) {
  test(`rejects ${name} before writing output`, t => {
    const context = fixture(t);
    mutate(context);
    assert.throws(() => prepareCodeQLJavaScript(context.root, context.destination));
    assert.equal(existsSync(context.destination), false);
  });
}
