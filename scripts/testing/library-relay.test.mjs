import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import {EventEmitter} from 'node:events';
import {PassThrough, Duplex} from 'node:stream';
import {createHash} from 'node:crypto';
const id='a'.repeat(64), netID='b'.repeat(64), image='sha256:'+'c'.repeat(64), name='kinosail-library-123-456';
const token='d'.repeat(32);
const container={owner:token,id,name:'/'+name,image,running:true,networks:[{name,id:netID,ip:'172.28.0.2'}]};
const network={owner:token,bridge:'',gateway:'172.28.0.1',id:netID,name,internal:true,driver:'bridge',subnets:['172.28.0.0/16'],members:[{id,name,address:'172.28.0.2/16'}]};
function owner(overrides={}) {
 const calls=[], listener=new EventEmitter();listener.listening=false;
 listener.listen=(port,host,ready)=>{calls.push(['listen',port,host]);listener.listening=true;ready();};
 listener.address=()=>({port:49152});listener.close=callback=>{listener.listening=false;calls.push(['closed']);callback();};
 const sockets=[];
 const source=readFileSync(new URL('../ci/library-internal-relay.mjs',import.meta.url),'utf8').split('\nif (process.argv[1]')[0].replace(/^import .*;\n/gm,'').replace(/^export /gm,'');
 const {execFileSync:metadataOverride,...rest}=overrides;
 const context={JSON,Buffer,Error,Set,Promise,Date,createHash,setTimeout,clearTimeout,process:{platform:'linux',argv:[],env:{}},
  networkInterfaces:()=>({['br-'+netID.slice(0,12)]:[{address:'172.28.0.1',family:'IPv4',internal:false,cidr:'172.28.0.1/16'}]}),
  execFileSync:(engine,args,options)=>{calls.push(['inspect',engine,args[0]]);if(overrides.beforeCommand)overrides.beforeCommand(options);if(args[0]==='context')return args[1]==='show'?'default\n':JSON.stringify('unix:///var/run/docker.sock');return metadataOverride?metadataOverride(engine,args,options):JSON.stringify(args[0]==='inspect'?container:network);},
  createServer:callback=>{listener.connection=callback;return listener;},
  createConnection:options=>{calls.push(['connect',options.host,options.port]);const socket=new PassThrough();socket.setTimeout=()=>socket;sockets.push(socket);return socket;},
  ...rest};
 const api=runInNewContext(`(()=>{${source}\nreturn {inspectTarget,startRelay,ownedResource:typeof ownedResource==='function'?ownedResource:undefined,localDocker:typeof localDocker==='function'?localDocker:undefined};})()`,context);
 return {api,calls,listener,sockets};
}
test('owned internal metadata admits one target before fixed loopback relay and joined cleanup',async()=>{
 const o=owner(),target=o.api.inspectTarget('docker',name,name,image,token);
 assert.equal(target.ip,'172.28.0.2');const relay=await o.api.startRelay(target);
 assert.equal(relay.port,49152);assert.deepEqual(o.calls.slice(0,2).map(v=>v[0]),['inspect','inspect']);
 assert.deepEqual(o.calls[4],['listen',0,'127.0.0.1']);
 const client=new PassThrough();o.listener.connection(client);
 assert.deepEqual(o.calls[5],['connect','172.28.0.2',38127]);await relay.close();
 assert.equal(client.destroyed,true);assert.equal(o.sockets[0].destroyed,true);assert.equal(o.listener.listening,false);
});
test('missing unknown malformed oversized duplicate and conflicting metadata reject before listen/connect',()=>{
 const invalid=[null,{},[],{...container,running:false},{...container,image:'private-image'},
  {...container,networks:[...container.networks,container.networks[0]]},
  {...container,networks:[{...container.networks[0],ip:'127.0.0.1'}]},
  {...container,networks:[{...container.networks[0],ip:'8.8.8.8'}]},
  {...container,networks:[{...container.networks[0],ip:'172.29.0.2'}]},
  {...container,networks:[{...container.networks[0],ip:'172.28.0.0'}]},
  {...container,name:'/unrelated'}, {...container,private:'secret'},
  '{', 'x'.repeat(65537), JSON.stringify(container).replace('"running":true','"running":false,"running":true')];
 for(const value of invalid){const o=owner({execFileSync:(_engine,args)=>typeof value==='string'?value:JSON.stringify(value)});
  assert.throws(()=>o.api.inspectTarget('docker',name,name,image,token),/invalid owned Library relay/);
  assert.equal(o.calls.some(v=>v[0]==='listen'||v[0]==='connect'),false);}
 for(const change of [{internal:false},{driver:'host'},{id:'d'.repeat(64)},{members:[]},
  {members:[{...network.members[0],id:'d'.repeat(64)}]},{subnets:['0.0.0.0/0']}, {subnets:['172.0.0.0/8']}, {subnets:['172.28.0.0/16','172.29.0.0/16']}]){
  const o=owner({execFileSync:(_engine,args)=>JSON.stringify(args[0]==='inspect'?container:{...network,...change})});
  assert.throws(()=>o.api.inspectTarget('docker',name,name,image,token),/invalid owned Library relay/);assert.equal(o.calls.some(v=>v[0]==='listen'||v[0]==='connect'),false);}
});
test('invalid owner CLI arguments reject before daemon effects',()=>{
 for(const args of [['curl',name,name,image],['docker','unrelated',name,image],['docker',name,name,'unknown'],['docker',name,name+'x',image]]){
  const o=owner();assert.throws(()=>o.api.inspectTarget(...args,token),/invalid owned Library relay/);assert.equal(o.calls.length,0);
 }
});
test('asynchronous listen failure preserves sanitized rejection and closes owned listener',async()=>{
 for(const failure of [new Error('private daemon data'),'private daemon string']) {
  const listener=new EventEmitter();listener.listening=false;
  listener.listen=()=>queueMicrotask(()=>listener.emit('error',failure));listener.close=cb=>cb();
  const o=owner({createServer:()=>listener});await assert.rejects(o.api.startRelay({ip:'172.28.0.2'}),/owned Library relay listen failed/);
  assert.equal(listener.listenerCount('error'),0);
 }
});

