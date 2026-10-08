import test from 'node:test';
import assert from 'node:assert/strict';
import { tvSelection, withTvSession, focusAndSelect } from './tv.mjs';
const ios = { platform: 'ios', target: 'tv', udid: '12345678-1234-1234-1234-123456789ABC' };
const android = { platform: 'android', target: 'tv', serial: 'emulator-5554' };
function fixture(selection = ios) {
  const calls = [];
  const device = { platform: selection.platform, target: 'tv', id: selection.udid ?? selection.serial,
    kind: selection.platform === 'ios' ? 'simulator' : 'emulator', booted: true,
    ...(selection.platform === 'ios' ? { appleOs: 'tvos' } : {}) };
  const client = {
    devices: { list: async options => { calls.push(['list', options]); return [device]; } },
    apps: { open: async options => { calls.push(['open', options]); return { appBundleId: options.app }; } },
    sessions: { close: async () => { calls.push(['close']); } },
    capture: { snapshot: async options => { calls.push(['snapshot', options]); return { nodes: [{ label: 'Movies', focused: true }] }; } },
    command: { tvRemote: async options => { calls.push(['remote', options]); }, appState: async options => { calls.push(['appstate', options]); return { platform: 'android', package: 'com.kinosail.player.dev', activity: 'com.kinosail.player.tv.TvActivity' }; } },
  };
  return { client, calls, device };
}
test('both finite selectors retain target tv and exact owned virtual-device IDs', () => {
  assert.deepEqual(tvSelection(ios), ios); assert.deepEqual(tvSelection(android), android);
});
test('malformed/unknown/missing/conflicting selectors reject before SDK calls', async () => {
  const bad = [null, [], {}, { ...ios, target: 'mobile' }, { ...ios, target: 'watch' },
    { ...ios, platform: 'macos' }, { ...ios, serial: 'emulator-5554' }, { ...ios, extra: true },
    { ...ios, udid: '' }, { ...ios, udid: 'a'.repeat(257) }, { ...ios, udid: '../device' },
    { ...android, serial: 'device-1' }, { ...android, serial: 'emulator-5554\n' }];
  for (const selection of bad) {
    const { client, calls } = fixture();
    await assert.rejects(() => withTvSession(client, selection, async () => {}), /invalid TV selector/);
    assert.deepEqual(calls, []);
  }
});
test('wrong/missing/duplicate/non-TV/unbooted inventory cannot open an app', async () => {
  for (const change of [d => [], d => [d, d], d => [{ ...d, target: 'mobile' }],
    d => [{ ...d, id: 'foreign' }], d => [{ ...d, appleOs: 'watchos' }],
    d => [{ ...d, platform: 'android' }], d => [{ ...d, kind: 'device' }], d => [{ ...d, booted: false }]]) {
    const { client, calls, device } = fixture();
    client.devices.list = async () => change(device);
    await assert.rejects(() => withTvSession(client, ios, async () => {}), /TV inventory mismatch/);
    assert.deepEqual(calls, []);
  }
});
test('each accepted TV opens the correct package and closes its owned session after use', async () => {
  for (const selection of [ios, android]) {
    const { client, calls } = fixture(selection);
    await withTvSession(client, selection, async pinned => {
      assert.deepEqual(pinned, selection); calls.push(['body']);
    });
    assert.deepEqual(calls.map(c => c[0]), selection.platform === 'ios' ? ['list', 'open', 'body', 'close'] : ['list', 'open', 'appstate', 'body', 'close']);
    assert.equal(calls[1][1].app, selection.platform === 'ios' ? 'com.kinosail.player' : 'com.kinosail.player.dev');
    assert.equal(calls[1][1].target, 'tv');
  }
});
test('partial open, wrong foreground identity and body errors always close the owned session', async () => {
  for (const mode of ['open-failure', 'wrong-app', 'body-failure']) {
    const { client, calls } = fixture();
    if (mode === 'open-failure') client.apps.open = async () => { calls.push(['open']); throw Error('open failed'); };
    if (mode === 'wrong-app') client.apps.open = async () => ({ appBundleId: 'foreign' });
    await assert.rejects(() => withTvSession(client, ios, async () => { throw Error('body failed'); }));
    assert.equal(calls.at(-1)[0], 'close');
  }
});
test('remote movement observes exact focused label before sending Select', async () => {
  const { client, calls } = fixture(); let snapshots = 0;
  client.capture.snapshot = async options => { calls.push(['snapshot', options]); return { nodes: [{ label: 'Movies', focused: ++snapshots > 1 }] }; };
  await focusAndSelect(client, ios, 'Movies', 'down');
  assert.deepEqual(calls.map(c => c[0]), ['snapshot', 'remote', 'snapshot', 'remote']);
  assert.deepEqual(calls.filter(c => c[0] === 'remote').map(c => c[1].button), ['down', 'select']);
  assert.ok(calls.every(c => c[1].target === 'tv'));
});
test('ambiguous/truncated/malformed snapshots cannot Select', async () => {
  for (const snapshot of [{ nodes: [{ label: 'Movies', focused: true }, { label: 'Movies', focused: true }] },
    { nodes: [{ label: 'Movies', focused: true }], truncated: true }, { nodes: null },
    { nodes: Array(10001).fill({}) }]) {
    const { client, calls } = fixture(); client.capture.snapshot = async () => snapshot;
    await assert.rejects(() => focusAndSelect(client, ios, 'Movies', 'down'), /invalid TV snapshot/);
    assert.deepEqual(calls, []);
  }
});
test('unknown direction or invalid label has no SDK side effects', async () => {
  for (const [label, direction] of [['Movies', 'tap'], ['', 'down'], ['x'.repeat(257), 'down'], ['Movies\n', 'down']]) {
    const { client, calls } = fixture();
    await assert.rejects(() => focusAndSelect(client, ios, label, direction), /invalid TV navigation/);
    assert.deepEqual(calls, []);
  }
});
test('unreachable focus is bounded and never Selects', async () => {
  const { client, calls } = fixture(); client.capture.snapshot = async () => ({ nodes: [] });
  await assert.rejects(() => focusAndSelect(client, ios, 'Movies', 'down'), /TV focus not reached/);
  assert.equal(calls.length, 32); assert.ok(calls.every(c => c[1].button === 'down'));
});
test('canceled/failed SDK observations do not issue Select', async () => {
  const { client, calls } = fixture(); client.capture.snapshot = async () => { throw Error('request canceled'); };
  await assert.rejects(() => focusAndSelect(client, ios, 'Movies', 'down'), /request canceled/);
  assert.deepEqual(calls, []);
});

