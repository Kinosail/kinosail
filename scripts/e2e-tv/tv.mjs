// @ts-check
import { resolve } from 'node:path';
// agent-device v0.21.22 known codes; arbitrary daemon-originated strings stay unqualified.
export const tvCaptureCodes=Object.freeze(['INVALID_ARGS','DEVICE_NOT_FOUND','DEVICE_IN_USE','TOOL_MISSING','APP_NOT_INSTALLED',
  'UNSUPPORTED_PLATFORM','UNSUPPORTED_OPERATION','NOT_IMPLEMENTED','COMMAND_FAILED','SESSION_NOT_FOUND','UNAUTHORIZED',
  'AMBIGUOUS_MATCH','REPLAY_DIVERGENCE','REPAIR_SESSION_EXPIRED','REPAIR_COMMIT_FAILED','unqualified']);
function captureCode(error) {
  try {const code=Object.getOwnPropertyDescriptor(error,'code')?.value;return tvCaptureCodes.includes(code)?code:'unqualified';}
  catch {return 'unqualified';}
}
const addressFailures = new WeakMap();
/** @param {unknown} error */
export function tvAddressFailure(error) {
  return error && typeof error === 'object' ? addressFailures.get(error) : undefined;
}
// A finite TV SDK fixture. Caller first validates V7 private run ownership.
/** @typedef {import('agent-device').AgentDeviceClient} TVClient */
/** @typedef {{platform:'ios',target:'tv',udid:string}|{platform:'android',target:'tv',serial:string}} TVSelection */
/** @param {unknown} value @returns {TVSelection} */
export function tvSelection(value) {
  const input = /** @type {Record<string,unknown>} */ (value);
  if (!input || typeof input !== 'object' || Array.isArray(input) ||
      ![Object.prototype, null].includes(Object.getPrototypeOf(input))) throw Error('invalid TV selector');
  const apple = input.platform === 'ios';
  if ((!apple && input.platform !== 'android') || input.target !== 'tv' ||
      Object.keys(input).sort().join(',') !== (apple ? 'platform,target,udid' : 'platform,serial,target')) throw Error('invalid TV selector');
  const id = apple ? input.udid : input.serial;
  if (typeof id !== 'string' || !(apple
    ? /^[A-Fa-f0-9]{8}(?:-[A-Fa-f0-9]{4}){3}-[A-Fa-f0-9]{12}$/
    : /^emulator-[0-9]{4,5}$/).test(id)) throw Error('invalid TV selector');
  return apple ? { platform: 'ios', target: 'tv', udid: id } : { platform: 'android', target: 'tv', serial: id };
}
/** @param {TVClient} client @param {unknown} input @param {(selection:TVSelection)=>Promise<unknown>} use @param {string|undefined} [appPath] @param {<T>(stage:string,operation:()=>Promise<T>)=>Promise<T>} [observe] */
export async function withTvSession(client, input, use, appPath, observe = async (_, operation) => operation()) {
  if (typeof observe !== 'function') throw Error('invalid TV observer');
  const selection = tvSelection(input), apple = selection.platform === 'ios';
  if (appPath !== undefined && (typeof appPath !== 'string' || appPath.length > 2048 || resolve(appPath) !== appPath || !/^\/(?:[^\x00-\x1f\x7f]+\/)?apps\/player\/apps\/(?:native\/\.build\/tvos-simulator\/Build\/Products\/Debug-appletvsimulator\/KinosailPlayer\.app|android\/app\/build\/outputs\/apk\/debug\/app-debug\.apk)$/.test(appPath) || (apple ? !appPath.endsWith('KinosailPlayer.app') : !appPath.endsWith('app-debug.apk')))) throw Error('invalid TV app path');
  const id = selection.platform === 'ios' ? selection.udid : selection.serial;
  const app = apple ? 'com.kinosail.player' : 'com.kinosail.player.dev';
  let actionFailed = false;
  try {
    await observe('inventory', async () => {
      const inventory = await client.devices.list({ ...selection, signal: AbortSignal.timeout(15000) });
      if (!Array.isArray(inventory) || inventory.length > 128) throw Error('TV inventory mismatch');
      const matches = inventory.filter(d => d && d.id === id);
      if (matches.length !== 1 || matches[0].platform !== selection.platform || matches[0].target !== 'tv' ||
          matches[0].kind !== (apple ? 'simulator' : 'emulator') || matches[0].booted !== true ||
          (apple && matches[0].appleOs !== 'tvos')) throw Error('TV inventory mismatch');
    });
    if (appPath !== undefined) await observe('reinstall', () => client.apps.reinstall({ ...selection, app, appPath, signal: AbortSignal.timeout(60000) }));
    await observe('open', async () => {
      const opened = await client.apps.open({ ...selection, app, ...(apple ? {} : { activity: 'com.kinosail.player.tv.TvActivity' }), signal: AbortSignal.timeout(30000) });
      if ((opened?.appBundleId ?? opened?.appId) !== app) throw Error('TV foreground identity mismatch');
    });
    if (!apple) await observe('foreground', async () => {
        const state = await client.command.appState({ ...selection, signal: AbortSignal.timeout(15000) });
        if (state.platform !== 'android' || state.package !== app ||
            !['com.kinosail.player.tv.TvActivity', '.tv.TvActivity'].includes(state.activity)) throw Error('TV foreground activity mismatch');
    });
    return await observe('use', () => use(selection));
  } catch (error) { actionFailed = true; throw error; } finally {
    // Client is bound by its caller to this run's unique private session.
    let closing = false, closeFailed = false;
    const close = async () => {
      closing = true;
      try { return await client.sessions.close({ signal: AbortSignal.timeout(15000) }); }
      catch (error) { closeFailed = true; throw error; }
    };
    try { await observe('close', close); }
    catch (error) {
      if (!closing) await close();
      // A diagnostic failure after successful cleanup must not mask the action.
      if (!actionFailed || closeFailed) throw error;
    }
  }
}
/** @param {TVClient} client @param {unknown} input @param {string} label @param {string} direction @param {string} [field] */
export async function focusAndSelect(client, input, label, direction, field = 'label') {
  const selection = tvSelection(input);
  if ((field !== 'label' && field !== 'identifier') || typeof label !== 'string' || !label.length || label.length > 256 || /[\x00-\x1f\x7f]/.test(label) ||
      (direction !== 'up' && direction !== 'down' && direction !== 'left' && direction !== 'right')) throw Error('invalid TV navigation');
  const signal = AbortSignal.timeout(15000);
  for (let moves = 0; moves < 32; moves++) {
    const snapshot = await client.capture.snapshot({ ...selection, signal });
    if (!snapshot || !Array.isArray(snapshot.nodes) || snapshot.nodes.length > 10000 || snapshot.truncated === true) throw Error('invalid TV snapshot');
    const matches = snapshot.nodes.filter(n => n && n[field] === label);
    if (matches.length > 1) throw Error('invalid TV snapshot');
    if (matches[0]?.focused === true) {
      await client.command.tvRemote({ ...selection, button: 'select', signal });
      return;
    }
    await client.command.tvRemote({ ...selection, button: direction, signal });
  }
  throw Error('TV focus not reached');
}

