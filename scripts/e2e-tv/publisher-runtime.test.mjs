import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, existsSync, statSync, realpathSync, symlinkSync, linkSync, chmodSync, rmSync, readdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { createHash } from 'node:crypto';
const publish = resolve('publish.mjs');
const secret = '18B5000j'; // Synthetic, valid build witness registered as private.
function fixture(t) {
  const cwd = realpathSync(mkdtempSync(join(tmpdir(), 'kino-publisher-contract-')));
  t.after(() => rmSync(cwd, { recursive: true, force: true }));
  const root = join(cwd, '.e2e'); mkdirSync(root, {mode:0o700}); mkdirSync(join(root,'sdk'),{mode:0o700});
  const current = statSync(root);
  const owned = {run:'12345-1',rootDev:current.dev,rootIno:current.ino,platform:'ios',profile:'tvos',target:'tv',device:null,emulatorPID:null,emulatorStart:null,complete:true};
  const receipt = {revision:'a'.repeat(40),command:'python3 hosted.py tvos',commands:[],platform:'ios',profile:'tvos',target:'tv',device:null,environment:'disposable hosted simulator/emulator',data:'synthetic Example Movie (testsrc2/AAC); synthetic MFA Owner; actual dynamic pairing code',witness:{nativeTool:{name:'Xcode',version:'27.1',build:secret}},result:1,cleanup:[],elapsedSeconds:1,packages:{e2e:'0.17.0','agent-device':'0.21.22'},boundaries:['no physical device','no phone/watchOS/Wear pairing','no casting/provider calls','no deployed server']};
  for(const [name,value] of [['owned-device.json',owned],['receipt.private.json',receipt],['secrets.json',{secrets:[secret,'839271'],stage:'started'}]]) writeFileSync(join(root,name),JSON.stringify(value),{mode:0o600});
  writeFileSync(join(root,'sdk/report.json'),JSON.stringify({error:secret,artifacts:[]}),{mode:0o600});
  return {cwd,root,owned,receipt};
}
function run(cwd) { return spawnSync(process.execPath,[publish],{cwd,encoding:'utf8',timeout:10000}); }
test('actual publisher redacts valid receipt fields, retains failure exit result, removes raw data and hashes only safe final bytes', t=>{
  const {cwd,root} = fixture(t);const result=run(cwd);assert.equal(result.status,0,result.stderr);
  const published=join(root,'published');
  for(const name of readdirSync(published)) assert.equal(readFileSync(join(published,name),'utf8').includes(secret),false,name);
  const receipt=JSON.parse(readFileSync(join(published,'receipt.json'),'utf8'));assert.equal(receipt.result,1);assert.equal(receipt.witness.nativeTool.build,'[REDACTED]');
  assert.equal(existsSync(join(root,'sdk')),false);assert.equal(existsSync(join(root,'receipt.private.json')),false);
  for(const line of readFileSync(join(published,'SHA256SUMS'),'utf8').trim().split('\n')) {
    const [hash,name]=line.split('  ');assert.equal(hash,createHash('sha256').update(readFileSync(join(published,name))).digest('hex'));
  }
});
for(const scenario of ['owned-symlink','owned-hardlink','receipt-symlink','receipt-hardlink','owned-loose-mode','receipt-loose-mode','receipt-oversized','receipt-unknown-field','receipt-invalid-result','receipt-platform-conflict','owned-unknown-field','owned-malformed-run','receipt-malformed-revision','receipt-invalid-utf8','owned-duplicate-field','receipt-duplicate-field','receipt-missing-field','receipt-malformed','receipt-missing','owned-missing','cleanup-path-symlink','cleanup-path-hardlink']) {
 test(`publisher rejects ${scenario} before sanitize, deletion or publication; preserves foreign content`,t=>{
  const {cwd,root,owned,receipt}=fixture(t);
  const foreign=join(cwd,'foreign.json');
  writeFileSync(foreign,JSON.stringify(scenario.startsWith('owned')?owned:receipt),{mode:0o600});
  const foreignBytes=readFileSync(foreign);
  const ownedPath=join(root,'owned-device.json'), receiptPath=join(root,'receipt.private.json');
  if(scenario==='owned-symlink'||scenario==='owned-hardlink'){rmSync(ownedPath);(scenario.endsWith('symlink')?symlinkSync:linkSync)(foreign,ownedPath);}
  if(scenario==='receipt-symlink'||scenario==='receipt-hardlink'){rmSync(receiptPath);(scenario.endsWith('symlink')?symlinkSync:linkSync)(foreign,receiptPath);}
  if(scenario==='owned-loose-mode')chmodSync(ownedPath,0o644);
  if(scenario==='receipt-loose-mode')chmodSync(receiptPath,0o644);
  if(scenario==='receipt-oversized')writeFileSync(receiptPath,' '.repeat(8*1024*1024)+JSON.stringify(receipt));
  if(scenario==='receipt-unknown-field')writeFileSync(receiptPath,JSON.stringify({...receipt,arbitrary:secret}));
  if(scenario==='receipt-invalid-result')writeFileSync(receiptPath,JSON.stringify({...receipt,result:'success'}));
  if(scenario==='receipt-platform-conflict')writeFileSync(receiptPath,JSON.stringify({...receipt,platform:'android'}));
  if(scenario==='owned-unknown-field')writeFileSync(ownedPath,JSON.stringify({...owned,arbitrary:secret}));
  if(scenario==='owned-malformed-run')writeFileSync(ownedPath,JSON.stringify({...owned,run:['12345-1']}));
  if(scenario==='receipt-malformed-revision')writeFileSync(receiptPath,JSON.stringify({...receipt,revision:[receipt.revision]}));
  if(scenario==='receipt-invalid-utf8')writeFileSync(receiptPath,Buffer.from(JSON.stringify(receipt).replace(secret,'\xFF'),'latin1'));
  if(scenario==='owned-duplicate-field')writeFileSync(ownedPath,JSON.stringify(owned).replace('{','{"complete":false,'));
  if(scenario==='receipt-duplicate-field')writeFileSync(receiptPath,JSON.stringify(receipt).replace('{','{"result":0,'));
  if(scenario==='receipt-missing-field'){const value={...receipt};delete value.commands;writeFileSync(receiptPath,JSON.stringify(value));}
  if(scenario==='receipt-malformed')writeFileSync(receiptPath,'{ malformed private '+secret);
  if(scenario==='receipt-missing')rmSync(receiptPath);
  if(scenario==='owned-missing')rmSync(ownedPath);
  if(scenario==='cleanup-path-symlink')symlinkSync(foreign,join(root,'control.json'));
  if(scenario==='cleanup-path-hardlink')linkSync(foreign,join(root,'control.json'));
  const before=readFileSync(join(root,'sdk/report.json'));const privateBefore=existsSync(receiptPath)?readFileSync(receiptPath):null;
  const result=run(cwd);assert.notEqual(result.status,0);
  assert.equal(existsSync(join(root,'published')),false);assert.equal(existsSync(join(root,'sanitizing')),false);
  assert.deepEqual(readFileSync(join(root,'sdk/report.json')),before);if(privateBefore)assert.deepEqual(readFileSync(receiptPath),privateBefore);else assert.equal(existsSync(receiptPath),false);
  assert.deepEqual(readFileSync(foreign),foreignBytes);assert.ok(existsSync(join(root,'secrets.json')));
 });
}
test('receipt result remains a numeric failure when a short ledger string matches a public numeric fact',t=>{
  const {cwd,root}=fixture(t);
  writeFileSync(join(root,'secrets.json'),JSON.stringify({secrets:[secret,'1'],stage:'started'}),{mode:0o600});
  const result=run(cwd);assert.equal(result.status,0,result.stderr);
  const receipt=JSON.parse(readFileSync(join(root,'published/receipt.json'),'utf8'));
  assert.equal(receipt.result,1);assert.equal(receipt.elapsedSeconds,1);assert.equal(receipt.witness.nativeTool.build,'[REDACTED]');
});
for(const field of ['data','nativeTool','runtime','command-argument','cleanup-resource','journey-approval']) {
 test(`unregistered private text in ${field} is rejected before any effects`,t=>{
  const {cwd,root,receipt}=fixture(t), sentinel='UNREGISTERED-SYNTHETIC-SECRET';
  if(field==='data')receipt.data=sentinel;
  if(field==='nativeTool')receipt.witness.nativeTool=sentinel;
  if(field==='runtime')receipt.witness.runtime=sentinel;
  if(field==='command-argument')receipt.commands=[['node',sentinel]];
  if(field==='cleanup-resource')receipt.cleanup=[{resource:sentinel,result:0}];
  if(field==='journey-approval')receipt.journey={approval:sentinel,decodedFrames:true,partialProgressSeconds:1,watched:false,connectionRestored:true,progressPersisted:true,remoteFocus:true,menuReturned:true,tvForeground:true};
  const privatePath=join(root,'receipt.private.json');writeFileSync(privatePath,JSON.stringify(receipt),{mode:0o600});
  const before=readFileSync(privatePath), sdkBefore=readFileSync(join(root,'sdk/report.json'));
  const foreign=join(cwd,'foreign.txt');writeFileSync(foreign,'PRESERVE');
  const result=run(cwd);assert.notEqual(result.status,0);
  assert.equal(existsSync(join(root,'published')),false);assert.equal(existsSync(join(root,'sanitizing')),false);
  assert.deepEqual(readFileSync(privatePath),before);assert.deepEqual(readFileSync(join(root,'sdk/report.json')),sdkBefore);
  assert.equal(readFileSync(foreign,'utf8'),'PRESERVE');assert.ok(existsSync(join(root,'secrets.json')));
 });
}
function producerReceipt(root, owned, receipt, platform) {
  owned.platform=platform;owned.profile=platform==='ios'?'tvos':'androidtv';owned.device=platform==='ios'?'12345678-ABCD-4321-9876-123456789ABC':'emulator-5554';
  if(platform==='android'){owned.emulatorPID=12345;owned.emulatorStart='9999';}
  receipt.platform=platform;receipt.profile=owned.profile;receipt.device=owned.device;receipt.command=`python3 hosted.py ${owned.profile}`;
  receipt.witness.runtime=platform==='ios'?'com.apple.CoreSimulator.SimRuntime.tvOS-27-1':'system-images;android-36;android-tv;x86_64';
  receipt.witness.nativeTool=platform==='ios'?{name:'Xcode',version:'27.1',build:secret}:{name:'Java',version:'17.0.17',build:'17.0.17+10'};
  receipt.witness.sourceManifestSHA256='b'.repeat(64);receipt.witness.serverSHA256='c'.repeat(64);receipt.witness.appSHA256='d'.repeat(64);
  receipt.commands=[['go','build','-o',join(root,'bin/player'),'./apps/player/cmd/kinosail'],
    ...(platform==='ios'?[['./scripts/build-apple.sh','tvos'],['xcrun','simctl','boot',owned.device],['xcrun','simctl','bootstatus',owned.device,'-b'],['xcrun','simctl','shutdown',owned.device],['xcrun','simctl','delete',owned.device]]:[['./gradlew',':app:assembleDebug'],['avdmanager','create','avd','--name',`Kinosail-TV-E2E-${owned.run}`,'--package','system-images;android-36;android-tv;x86_64','--device','tv_1080p']]),
    ['node','node_modules/e2e/dist/cli/bin.js','run'],['node','node_modules/agent-device/bin/agent-device.mjs','daemon','stop','--state-dir',join(root,'agent-device')]];
  receipt.cleanup=[{resource:'owned agent-device daemon',result:0},...(platform==='ios'?['shutdown','delete'].map(action=>({resource:`owned simulator ${owned.device}`,action,result:0})):[{resource:'owned emulator process',result:-15}])];
  receipt.journey={approval:'public Owner API and actual dynamic UI code',decodedFrames:true,partialProgressSeconds:3,watched:false,connectionRestored:true,progressPersisted:true,remoteFocus:true,menuReturned:true,tvForeground:true};
}
for(const platform of ['ios','android']) {
 test(`publisher accepts ${platform} producer command templates and parsed synthetic build facts`,t=>{
  const {cwd,root,owned,receipt}=fixture(t);producerReceipt(root,owned,receipt,platform);
  for(const [name,value] of [['owned-device.json',owned],['receipt.private.json',receipt]])writeFileSync(join(root,name),JSON.stringify(value),{mode:0o600});
  const result=run(cwd);assert.equal(result.status,0,result.stderr);
  const safe=JSON.parse(readFileSync(join(root,'published/receipt.json'),'utf8'));
  assert.equal(safe.result,1);assert.equal(safe.journey.partialProgressSeconds,3);assert.equal(safe.platform,platform);assert.equal(safe.commands.length,receipt.commands.length);
  if(platform==='android')assert.deepEqual(safe.witness.nativeTool,{name:'Java',version:'17.0.17',build:'17.0.17+10'});
  assert.equal(readFileSync(join(root,'published/receipt.json'),'utf8').includes(secret),false);
 });
}
for(const scenario of ['tool-name','tool-version','tool-build','tool-extra-field','java-build','java-version-conflict','java-wrong-major','ios-runtime-format','android-runtime-conflict','command-extra-arg','command-missing-arg','command-other-platform','command-foreign-path','command-foreign-device','cleanup-foreign-device','cleanup-wrong-action','cleanup-other-platform','boundaries-missing','journey-fixture-overflow','android-serial-conflict']) {
 test(`publisher rejects ${scenario} before any effects and preserves raw and foreign bytes`,t=>{
  const {cwd,root,owned,receipt}=fixture(t);producerReceipt(root,owned,receipt,scenario.startsWith('java')||scenario.startsWith('android')?'android':'ios');
  const sentinel='UNREGISTERED-SYNTHETIC-SECRET';
  if(scenario==='tool-name')receipt.witness.nativeTool.name=sentinel;
  if(scenario==='tool-version')receipt.witness.nativeTool.version=sentinel;
  if(scenario==='tool-build')receipt.witness.nativeTool.build=sentinel;
  if(scenario==='tool-extra-field')receipt.witness.nativeTool.extra=sentinel;
  if(scenario==='java-build')receipt.witness.nativeTool.build=sentinel;
  if(scenario==='java-version-conflict')receipt.witness.nativeTool.build='17.0.16+10';
  if(scenario==='java-wrong-major')receipt.witness.nativeTool.version='21.0.1';
  if(scenario==='ios-runtime-format')receipt.witness.runtime+='-'+sentinel;
  if(scenario==='android-runtime-conflict')receipt.witness.runtime='system-images;android-35;google_apis;x86_64';
  if(scenario==='command-extra-arg')receipt.commands[0].push(sentinel);
  if(scenario==='command-missing-arg')receipt.commands[0].pop();
  if(scenario==='command-other-platform')receipt.commands.push(['./gradlew',':app:assembleDebug']);
  if(scenario==='command-foreign-path')receipt.commands[0][3]=join(cwd,'foreign.txt');
  if(scenario==='command-foreign-device')receipt.commands.push(['xcrun','simctl','delete','87654321-ABCD-4321-9876-123456789ABC']);
  if(scenario==='cleanup-foreign-device')receipt.cleanup[1].resource='owned simulator 87654321-ABCD-4321-9876-123456789ABC';
  if(scenario==='cleanup-wrong-action')receipt.cleanup[0].action='delete';
  if(scenario==='cleanup-other-platform')receipt.cleanup.push({resource:'owned emulator process',result:0});
  if(scenario==='boundaries-missing')receipt.boundaries.pop();
  if(scenario==='journey-fixture-overflow')receipt.journey.partialProgressSeconds=100000;
  if(scenario==='android-serial-conflict'){owned.device='emulator-5556';receipt.device=owned.device;}
  for(const [name,value] of [['owned-device.json',owned],['receipt.private.json',receipt]])writeFileSync(join(root,name),JSON.stringify(value),{mode:0o600});
  const privatePath=join(root,'receipt.private.json'),before=readFileSync(privatePath),sdkBefore=readFileSync(join(root,'sdk/report.json'));
  const foreign=join(cwd,'foreign.txt');writeFileSync(foreign,'PRESERVE');
  assert.notEqual(run(cwd).status,0);assert.equal(existsSync(join(root,'published')),false);assert.equal(existsSync(join(root,'sanitizing')),false);
  assert.deepEqual(readFileSync(privatePath),before);assert.deepEqual(readFileSync(join(root,'sdk/report.json')),sdkBefore);assert.equal(readFileSync(foreign,'utf8'),'PRESERVE');assert.ok(existsSync(join(root,'secrets.json')));
 });
}
test('publisher rejects unresolved creation even if a malformed owner claims cleanup complete',t=>{
 const {cwd,root,owned}=fixture(t);owned.creationPending={name:'Kinosail-TV-E2E-12345-1',runtime:'com.apple.CoreSimulator.SimRuntime.tvOS-27-1',type:'com.apple.CoreSimulator.SimDeviceType.Apple-TV-4K-3rd-generation-4K',inventoryZero:true,inventorySHA256:'a'.repeat(64)};
 writeFileSync(join(root,'owned-device.json'),JSON.stringify(owned),{mode:0o600});const before=readFileSync(join(root,'sdk/report.json'));
 assert.notEqual(run(cwd).status,0);assert.equal(existsSync(join(root,'published')),false);assert.equal(existsSync(join(root,'sanitizing')),false);assert.deepEqual(readFileSync(join(root,'sdk/report.json')),before);assert.ok(existsSync(join(root,'receipt.private.json')));
});

