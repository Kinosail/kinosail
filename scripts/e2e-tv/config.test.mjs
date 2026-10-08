import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync, mkdirSync, writeFileSync, readFileSync, lstatSync, realpathSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join, resolve} from 'node:path';
import {randomUUID} from 'node:crypto';

const sdk = new URL(import.meta.resolve('e2e'));
const {resolveConfig} = await import(new URL('./config/resolve.js', sdk));
const config = new URL('./e2e.config.ts', import.meta.url);

function fixture(profile) {
  const project = realpathSync(mkdtempSync(join(tmpdir(), 'kino-tv-config-')));
  const root = join(project, '.e2e'); mkdirSync(root, {mode: 0o700});
  const stat = lstatSync(root), platform = profile === 'tvos' ? 'ios' : 'android';
  const device = platform === 'ios' ? '12345678-1234-1234-1234-123456789ABC' : 'emulator-5554';
  const owned = {run:'123-1', rootDev:stat.dev, rootIno:stat.ino, platform, profile, target:'tv', device,
    emulatorPID:platform === 'ios' ? null : 123, emulatorStart:platform === 'ios' ? null : '12345', complete:false,
    ...(platform === 'ios' ? {creationPending:{name:'Kinosail-TV-E2E-123-1', runtime:'com.apple.CoreSimulator.SimRuntime.tvOS-27-0', type:'com.apple.CoreSimulator.SimDeviceType.Apple-TV-4K-3rd-generation-4K', inventoryZero:true, inventorySHA256:'a'.repeat(64)}} : {})};
  const control = {identity:{platform, profile, target:'tv', device, port:'18769', revision:'b'.repeat(40), run:'123-1'},
    appPath:resolve(project, '../..', platform === 'ios' ? 'apps/player/apps/native/.build/tvos-simulator/Build/Products/Debug-appletvsimulator/KinosailPlayer.app' : 'apps/player/apps/android/app/build/outputs/apk/debug/app-debug.apk')};
  const save = () => {writeFileSync(join(root, 'owned-device.json'), JSON.stringify(owned), {mode:0o600}); writeFileSync(join(root, 'control.json'), JSON.stringify(control), {mode:0o600});};
  save();
  return {project, root, control, owned, save, close:() => rmSync(project, {recursive:true})};
}
async function load(f) {
  const previous = process.cwd();
  try {process.chdir(f.project); return (await import(config.href + '?case=' + randomUUID())).default;}
  finally {process.chdir(previous);}
}

for (const profile of ['tvos', 'androidtv']) test(profile + ' real config passes pinned SDK validator with owned command and readiness', async () => {
    const f = fixture(profile);
    try {
      const raw = await load(f);
      const resolved = resolveConfig(raw, {projectRoot:f.project, env:{}, cli:{}});
      assert.equal(resolved.targets[0].platform, profile);
      assert.equal(raw.targets[0].engine.name, 'kinosail-tv');
      assert.equal(raw.targets[0].engine.spiVersion, 1);
      assert.equal(raw.targets[0].app.command.executable, 'node');
      assert.deepEqual(raw.targets[0].app.command.args, ['fixture-start.mjs', '18769']);
      assert.equal(raw.targets[0].app.readyUrl, 'http://127.0.0.1:18769/healthz');
      assert.equal(readFileSync(join(f.root, 'control.json'), 'utf8'), JSON.stringify(f.control));
    } finally {f.close();}
});

test('invalid owned controls reject config before engine or app provisioning', async () => {
  for (const mutate of [f => delete f.control.identity.profile, f => f.control.identity.profile = 'watchos',
    f => f.control.identity.target = 'mobile', f => f.control.identity.port = '0',
    f => f.control.identity.revision = 'x'.repeat(41), f => f.control.appPath = '/foreign/app',
    f => f.owned.device = '87654321-1234-1234-1234-123456789ABC']) {
    const f = fixture('tvos');
    try {mutate(f); f.save(); await assert.rejects(load(f));
      assert.equal(readFileSync(join(f.root, 'control.json'), 'utf8'), JSON.stringify(f.control));
    } finally {f.close();}
  }
});
