import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { browserScriptBundles } from './browser-script-bundles.mjs';

function fixture(t, player) {
  const root = mkdtempSync(path.join(os.tmpdir(), 'script-bundles-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const [file, source] of [
    ['packages/webassets/webassets.go', '//go:embed static/core.js\ncore []byte\n//go:embed static/util.js\nutil []byte\nShared = append(append([]byte(nil), core...), util...)'],
    ['apps/player/internal/server/assets.go', player],
    ['apps/subtitles/internal/server/assets.go', '//go:embed static/base.css\nbase []byte\n//go:embed static/theme.css\ntheme []byte\nstyles = append(append([]byte(nil), base...), theme...)'],
  ]) {
    const target = path.join(root, file);
    mkdirSync(path.dirname(target), { recursive: true });
    writeFileSync(target, source);
  }
  return root;
}

test('resolves final browser scope and excludes CSS compositions', t => {
  const root = fixture(t, '//go:embed static/app.js\napp []byte\nshared = webassets.Shared\nscript = joinScripts(shared, app)');
  assert.deepEqual(browserScriptBundles(root), [{ name: 'player.script', files: [
    'packages/webassets/static/core.js', 'packages/webassets/static/util.js', 'apps/player/internal/server/static/app.js',
  ] }]);
});

test('rejects unresolved dependencies', t => {
  assert.throws(() => browserScriptBundles(fixture(t, 'script = joinScripts(missing, webassets.Shared)')), /Unknown script composition/);
});

test('rejects mixed CSS and JavaScript', t => {
  const root = fixture(t, '//go:embed static/app.css\nstyle []byte\nscript = joinScripts(style, webassets.Shared)');
  assert.throws(() => browserScriptBundles(root), /Mixed script\/resource composition/);
});

test('rejects circular composition even without a root bundle', t => {
  const root = fixture(t, 'first = joinScripts(second, webassets.Shared)\nsecond = joinScripts(first, webassets.Shared)');
  assert.throws(() => browserScriptBundles(root), /Circular script composition/);
});