test('relay operation rejects malformed or external targets before socket effects',()=>{
 for(const target of [{ip:'8.8.8.8'},null,{}, {ip:'127.0.0.1'}, {ip:'172.28.0.2',port:80}, {ip:'172.028.0.2'}]){
  const o=owner();assert.throws(()=>o.api.startRelay(target),/invalid owned Library relay/);
  assert.equal(o.calls.length,0);
 }
});

test('owned local bridge and cryptographic labels reject foreign topology before listen',()=>{
 for(const overrides of [
  {process:{platform:'linux',argv:[],env:{DOCKER_HOST:'tcp://outside.invalid:2375'}}},
  {process:{platform:'linux',argv:[],env:{DOCKER_CONTEXT:'remote'}}},
  {networkInterfaces:()=>({})},
  {networkInterfaces:()=>({['br-'+netID.slice(0,12)]:[{address:'172.28.0.3',family:'IPv4',internal:false,cidr:'172.28.0.3/16'}]})},
  {execFileSync:(_engine,args)=>JSON.stringify(args[0]==='inspect'?{...container,owner:'foreign'}:network)},
  {execFileSync:(_engine,args)=>JSON.stringify(args[0]==='inspect'?container:{...network,owner:'foreign'})}
 ]){
  const o=owner(overrides);assert.throws(()=>o.api.inspectTarget('docker',name,name,image,token),/invalid owned Library relay/);
  assert.equal(o.calls.some(v=>v[0]==='listen'||v[0]==='connect'),false);
 }
});
test('resource ownership accepts only captured canonical identity and fresh matching label',()=>{
 const o=owner({execFileSync:()=>JSON.stringify({id,name:'/'+name,owner:token})});
 assert.equal(o.api.ownedResource('docker','container',name,token,'-'),id);
 assert.equal(o.api.ownedResource('docker','container',name,token,id),id);
 for(const value of [{id,name:'/'+name,owner:'foreign'}, {id:'e'.repeat(64),name:'/'+name,owner:token},
  {id,name:'/unrelated',owner:token}, {id,name:'/'+name,owner:token,private:'secret'}]){
  const peer=owner({execFileSync:()=>JSON.stringify(value)});
  assert.throws(()=>peer.api.ownedResource('docker','container',name,token,id),/invalid owned Library relay/);
 }
});

