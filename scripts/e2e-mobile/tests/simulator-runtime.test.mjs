import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, copyFileSync, existsSync, rmSync, realpathSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
const name='Kinosail-E2E-12345-1', runtime='com.apple.CoreSimulator.SimRuntime.iOS-27-1', type='com.apple.CoreSimulator.SimDeviceType.iPhone-17-Pro';
const ownedUUID='12345678-ABCD-4321-9876-123456789ABC', foreignUUID='87654321-ABCD-4321-9876-123456789ABC';
function fixture(t,mode) {
 const cwd=realpathSync(mkdtempSync(join(tmpdir(),'kino-simulator-contract-')));t.after(()=>rmSync(cwd,{recursive:true,force:true}));
 const project=join(cwd,'scripts/e2e-mobile'),bin=join(cwd,'stub-bin');mkdirSync(project,{recursive:true});mkdirSync(bin);
 for(const file of ['hosted.py','cleanup.py','simulator.py'])if(existsSync(resolve(file)))copyFileSync(file==='simulator.py'&&process.env.KINOSAIL_SIMULATOR_CONTRACT_SOURCE?process.env.KINOSAIL_SIMULATOR_CONTRACT_SOURCE:resolve(file),join(project,file));
 const shared=join(cwd,'scripts/e2e');mkdirSync(shared);copyFileSync(resolve('../e2e/native_tool.py'),join(shared,'native_tool.py'));
 const state=join(cwd,'state.json'),calls=join(cwd,'calls.jsonl');
 const foreign={name:'PERSONAL-PRESERVE',udid:foreignUUID,deviceTypeIdentifier:type};
 writeFileSync(state,JSON.stringify({devices:{[runtime]:[foreign]}}));
 const stub=`#!/usr/bin/env python3
import json,os,sys,signal
from pathlib import Path
args=sys.argv[1:];tool=Path(sys.argv[0]).name
state=Path(${JSON.stringify(state)});calls=Path(${JSON.stringify(calls)})
with calls.open('a') as log:log.write(json.dumps([tool,*args])+'\\n')
if tool=='git':
 if args[:2]==['rev-parse','HEAD']:print('a'*40)
 sys.exit(0)
if tool=='go':Path(args[args.index('-o')+1]).write_bytes(b'SYNTHETIC SERVER');sys.exit(0)
if tool=='xcrun':
 data=json.loads(state.read_text())
 if args[1:3]==['list','runtimes']:print(json.dumps({'runtimes':[{'isAvailable':True,'identifier':${JSON.stringify(runtime)},'version':'27.1'}]}))
 elif args[1:3]==['list','devices']:print(state.read_text())
 elif args[1]=='create':
  data['devices'][${JSON.stringify(runtime)}].append({'name':args[2],'udid':${JSON.stringify(ownedUUID)},'deviceTypeIdentifier':args[3]});state.write_text(json.dumps(data))
  if ${JSON.stringify(mode)}=='cancel':os.kill(os.getppid(),signal.SIGTERM)
  print('MALFORMED-CREATE-OUTPUT')
 elif args[1]=='delete':
  for devices in data['devices'].values():devices[:]=[d for d in devices if d['udid']!=args[2]]
  state.write_text(json.dumps(data))
 sys.exit(0)
raise SystemExit('UNEXPECTED STUB COMMAND')
`;
 for(const tool of ['git','go','xcrun'])writeFileSync(join(bin,tool),stub,{mode:0o700});
 const native=join(cwd,'apps/player/apps/native');mkdirSync(join(native,'scripts'),{recursive:true});
 writeFileSync(join(native,'scripts/build-apple.sh'),'#!/bin/sh\nmkdir -p .build/ios-simulator/Build/Products/Debug-iphonesimulator/KinosailPlayer.app\nprintf SYNTHETIC-APP > .build/ios-simulator/Build/Products/Debug-iphonesimulator/KinosailPlayer.app/KinosailPlayer\n',{mode:0o700});
 const env={PATH:bin+':'+process.env.PATH,GITHUB_ACTIONS:'true',GITHUB_RUN_ID:'12345',GITHUB_RUN_ATTEMPT:'1',RUNNER_OS:'macOS'};
 return {cwd,project,state,calls,env,foreign};
}
function invoke(f,file,args=[]){return spawnSync('python3',[join(f.project,file),...args],{cwd:f.project,env:f.env,encoding:'utf8',timeout:10000});}
function inventory(f){return JSON.parse(readFileSync(f.state,'utf8')).devices[runtime];}
for(const mode of ['malformed','cancel'])test(`actual hosted process recovers a simulator after ${mode} create output without touching foreign devices`,t=>{
 const f=fixture(t,mode);const result=invoke(f,'hosted.py',['ios']);assert.notEqual(result.status,0);
 assert.deepEqual(inventory(f),[f.foreign]);
 const owner=JSON.parse(readFileSync(join(f.project,'.e2e/owned-device.json'),'utf8'));assert.equal(owner.complete,true);assert.equal(owner.creationPending,null);
 const calls=readFileSync(f.calls,'utf8');assert.ok(calls.includes('"delete","'+ownedUUID+'"')||calls.includes('"delete", "'+ownedUUID+'"'));assert.equal(calls.includes('"delete", "'+foreignUUID+'"'),false);
});
for(const mode of ['unique','ambiguous','wrong-type','wrong-runtime'])test(`fallback ${mode} pending creation recovery preserves foreign simulators`,t=>{
 const f=fixture(t,mode),root=join(f.project,'.e2e');mkdirSync(root,{mode:0o700});
 const {dev,ino}=awaitStat(root);
 const owner={run:'12345-1',rootDev:dev,rootIno:ino,platform:'ios',device:null,emulatorPID:null,emulatorStart:null,complete:false,creationPending:{name,runtime,type,inventoryZero:true,inventorySHA256:'a'.repeat(64)}};
 writeFileSync(join(root,'owned-device.json'),JSON.stringify(owner),{mode:0o600});
 const data=JSON.parse(readFileSync(f.state,'utf8'));const entry={name,udid:ownedUUID,deviceTypeIdentifier:mode==='wrong-type'?'com.apple.CoreSimulator.SimDeviceType.iPhone-16-Pro':type};
 if(mode==='wrong-runtime')data.devices['com.apple.CoreSimulator.SimRuntime.iOS-27-0']=[entry];else data.devices[runtime].push(entry);
 if(mode==='ambiguous')data.devices[runtime].push({...entry,udid:'11111111-ABCD-4321-9876-123456789ABC'});
 writeFileSync(f.state,JSON.stringify(data));const before=readFileSync(f.state);
 const result=invoke(f,'cleanup.py');
 if(mode==='unique'){assert.equal(result.status,0,result.stderr);assert.deepEqual(inventory(f),[f.foreign]);assert.equal(JSON.parse(readFileSync(join(root,'owned-device.json'),'utf8')).complete,true);}
 else {assert.notEqual(result.status,0);assert.deepEqual(readFileSync(f.state),before);assert.equal(JSON.parse(readFileSync(join(root,'owned-device.json'),'utf8')).complete,false);}
});
import { statSync as awaitStat } from 'node:fs';
test('pre-create name collision rejects before creation and preserves the foreign simulator',t=>{
 const f=fixture(t,'collision');const data=JSON.parse(readFileSync(f.state,'utf8'));data.devices[runtime][0].name=name;writeFileSync(f.state,JSON.stringify(data));const before=readFileSync(f.state);
 const result=invoke(f,'hosted.py',['ios']);assert.notEqual(result.status,0);assert.deepEqual(readFileSync(f.state),before);assert.equal(readFileSync(f.calls,'utf8').includes('"create"'),false);assert.equal(readFileSync(f.calls,'utf8').includes('"delete"'),false);
});
for(const scenario of ['complete-with-pending','missing-preinventory-witness','malformed-inventory','owner-duplicate-field','inventory-duplicate-field'])test(`fallback rejects ${scenario} before touching any simulator`,t=>{
 const f=fixture(t,scenario),root=join(f.project,'.e2e');mkdirSync(root,{mode:0o700});const {dev,ino}=awaitStat(root);
 const owner={run:'12345-1',rootDev:dev,rootIno:ino,platform:'ios',device:null,emulatorPID:null,emulatorStart:null,complete:scenario==='complete-with-pending',creationPending:{name,runtime,type,inventoryZero:true,inventorySHA256:'a'.repeat(64)}};
 if(scenario==='missing-preinventory-witness')delete owner.creationPending.inventorySHA256;
 writeFileSync(join(root,'owned-device.json'),JSON.stringify(owner),{mode:0o600});
 if(scenario==='owner-duplicate-field')writeFileSync(join(root,'owned-device.json'),'{"complete":true,' + JSON.stringify(owner).slice(1));
 if(scenario==='inventory-duplicate-field')writeFileSync(f.state,'{"devices":{},' + readFileSync(f.state,'utf8').slice(1));
 if(scenario==='malformed-inventory')writeFileSync(f.state,'MALFORMED-PRIVATE-INVENTORY');const before=readFileSync(f.state),ownerBefore=readFileSync(join(root,'owned-device.json'));
 assert.notEqual(invoke(f,'cleanup.py').status,0);assert.deepEqual(readFileSync(f.state),before);assert.deepEqual(readFileSync(join(root,'owned-device.json')),ownerBefore);assert.equal(existsSync(f.calls)&&readFileSync(f.calls,'utf8').includes('"delete"'),false);
});