test('Android launch explicitly names TV activity and observes its real foreground identity', async () => {
  const { client, calls } = fixture(android);
  await withTvSession(client, android, async () => {});
  assert.equal(calls[1][1].activity, 'com.kinosail.player.tv.TvActivity');
  assert.equal(calls[2][0], 'appstate');
});
test('Android phone activity, missing/wrong package or foreground data cannot run the TV body', async () => {
  for (const state of [{ platform: 'android', package: 'com.kinosail.player.dev', activity: 'com.kinosail.player.mobile.MobileActivity' },
    { platform: 'android', package: 'foreign', activity: 'com.kinosail.player.tv.TvActivity' }, {},
    { platform: 'ios', appBundleId: 'com.kinosail.player.dev' }]) {
    const { client, calls } = fixture(android); client.command.appState = async () => state;
    let body = false;
    await assert.rejects(() => withTvSession(client, android, async () => { body = true; }), /TV foreground activity mismatch/);
    assert.equal(body, false); assert.equal(calls.at(-1)[0], 'close');
  }
});

test('invalid TV app path rejects before inventory or installation', async () => { for(const path of ['', '../app', '/tmp/phone.app', 'x'.repeat(2049)]) { const {client,calls}=fixture(); await assert.rejects(()=>withTvSession(client,ios,async()=>{},path));assert.deepEqual(calls,[]); } });
test('installation follows TV admission and failed install still closes session',async()=>{const {client,calls}=fixture();client.apps.reinstall=async options=>{calls.push(['install',options]);throw Error('install failure');};await assert.rejects(()=>withTvSession(client,ios,async()=>{},'/tmp/repo/apps/player/apps/native/.build/tvos-simulator/Build/Products/Debug-appletvsimulator/KinosailPlayer.app'),/install failure/);assert.deepEqual(calls.map(c=>c[0]),['list','install','close']);});
test('unknown focus field has no effects and exact identifier can select default Play',async()=>{const {client,calls}=fixture();await assert.rejects(()=>focusAndSelect(client,ios,'detail.play.movie','up','xpath'));assert.deepEqual(calls,[]);client.capture.snapshot=async()=>({nodes:[{identifier:'detail.play.movie',focused:true}]});await focusAndSelect(client,ios,'detail.play.movie','up','identifier');assert.equal(calls[0][1].button,'select');});

test('server input requires actual focused editable field before text entry',async()=>{const {client,calls}=fixture();client.capture.snapshot=async()=>({nodes:[{label:'Server address',type:'TextField',focused:true},{label:'Server address',type:'StaticText'}]});client.interactions={type:async o=>calls.push(['type',o])};const {enterServerAddress}=await import('./tv.mjs');await enterServerAddress(client,ios,'18769');assert.deepEqual(calls.map(c=>c[0]),['remote','type','remote']);assert.equal(calls[1][1].text,'http://127.0.0.1:18769');});
test('server port is finite and invalid input causes no SDK calls',async()=>{const {enterServerAddress}=await import('./tv.mjs');for(const port of ['',18769,'80','18769\n','18770']){const {client,calls}=fixture();await assert.rejects(()=>enterServerAddress(client,ios,port));assert.deepEqual(calls,[]);}});

test('ambiguous traversal app path rejects before SDK inventory',async()=>{const {client,calls}=fixture();await assert.rejects(()=>withTvSession(client,ios,async()=>{},'/tmp/repo/../repo/apps/player/apps/native/.build/tvos-simulator/Build/Products/Debug-appletvsimulator/KinosailPlayer.app'));assert.deepEqual(calls,[]);});