test('successful receipt cannot omit actual journey proof',t=>{const {cwd,root,owned,receipt}=fixture(t);producerReceipt(root,owned,receipt,'ios');receipt.result=0;delete receipt.journey;writeFileSync(join(root,'owned-device.json'),JSON.stringify(owned),{mode:0o600});writeFileSync(join(root,'receipt.private.json'),JSON.stringify(receipt),{mode:0o600});assert.notEqual(run(cwd).status,0);assert.equal(existsSync(join(root,'published')),false);assert.ok(existsSync(join(root,'sdk')));});
test('successful journey without terminal privacy stage cannot publish',t=>{const {cwd,root,owned,receipt}=fixture(t);producerReceipt(root,owned,receipt,'ios');receipt.result=0;writeFileSync(join(root,'owned-device.json'),JSON.stringify(owned),{mode:0o600});writeFileSync(join(root,'receipt.private.json'),JSON.stringify(receipt),{mode:0o600});assert.notEqual(run(cwd).status,0);assert.equal(existsSync(join(root,'published')),false);assert.ok(existsSync(join(root,'sdk')));});
test('complete real-journey facts publish only masked text after owned cleanup',t=>{const {cwd,root,owned,receipt}=fixture(t);producerReceipt(root,owned,receipt,'android');receipt.result=0;for(const [name,value] of [['owned-device.json',owned],['receipt.private.json',receipt],['secrets.json',{secrets:[secret,'839271'],stage:'complete'}]])writeFileSync(join(root,name),JSON.stringify(value),{mode:0o600});writeFileSync(join(root,'sdk/frame-first.png'),'839271',{mode:0o600});assert.equal(run(cwd).status,0);assert.deepEqual(readdirSync(join(root,'published')).filter(n=>n.endsWith('.png')),[]);assert.equal(existsSync(join(root,'sdk')),false);assert.equal(JSON.parse(readFileSync(join(root,'published/receipt.json'))).privacy.stage,'complete');});
