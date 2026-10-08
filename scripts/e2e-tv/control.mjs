import { lstatSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { privateFile } from '../e2e-mobile/privacy.mjs';
import { parseStrictJSON, validateActiveOwned } from './receipt.mjs';
import { validateRun } from '../e2e-mobile/contracts.mjs';
export function readControl(directory = process.cwd()) {
  const project = resolve(directory), root = join(project, '.e2e'), stat = lstatSync(root);
  if (!stat.isDirectory() || stat.isSymbolicLink() || stat.uid !== process.getuid() || (stat.mode & 0o777) !== 0o700) throw new Error('unsafe active output root');
  const control = parseStrictJSON(privateFile(join(root, 'control.json'), 4096, true));
  if (!control || typeof control !== 'object' || Array.isArray(control) || Object.keys(control).sort().join(',') !== 'appPath,identity') throw new Error('invalid control envelope');
  const raw = control.identity;
  if (!raw || Object.keys(raw).sort().join(',') !== 'device,platform,port,profile,revision,run,target' || raw.target !== 'tv' || raw.profile !== (raw.platform === 'ios' ? 'tvos' : 'androidtv')) throw Error('invalid TV control identity');
  const { profile, target, ...base } = raw;
  const identity = { ...validateRun(base), profile, target };
  const owned = validateActiveOwned(parseStrictJSON(privateFile(join(root, 'owned-device.json'), 4096, true)), stat);
  if (identity.run !== owned.run || identity.platform !== owned.platform || identity.profile !== owned.profile || owned.target !== 'tv' || identity.device !== owned.device || identity.port !== '18769') throw new Error('active control owner mismatch');
  const appPath = resolve(project, '../..', identity.platform === 'ios'
    ? 'apps/player/apps/native/.build/tvos-simulator/Build/Products/Debug-appletvsimulator/KinosailPlayer.app'
    : 'apps/player/apps/android/app/build/outputs/apk/debug/app-debug.apk');
  if (control.appPath !== appPath) throw new Error('foreign native app path');
  return control;
}
