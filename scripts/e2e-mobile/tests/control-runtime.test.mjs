import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, readdirSync, copyFileSync, statSync, chmodSync, rmSync, symlinkSync, linkSync, existsSync, realpathSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import childProcess from 'node:child_process';
const source = resolve(process.env.KINOSAIL_CONTROL_CONTRACT_SOURCE ?? resolve(import.meta.dirname, '..'));
const uuid = '12345678-1234-1234-1234-123456789abc';
const childHarness = String.raw`
import {appendFileSync, writeFileSync, readFileSync} from 'node:fs';
const [entry,operation] = process.argv.slice(1);
if (!['./e2e.config.ts','./tests/phone.e2e.ts'].includes(entry) ||
    !['import','replace-control','replace-owner','replace-device-pair'].includes(operation) ||
    (entry !== './tests/phone.e2e.ts' && operation !== 'import')) throw Error('invalid child selection');
globalThis.fetch=()=>{appendFileSync('calls.txt','http\n');throw Error('unexpected HTTP');};
if (entry === './e2e.config.ts') await import('./e2e.config.ts');
else await import('./tests/phone.e2e.ts');
if (operation !== 'import') {
  if (operation === 'replace-device-pair') {
    for (const name of ['control.json','owned-device.json']) {
      const path='.e2e/'+name,value=JSON.parse(readFileSync(path));
      if (name === 'control.json') value.identity.device='aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
      else value.device='aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
      writeFileSync(path,JSON.stringify(value));
    }
  } else {
    const path=operation === 'replace-control' ? '.e2e/control.json' : '.e2e/owned-device.json';
    const value=JSON.parse(readFileSync(path));
    if (operation === 'replace-control') value.appPath='foreign';
    else value.device='foreign';
    writeFileSync(path,JSON.stringify(value));
  }
  await globalThis.phoneCallback({platform:'ios',app:{},
    device:{installApp(){appendFileSync('calls.txt','install\n');}},screen:{}});
}
`;
function fixture(platform = 'ios') {
  const base = realpathSync(mkdtempSync(join(tmpdir(),'kino-control-contract-'))), project = join(base,'scripts/e2e-mobile'), root = join(project,'.e2e');
  mkdirSync(root,{recursive:true,mode:0o700}); mkdirSync(join(project,'tests'));
  for (const name of readdirSync(source).filter(name => name.endsWith('.mjs'))) copyFileSync(join(source,name),join(project,name));
  mkdirSync(join(base,'scripts/e2e'));
  for (const name of ['fixture-response.mjs','fixture-setup.mjs']) copyFileSync(join(source,'../e2e',name),join(base,'scripts/e2e',name));
  copyFileSync(join(source,'e2e.config.ts'),join(project,'e2e.config.ts'));
  copyFileSync(join(source,'tests/phone.e2e.ts'),join(project,'tests/phone.e2e.ts'));
  writeFileSync(join(project,'package.json'),'{"type":"module"}');
  const identity = {platform,device:platform === 'ios' ? uuid : 'emulator-5554',port:'18769',revision:'a'.repeat(40),run:'12345-1'};
  const appPath = join(base,platform === 'ios' ? 'apps/player/apps/native/.build/ios-simulator/Build/Products/Debug-iphonesimulator/KinosailPlayer.app' : 'apps/player/apps/android/app/build/outputs/apk/debug/app-debug.apk');
  const control = {identity,appPath}, stat = statSync(root);
  const owned = {run:identity.run,rootDev:stat.dev,rootIno:stat.ino,platform,device:identity.device,emulatorPID:platform === 'android' ? 1234 : null,emulatorStart:platform === 'android' ? '12345678' : null,complete:false};
  if (platform === 'ios') owned.creationPending = {name:'Kinosail-E2E-12345-1',runtime:'com.apple.CoreSimulator.SimRuntime.iOS-27-1',type:'com.apple.CoreSimulator.SimDeviceType.iPhone-17-Pro',inventoryZero:true,inventorySHA256:'b'.repeat(64)};
  const save = () => {writeFileSync(join(root,'control.json'),JSON.stringify(control),{mode:0o600});writeFileSync(join(root,'owned-device.json'),JSON.stringify(owned),{mode:0o600});}; save();
  for (const [name,body] of Object.entries({
    '@e2e-dev/mobile':`import{appendFileSync}from'node:fs';const mark=(name)=>appendFileSync('calls.txt',name+'\\n');export function mobile(options){mark('builder');return{options}};export function test(name,callback){mark('registration');globalThis.phoneCallback=callback}`,
    'e2e':`export const expect = {};`, 'pngjs':`export const PNG = {};`
  })) {const dir=join(project,'node_modules',name);mkdirSync(dir,{recursive:true});writeFileSync(join(dir,'package.json'),'{"type":"module","exports":"./index.mjs"}');writeFileSync(join(dir,'index.mjs'),body);}
  const execute = (entry, operation = 'import') => {
    if (!['./e2e.config.ts','./tests/phone.e2e.ts'].includes(entry) ||
        !['import','replace-control','replace-owner','replace-device-pair'].includes(operation) ||
        (entry !== './tests/phone.e2e.ts' && operation !== 'import')) throw Error('invalid child selection');
    return childProcess.spawnSync(process.execPath,['--input-type=module','-e',childHarness,entry,operation],
      {cwd:project,encoding:'utf8',timeout:10000});
  };
  return {base,project,root,control,owned,save,execute,dispose:()=>rmSync(base,{recursive:true,force:true})};
}
const cases = {
  'missing control':f=>rmSync(join(f.root,'control.json')),
  'oversized control':f=>writeFileSync(join(f.root,'control.json'),' '.repeat(2*1024*1024)+JSON.stringify(f.control)),
  'linked control':f=>{const p=join(f.root,'control.json');copyFileSync(p,join(f.base,'foreign'));rmSync(p);symlinkSync(join(f.base,'foreign'),p)},
  'hardlinked control':f=>linkSync(join(f.root,'control.json'),join(f.base,'foreign')),
  'public control':f=>chmodSync(join(f.root,'control.json'),0o644),
  'invalid UTF8':f=>writeFileSync(join(f.root,'control.json'),Buffer.concat([Buffer.from(JSON.stringify(f.control)),Buffer.from([0xff])])),
  'duplicate envelope':f=>writeFileSync(join(f.root,'control.json'),JSON.stringify(f.control).replace('"identity":','"identity":{},"identity":')),
  'unknown envelope':f=>{f.control.extra='UNREGISTERED-SENTINEL';f.save()},
  'missing envelope':f=>{delete f.control.appPath;f.save()},
  'array envelope':f=>writeFileSync(join(f.root,'control.json'),'[]'),
  'unknown identity':f=>{f.control.identity.extra='bad';f.save()},
  'invalid platform':f=>{f.control.identity.platform='tvos';f.save()},
  'invalid UUID':f=>{f.control.identity.device='foreign';f.save()},
  'invalid revision':f=>{f.control.identity.revision='bad';f.save()},
  'invalid run':f=>{f.control.identity.run='bad';f.save()},
  'foreign port':f=>{f.control.identity.port='18770';f.save()},
  'foreign app':f=>{f.control.appPath='/tmp/foreign-app.app';f.save()},
  'normalized foreign app':f=>{f.control.appPath=f.control.appPath.replace('/apps/','/else/../apps/');f.save()},
  'missing owner':f=>rmSync(join(f.root,'owned-device.json')),
  'oversized owner':f=>writeFileSync(join(f.root,'owned-device.json'),' '.repeat(8192)+JSON.stringify(f.owned)),
  'linked owner':f=>{const p=join(f.root,'owned-device.json');copyFileSync(p,join(f.base,'foreign'));rmSync(p);symlinkSync(join(f.base,'foreign'),p)},
  'hardlinked owner':f=>linkSync(join(f.root,'owned-device.json'),join(f.base,'foreign')),
  'public owner':f=>chmodSync(join(f.root,'owned-device.json'),0o644),
  'unknown owner':f=>{f.owned.extra='bad';f.save()},
  'duplicate owner':f=>writeFileSync(join(f.root,'owned-device.json'),JSON.stringify(f.owned).replace('"run":','"run":"999-1","run":')),
  'mismatched run':f=>{f.owned.run='999-1';f.save()},
  'mismatched device':f=>{f.owned.device='aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';f.save()},
  'mismatched platform':f=>{f.owned.platform='android';f.save()},
  'mismatched inode':f=>{f.owned.rootIno++;f.save()},
  'mismatched filesystem':f=>{f.owned.rootDev++;f.save()},
  'completed owner':f=>{f.owned.complete=true;f.save()},
  'missing device':f=>{f.owned.device=null;f.save()},
  'foreign pending name':f=>{f.owned.creationPending.name='Foreign';f.save()},
  'unproven inventory':f=>{f.owned.creationPending.inventoryZero=false;f.save()},
  'missing pending':f=>{delete f.owned.creationPending;f.save()},
  'conflicting iOS process':f=>{f.owned.emulatorPID=1234;f.owned.emulatorStart='123';f.save()},
  'unknown pending':f=>{f.owned.creationPending.extra='bad';f.save()},
  'invalid pending runtime':f=>{f.owned.creationPending.runtime='foreign';f.save()},
  'invalid pending type':f=>{f.owned.creationPending.type='foreign';f.save()},
  'invalid pending hash':f=>{f.owned.creationPending.inventorySHA256='bad';f.save()},
  'public root':f=>chmodSync(f.root,0o755),
  'linked root':f=>{const p=join(f.base,'foreign-root');importRename(f.root,p);symlinkSync(p,f.root)}
};
// Sync rename is used only on this contract's exclusively owned temporary tree.
import { renameSync as importRename } from 'node:fs';
function bytes(f) {const result={};for(const path of [join(f.root,'control.json'),join(f.root,'owned-device.json'),join(f.base,'foreign')]) if(existsSync(path)) result[path]=readFileSync(path).toString('base64');return result}
for(const [name,mutate]of Object.entries(cases))for(const entry of ['./e2e.config.ts','./tests/phone.e2e.ts'])test(`admission rejects ${name} before ${entry} effects`,()=>{
  const f=fixture();try{mutate(f);const before=bytes(f),children=readdirSync(f.root);const result=f.execute(entry);assert.notEqual(result.status,0,result.stderr);assert.equal(existsSync(join(f.project,'calls.txt')),false,'SDK builder/registration must not run');assert.deepEqual(bytes(f),before);assert.deepEqual(readdirSync(f.root),children)}finally{f.dispose()}
});
for(const platform of ['ios','android'])test(`active ${platform} producer admits exact owned target`,()=>{
 const f=fixture(platform);try{const result=f.execute('./e2e.config.ts');assert.equal(result.status,0,result.stderr);assert.equal(readFileSync(join(f.project,'calls.txt'),'utf8'),'builder\n');rmSync(join(f.project,'calls.txt'));const journey=f.execute('./tests/phone.e2e.ts');assert.equal(journey.status,0,journey.stderr);assert.equal(readFileSync(join(f.project,'calls.txt'),'utf8'),'registration\n')}finally{f.dispose()}
});
for(const [name,mutate]of Object.entries({
 'missing process':f=>{f.owned.emulatorPID=null;f.owned.emulatorStart=null},
 'invalid process':f=>{f.owned.emulatorPID=1},
 'missing process witness':f=>{f.owned.emulatorStart=null},
 'foreign serial':f=>{f.owned.device=f.control.identity.device='emulator-5556'},
 'simulator pending on Android':f=>{f.owned.creationPending={name:'bad'}}
}))test(`Android rejects ${name} before builder`,()=>{const f=fixture('android');try{mutate(f);f.save();const before=bytes(f);const result=f.execute('./e2e.config.ts');assert.notEqual(result.status,0,result.stderr);assert.equal(existsSync(join(f.project,'calls.txt')),false);assert.deepEqual(bytes(f),before)}finally{f.dispose()}});
for(const replacement of ['control','owner'])test(`journey execution rechecks replaced ${replacement} before HTTP/install`,()=>{
 const f=fixture();try{const result=f.execute('./tests/phone.e2e.ts','replace-'+replacement);assert.notEqual(result.status,0,result.stderr);assert.equal(readFileSync(join(f.project,'calls.txt'),'utf8'),'registration\n')}finally{f.dispose()}
});
test('journey pins discovered device when both control and owner change',()=>{
 const f=fixture();try{const result=f.execute('./tests/phone.e2e.ts','replace-device-pair');assert.notEqual(result.status,0,result.stderr);assert.equal(readFileSync(join(f.project,'calls.txt'),'utf8'),'registration\n')}finally{f.dispose()}
});

test('child module and operation reject unknown input before any subprocess or fixture writes',t=>{
 const f=fixture();try{
  const before=bytes(f),children=readdirSync(f.root);let spawned=0;
  t.mock.method(childProcess,'spawnSync',()=>{spawned++;throw Error('subprocess must not start');});
  for(const [entry,operation] of [
   [undefined,'import'],[null,'import'],[false,'import'],['','import'],['x'.repeat(4097),'import'],
   ['../foreign.mjs','import'],['./tests/phone.e2e.ts?extra','import'],['./tests/phone.e2e.ts',''],
   ['./tests/phone.e2e.ts',null],['./tests/phone.e2e.ts',false],
   ['./tests/phone.e2e.ts','x'.repeat(4097)],['./tests/phone.e2e.ts',"writeFileSync('foreign','unsafe')"],
   ['./e2e.config.ts','replace-owner'],['./tests/phone.e2e.ts','unknown']
  ])assert.throws(()=>f.execute(entry,operation),/invalid child selection/);
  assert.equal(spawned,0);assert.deepEqual(bytes(f),before);assert.deepEqual(readdirSync(f.root),children);
  assert.equal(existsSync(join(f.project,'calls.txt')),false);
 }finally{f.dispose()}
});