/** @param {TVClient} client @param {unknown} input @param {unknown} port */
export async function enterServerAddress(client, input, port) {
  const selection = tvSelection(input);
  if (port !== '18769') throw Error('invalid TV server port');
  const signal = AbortSignal.timeout(15000);
  const facts = {substage:'capture',candidateCount:/** @type {number|null} */ (null),focusedPropertyPresent:/** @type {boolean|null} */ (null),inheritedLabel:/** @type {boolean|null} */ (null)};
  try {
  for (let moves = 0; moves < 32; moves++) {
    Object.assign(facts,{substage:'capture',candidateCount:null,focusedPropertyPresent:null,inheritedLabel:null});
    const snapshot = await client.capture.snapshot({ ...selection, signal });
    facts.substage='snapshot_validation';
    if (!snapshot || !Array.isArray(snapshot.nodes) || snapshot.nodes.length > 10000 || snapshot.truncated === true) throw Error('invalid TV snapshot');
    facts.substage='candidate_match';
    const fields = snapshot.nodes.filter(n => n && (n.label === 'Server address' || n.contentDescription === 'Server address') &&
      (n.editable === true || /(?:TextField|EditText)$/.test(n.type ?? '')));
    try {Object.assign(facts,{candidateCount:fields.length,focusedPropertyPresent:fields.some(n=>Object.getOwnPropertyDescriptor(n,'focused')!==undefined),
      inheritedLabel:snapshot.nodes.some(n=>n && Object.getOwnPropertyDescriptor(n,'inheritsLabel')?.value===true)});} catch {}
    if (fields.length > 1) throw Error('ambiguous TV address field');
    facts.substage='focus';
    if (fields[0]?.focused === true) {
      facts.substage='select';
      await client.command.tvRemote({ ...selection, button: 'select', signal });
      // Fresh reinstall supplies an empty public field. Focused typing uses no coordinate tap.
      facts.substage='type';
      await client.interactions.type({ ...selection, text: `http://${selection.platform === 'ios' ? '127.0.0.1' : '10.0.2.2'}:${port}`, signal });
      facts.substage='menu';
      await client.command.tvRemote({ ...selection, button: selection.platform === 'ios' ? 'menu' : 'back', signal });
      return;
    }
    await client.command.tvRemote({ ...selection, button: 'up', signal });
  }
  throw Error('TV address focus not reached');
  } catch(error) {
    if(error && typeof error==='object')addressFailures.set(error,Object.freeze({...facts,...(facts.substage==='capture'?{sdkCode:captureCode(error)}:{})}));
    throw error;
  }
}
