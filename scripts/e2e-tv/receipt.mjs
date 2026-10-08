import { validateTvActions } from './actions.mjs';
import { join, isAbsolute } from 'node:path';
// Accept only facts and command templates the hosted producer actually writes.
const fields = ['revision','command','commands','platform','profile','target','device','environment','data','witness','result','cleanup','elapsedSeconds','packages','boundaries'];
function object(value, allowed, required = allowed) {
  if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).some(k => !allowed.includes(k)) || required.some(k => !Object.hasOwn(value,k))) throw new Error('invalid private record fields');
}
function integer(value, min, max) { if (!Number.isInteger(value) || value < min || value > max) throw new Error('invalid receipt integer'); }
function device(value, platform) {
  if (value !== null && (typeof value !== 'string' || !(platform === 'ios' ? /^[A-Fa-f0-9]{8}(?:-[A-Fa-f0-9]{4}){3}-[A-Fa-f0-9]{12}$/ : /^emulator-\d{4}$/).test(value))) throw new Error('invalid receipt device');
  if (platform === 'android' && value !== null && value !== 'emulator-5554') throw new Error('invalid emulator serial');
}
function ownedRecord(value, root, complete) {
  object(value,['run','rootDev','rootIno','platform','profile','target','device','emulatorPID','emulatorStart','complete','creationPending'],['run','rootDev','rootIno','platform','profile','target','device','emulatorPID','emulatorStart','complete']);
  if (typeof value.run !== 'string' || !/^\d{1,20}-\d{1,5}$/.test(value.run) || !['ios','android'].includes(value.platform) || value.target !== 'tv' || value.profile !== (value.platform === 'ios' ? 'tvos' : 'androidtv') || value.complete !== complete || value.rootDev !== root.dev || value.rootIno !== root.ino) throw new Error('cleanup owner mismatch');
  device(value.device,value.platform);
  if (value.emulatorPID === null) {
    if (value.emulatorStart !== null) throw new Error('conflicting emulator identity');
  } else {
    integer(value.emulatorPID,2,2 ** 31-1);
    if (value.platform !== 'android' || value.device === null || typeof value.emulatorStart !== 'string' || !/^\d{1,30}$/.test(value.emulatorStart)) throw new Error('conflicting emulator identity');
  }
  return value;
}
export function validateOwned(value, root) {
  ownedRecord(value, root, true);
  if (value.creationPending !== undefined && value.creationPending !== null) throw new Error('simulator creation unresolved');
  return value;
}
export function validateActiveOwned(value, root) {
  ownedRecord(value, root, false);
  if (value.device === null) throw new Error('active device missing');
  if (value.platform === 'ios') {
    const pending = value.creationPending;
    object(pending,['name','runtime','type','inventoryZero','inventorySHA256']);
    if (pending.name !== `Kinosail-TV-E2E-${value.run}` || pending.type !== 'com.apple.CoreSimulator.SimDeviceType.Apple-TV-4K-3rd-generation-4K' || pending.inventoryZero !== true || typeof pending.runtime !== 'string' || !/^com\.apple\.CoreSimulator\.SimRuntime\.tvOS-27-\d{1,2}(?:-\d{1,2})?$/.test(pending.runtime) || typeof pending.inventorySHA256 !== 'string' || !/^[a-f0-9]{64}$/.test(pending.inventorySHA256)) throw new Error('invalid active simulator witness');
  } else if (value.emulatorPID === null || (value.creationPending !== undefined && value.creationPending !== null)) throw new Error('invalid active emulator witness');
  return value;
}
export function validateReceipt(value, owned, root) {
  object(value,[...fields,'journey','tvActions'],fields);
  if(value.tvActions!==undefined) validateTvActions(value.tvActions);
  if (typeof value.revision !== 'string' || !/^[a-f0-9]{40}$/.test(value.revision) || value.platform !== owned.platform || value.profile !== owned.profile || value.target !== 'tv' || value.device !== owned.device || value.command !== `python3 hosted.py ${owned.profile}` || value.environment !== 'disposable hosted simulator/emulator') throw new Error('conflicting receipt identity');
  device(value.device,value.platform);
  if (value.data !== 'synthetic Example Movie (testsrc2/AAC); synthetic MFA Owner; actual dynamic pairing code' || typeof root !== 'string' || !isAbsolute(root)) throw new Error('invalid fixture facts');
  integer(value.result,-128,255);
  if (!Number.isFinite(value.elapsedSeconds) || value.elapsedSeconds < 0 || value.elapsedSeconds > 86400) throw new Error('invalid receipt elapsed time');
  if (!Array.isArray(value.commands) || value.commands.length > 128) throw new Error('invalid command count');
  const templates = [
    ['go','build','-o',join(root,'bin/player'),'./apps/player/cmd/kinosail'],
    ['node','node_modules/e2e/dist/cli/bin.js','run'],
    ['node','node_modules/agent-device/bin/agent-device.mjs','daemon','stop','--state-dir',join(root,'agent-device')],
    ...(owned.platform === 'ios' ? [
      ['./scripts/build-apple.sh','tvos'],
      ...(owned.device === null ? [] : [
        ['xcrun','simctl','boot',owned.device], ['xcrun','simctl','bootstatus',owned.device,'-b'],
        ['xcrun','simctl','shutdown',owned.device], ['xcrun','simctl','delete',owned.device]
      ])
    ] : [
      ['./gradlew',':app:assembleDebug'],
      ['avdmanager','create','avd','--name',`Kinosail-TV-E2E-${owned.run}`,'--package','system-images;android-36;android-tv;x86_64','--device','tv_1080p']
    ])
  ];
  for (const command of value.commands) {
    if (!Array.isArray(command) || !templates.some(template => command.length === template.length && command.every((arg,i) => typeof arg === 'string' && arg === template[i]))) throw new Error('invalid receipt command');
  }
  object(value.witness,['sourceManifestSHA256','serverSHA256','appSHA256','runtime','nativeTool','imageMetadataSHA256','nativeToolProbe'],[]);
  for (const [key,entry] of Object.entries(value.witness)) {
    if (key.endsWith('SHA256')) {
      if (typeof entry !== 'string' || !/^[a-f0-9]{64}$/.test(entry)) throw new Error('invalid build hash');
    } else if (key === 'runtime') {
      if (typeof entry !== 'string' || !(owned.platform === 'ios' ? /^com\.apple\.CoreSimulator\.SimRuntime\.tvOS-27-\d{1,2}(?:-\d{1,2})?$/.test(entry) : entry === 'system-images;android-36;android-tv;x86_64')) throw new Error('invalid runtime witness');
    } else if (key === 'nativeToolProbe') {
      object(entry,['tool','outcome','exitCode','stdoutBytes','stderrBytes','stdoutTokensRecognized','combinedTokensRecognized','outputTruncated']);
      if (entry.tool !== (owned.platform === 'ios' ? 'Xcode' : 'Java') || !['started','unavailable','timeout','overflow','nonzero','rejected','valid'].includes(entry.outcome)) throw new Error('invalid native probe stage');
      if (entry.exitCode !== null) integer(entry.exitCode,-128,255);
      integer(entry.stdoutBytes,0,8193); integer(entry.stderrBytes,0,8193);
      if (['stdoutTokensRecognized','combinedTokensRecognized','outputTruncated'].some(field => typeof entry[field] !== 'boolean')) throw new Error('invalid native probe facts');
      if ((!entry.outputTruncated && entry.stdoutBytes + entry.stderrBytes > 8192) || entry.outputTruncated !== (entry.outcome === 'overflow') || (entry.outcome === 'valid' && (entry.exitCode !== 0 || !entry.combinedTokensRecognized)) || (entry.outcome === 'nonzero' && (entry.exitCode === null || entry.exitCode === 0))) throw new Error('conflicting native probe facts');
    } else {
      object(entry,['name','version','build']);
      if (owned.platform === 'ios') {
        if (entry.name !== 'Xcode' || typeof entry.version !== 'string' || !/^27(?:\.\d{1,2}){0,2}$/.test(entry.version) || typeof entry.build !== 'string' || !/^\d{2}[A-Z]\d{1,7}[a-z]?$/.test(entry.build)) throw new Error('invalid Xcode witness');
      } else if (entry.name !== 'Java' || typeof entry.version !== 'string' || !/^17(?:\.\d{1,3}){1,3}(?:-ea)?$/.test(entry.version) || typeof entry.build !== 'string' || !/^17(?:\.\d{1,3}){1,3}(?:-ea)?\+\d{1,5}(?:-LTS)?$/.test(entry.build) || entry.build.split('+')[0] !== entry.version) throw new Error('invalid Java witness');
    }
  }
  if (!Array.isArray(value.cleanup) || value.cleanup.length > 32) throw new Error('invalid cleanup count');
  for (const entry of value.cleanup) {
    object(entry,['resource','action','result'],['resource','result']); integer(entry.result,-128,255);
    if (entry.resource === 'owned agent-device daemon') {
      if (entry.action !== undefined) throw new Error('invalid daemon cleanup');
    } else if (owned.platform === 'ios' && owned.device !== null && entry.resource === `owned simulator ${owned.device}`) {
      if (!['shutdown','delete'].includes(entry.action)) throw new Error('invalid simulator cleanup');
    } else if (owned.platform === 'android' && owned.device !== null && entry.resource === 'owned emulator process') {
      if (entry.action !== undefined) throw new Error('invalid emulator cleanup');
    } else throw new Error('invalid cleanup resource');
  }
  object(value.packages,['e2e','agent-device']);
  if (value.packages.e2e !== '0.17.0' || value.packages['agent-device'] !== '0.21.22') throw new Error('invalid pinned receipt packages');
  if (!Array.isArray(value.boundaries) || JSON.stringify(value.boundaries) !== JSON.stringify(['no physical device','no phone/watchOS/Wear pairing','no casting/provider calls','no deployed server'])) throw new Error('invalid receipt boundaries');
  if (value.journey !== undefined) {
    object(value.journey,['approval','decodedFrames','partialProgressSeconds','watched','connectionRestored','progressPersisted','remoteFocus','menuReturned','tvForeground']);
    if (value.journey.approval !== 'public Owner API and actual dynamic UI code') throw new Error('invalid approval witness');
    if (value.journey.remoteFocus !== true || value.journey.menuReturned !== true || value.journey.tvForeground !== true || value.journey.decodedFrames !== true || value.journey.watched !== false || value.journey.connectionRestored !== true || value.journey.progressPersisted !== true || !Number.isFinite(value.journey.partialProgressSeconds) || value.journey.partialProgressSeconds <= 0 || value.journey.partialProgressSeconds >= 16 * .8) throw new Error('invalid journey witness');
  }
  if (value.result === 0 && (value.device === null || value.journey === undefined || ['sourceManifestSHA256','serverSHA256','appSHA256','runtime','nativeTool'].some(key => !Object.hasOwn(value.witness,key)) || !value.commands.some(args => JSON.stringify(args) === JSON.stringify(['node','node_modules/e2e/dist/cli/bin.js','run'])))) throw Error('successful TV proof incomplete');
  return Object.fromEntries([...fields,'journey','tvActions'].filter(key => Object.hasOwn(value,key)).map(key => [key,value[key]]));
}

// JSON.parse alone accepts duplicate object fields; private records reject that ambiguity.
export function parseStrictJSON(input) {
  const text = typeof input === 'string' ? input : new TextDecoder('utf-8',{fatal:true}).decode(input);
  const parsed = JSON.parse(text), stack = [];
  for (let i = 0; i < text.length; i++) {
    const character = text[i];
    if (character === '{' || character === '[') {
      if (stack.length >= 40) throw new Error('private JSON nesting too deep');
      stack.push(character === '{' ? { keys: new Set(), key: true } : null);
    } else if (character === '}' || character === ']') stack.pop();
    else if (character === ',' && stack.at(-1)) stack.at(-1).key = true;
    else if (character === '"') {
      const start = i;
      while (++i < text.length && text[i] !== '"') if (text[i] === '\\') i++;
      const container = stack.at(-1);
      if (container?.key) {
        const key = JSON.parse(text.slice(start, i + 1));
        if (container.keys.has(key)) throw new Error('duplicate private JSON field');
        container.keys.add(key); container.key = false;
      }
    }
  }
  return parsed;
}
