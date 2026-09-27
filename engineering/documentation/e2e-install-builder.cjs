#!/usr/bin/env node
// Run against a site built by build.py: node e2e-install-builder.cjs /path/to/site
const assert = require('node:assert/strict');
const { execFileSync } = require('node:child_process');
const { mkdtempSync, readFileSync, writeFileSync } = require('node:fs');
const { tmpdir } = require('node:os');
const { join, resolve } = require('node:path');
const { JSDOM } = require('jsdom');

const site = resolve(process.argv[2] || '');
const page = readFileSync(join(site, 'getting-started/platforms/index.html'), 'utf8');
const script = readFileSync(join(site, 'assets/js/platform-install.js'), 'utf8');
const output = mkdtempSync(join(tmpdir(), 'kinosail-install-e2e-'));
const revision = execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
const composeVersion = execFileSync('docker-compose', ['version'], { encoding: 'utf8' }).trim();

async function verify(app, media, port) {
  const dom = new JSDOM(page, { url: 'https://kinosail.com/getting-started/platforms/', runScripts: 'outside-only' });
  const { window } = dom;
  window.fetch = async url => ({
    ok: true,
    text: async () => readFileSync(join(site, url.slice(1)), 'utf8'),
  });
  window.URL.createObjectURL = () => 'blob:kinosail-test';
  window.URL.revokeObjectURL = () => {};
  window.eval(script);
  const field = name => window.document.querySelector(`[data-install-${name}]`);
  assert.equal(field('builder').hidden, false);
  field('app').value = app;
  field('app').dispatchEvent(new window.Event('change'));
  field('media').value = media;
  field('port').value = String(port);
  field('create').click();
  await new Promise(setImmediate);
  assert.equal(field('error').hidden, true, field('error').textContent);
  assert.equal(field('result').hidden, false);
  assert.equal(field('download').download, `kinosail-${app}-compose.yaml`);
  const yaml = field('preview').textContent;
  assert.ok(yaml.includes(`source: ${JSON.stringify(media)}`));
  assert.ok(!yaml.includes('KINOSAIL_MEDIA_PATH'));
  const file = join(output, field('download').download);
  writeFileSync(file, yaml);
  const config = JSON.parse(execFileSync('docker-compose', ['-f', file, 'config', '--format', 'json'], { encoding: 'utf8' }));
  const service = config.services.kinosail;
  assert.equal(config.name, `kinosail-${app}`);
  assert.equal(service.image, `ghcr.io/kinosail/kinosail-${app}:latest`);
  assert.equal(service.volumes.find(volume => volume.target === '/media').source, media);
  assert.equal(Boolean(service.volumes.find(volume => volume.target === '/media').read_only), app === 'player');
  assert.equal(String(service.ports[0].published), String(port));
  dom.window.close();
  return { app, media, port, file, result: 'pass' };
}

(async () => {
  const cases = [
    await verify('player', '/mnt/tank/Movies & TV', 38127),
    await verify('subtitles', '/srv/media', 49128),
  ];
  const artifact = { revision, command: `node engineering/documentation/e2e-install-builder.cjs ${site}`, environment: { platform: process.platform, node: process.version, compose: composeVersion }, cases };
  writeFileSync(join(output, 'result.json'), JSON.stringify(artifact, null, 2) + '\n');
  console.log(join(output, 'result.json'));
})().catch(error => { console.error(error); process.exitCode = 1; });