test('four near-bound metadata reads fit one declared total startup budget',()=>{
 let elapsed=0;
 const clock=class extends Date {static now(){return elapsed;}};
 const o=owner({Date:clock,beforeCommand:options=>{assert.equal(options.timeout,3000);elapsed+=2900;}});
 assert.equal(o.api.inspectTarget('docker',name,name,image,token).ip,'172.28.0.2');
 assert.equal(elapsed,11600);
});
test('stalled listener fails within remaining startup budget and closes without a socket',async()=>{
 const listener=new EventEmitter();listener.listening=false;let closes=0;
 listener.listen=()=>{};listener.close=callback=>{closes++;callback();};
 const o=owner({createServer:()=>listener});
 const result=await Promise.race([o.api.startRelay({ip:'172.28.0.2'},Date.now()+15).then(()=> 'unexpected success',error=>error.message),
  new Promise(resolve=>setTimeout(()=>resolve('unbounded pending listener'),60))]);
 assert.match(result,/owned Library relay startup deadline/);
 assert.equal(closes,1);assert.equal(listener.listenerCount('error'),0);
});

test('resource kind and owned name are a closed cross-field admission before inspect',()=>{
 for(const [kind,resource] of [['volume',name],['network','kinosail-library-config-123-456'],['container','kinosail-library-cache-123-456']]){
  const o=owner();assert.throws(()=>o.api.ownedResource('docker',kind,resource,token,'-'),/invalid owned Library relay/);
  assert.equal(o.calls.length,0);
 }
});

test('default unnamed Docker bridge option still requires its actual owned host interface',()=>{
 const o=owner({execFileSync:(_engine,args)=>JSON.stringify(args[0]==='inspect'?container:{...network,bridge:null})});
 assert.equal(o.api.inspectTarget('docker',name,name,image,token).ip,'172.28.0.2');
 assert.equal(o.calls.some(v=>v[0]==='listen'||v[0]==='connect'),false);
});

test('volume cleanup identity binds its fresh owner and creation time',()=>{
 const volume='kinosail-library-config-123-456',created='2026-10-08T00:00:00Z';
 const metadata={id:volume,name:volume,owner:token,created};
 const o=owner({execFileSync:()=>JSON.stringify(metadata)});
 const fingerprint=o.api.ownedResource('docker','volume',volume,token,'-');
 assert.match(fingerprint,/^[a-f0-9]{64}$/);
 assert.equal(o.api.ownedResource('docker','volume',volume,token,fingerprint),fingerprint);
 for(const change of [{created:'2026-10-08T00:00:01Z'},{created:'invalid'},{owner:'foreign'},{id:'foreign'}]){
  const peer=owner({execFileSync:()=>JSON.stringify({...metadata,...change})});
  assert.throws(()=>peer.api.ownedResource('docker','volume',volume,token,fingerprint),/invalid owned Library relay/);
  assert.equal(peer.calls.some(row=>row[0]==='listen'||row[0]==='connect'),false);
 }
});
test('opaque streams preserve binary bytes both ways and cleanup joins both endpoints',async()=>{
 const sent=[],received=[];
 const client=new Duplex({read(){},write(chunk,_encoding,callback){received.push(Buffer.from(chunk));callback();}});
 const peer=new Duplex({read(){},write(chunk,_encoding,callback){sent.push(Buffer.from(chunk));callback();}});
 peer.setTimeout=()=>peer;
 const o=owner({createConnection:options=>{assert.equal(options.host,'172.28.0.2');assert.equal(options.port,38127);return peer;}});
 const relay=await o.api.startRelay({ip:'172.28.0.2'});
 o.listener.connection(client);
 const request=Buffer.from([0,255,22,3,1,10,13]),response=Buffer.from([255,0,23,3,3,128]);
 client.push(request);peer.push(response);
 await new Promise(resolve=>setImmediate(resolve));
 assert.deepEqual(Buffer.concat(sent),request);assert.deepEqual(Buffer.concat(received),response);
 await relay.close();assert.equal(client.destroyed,true);assert.equal(peer.destroyed,true);
 assert.equal(o.listener.listening,false);
});
