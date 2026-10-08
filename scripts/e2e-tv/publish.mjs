import { lstatSync, readdirSync, writeFileSync, rmSync, renameSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { createHash } from 'node:crypto';
import { sanitize, privateFile } from '../e2e-mobile/privacy.mjs';
import { validateOwned, validateReceipt, parseStrictJSON } from './receipt.mjs';
// Validate every private input and cleanup path before any staging, removal or publication.
try {
  const root = '.e2e', stat = lstatSync(root);
  if (!stat.isDirectory() || stat.isSymbolicLink() || stat.uid !== process.getuid() || (stat.mode & 0o777) !== 0o700) throw new Error('unsafe output root');
  const owned = validateOwned(parseStrictJSON(privateFile(join(root, 'owned-device.json'), 4096, true)), stat);
  const privateReceipt = parseStrictJSON(privateFile(join(root, 'receipt.private.json'), 65536, true));
  try { privateReceipt.tvActions = parseStrictJSON(privateFile(join(root,'tv-actions.private.json'),65536,true)); }
  catch(error) { if(error.code!=='ENOENT')throw error; }
  const receipt = validateReceipt(privateReceipt, owned, resolve(root));
  if (receipt.result === 0 && parseStrictJSON(privateFile(join(root,'secrets.json'),65536,true)).stage !== 'complete') throw Error('TV privacy stage incomplete');
  const names = ['sdk','agent-device','avd','bin','fixtures','secrets.json','control.json','journey.json','tv-actions.private.json','receipt.private.json','process-owned.json'];
  const removal = [];
  let count = 0;
  function inspect(path, depth = 0) {
    const entry = lstatSync(path);
    if (++count > 16384 || depth > 20 || entry.uid !== process.getuid() || entry.isSymbolicLink() || (!entry.isDirectory() && entry.nlink !== 1) || (!entry.isDirectory() && !entry.isFile() && !entry.isSocket())) throw new Error('unsafe raw cleanup tree');
    if (entry.isDirectory()) for (const name of readdirSync(path)) inspect(join(path,name),depth + 1);
    return entry;
  }
  for (const name of names) {
    const path = join(root,name);
    try { removal.push({path,identity:inspect(path)}); } catch (error) { if (error.code !== 'ENOENT') throw error; }
  }
  // Only redacted receipt values are retained. The upload directory appears after raw cleanup.
  const safe = sanitize(root,receipt,{stageOnly:true});
  for (const {path,identity} of removal) {
    const current = lstatSync(path);
    if (current.dev !== identity.dev || current.ino !== identity.ino) throw new Error('cleanup ownership changed');
    rmSync(path,{recursive:true,force:false});
  }
  safe.cleanup.push({resource:'owned raw SDK, secrets, fixture receipts, binaries, daemon state and AVD paths',result:0});
  const staged = join(root,'sanitizing');
  writeFileSync(join(staged,'receipt.json'),JSON.stringify(safe,null,2),{mode:0o600});
  writeFileSync(join(staged,'SHA256SUMS'),readdirSync(staged).filter(n => n !== 'SHA256SUMS').sort().map(n => `${createHash('sha256').update(privateFile(join(staged,n),2 * 1024 * 1024,true)).digest('hex')}  ${n}`).join('\n') + '\n',{mode:0o600});
  renameSync(staged,join(root,'published'));
  console.log('Sanitized TV native evidence: .e2e/published');
} catch {
  console.error('Privacy or cleanup boundary rejected output; publication withheld');
  process.exitCode = 1;
}
