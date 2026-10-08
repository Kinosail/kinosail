import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import {EventEmitter} from 'node:events';
import {PassThrough} from 'node:stream';
const id='a'.repeat(64), netID='b'.repeat(64), image='sha256:'+'c'.repeat(64), name='kinosail-library-123-456';
const container={id,name:'/'+name,image,running:true,networks:[{name,id:netID,ip:'172.28.0.2'}]};
const network={id:netID,name,internal:true,driver:'bridge',subnets:['172.28.0.0/16'],members:[{id,name,address:'172.28.0.2/16'}]};
function owner(overrides={}) {
 const calls=[], listener=new EventEmitter();listener.listening=false;
 listener.listen=(port,host,ready)=>{calls.push(['listen',port,host]);listener.listening=true;ready();};
 listener.address=()=>({port:49152});listener.close=callback=>{listener.listening=false;calls.push(['closed']);callback();};
 const sockets=[];
 const source=readFileSync(new URL('../ci/library-internal-relay.mjs',import.meta.url),'utf8').split('\nif (process.argv[1]')[0].replace(/^import .*;\n/gm,'').replace(/^export /gm,'');
 const context={JSON,Buffer,Error,Set,Promise,process:{platform:'linux',argv:[]},
  execFileSync:(engine,args,options)=>{calls.push(['inspect',engine,args[0]]);return JSON.stringify(args[0]==='inspect'?container:network);},
  createServer:callback=>{listener.connection=callback;return listener;},
  createConnection:options=>{calls.push(['connect',options.host,options.port]);const socket=new PassThrough();socket.setTimeout=()=>socket;sockets.push(socket);return socket;},
  ...overrides};
 const api=runInNewContext(`(()=>{${source}\nreturn {inspectTarget,startRelay};})()`,context);
 return {api,calls,listener,sockets};
}
test('owned internal metadata admits one target before fixed loopback relay and joined cleanup',async()=>{
 const o=owner(),target=o.api.inspectTarget('docker',name,name,image);
 assert.equal(target.ip,'172.28.0.2');const relay=await o.api.startRelay(target);
 assert.equal(relay.port,49152);assert.deepEqual(o.calls.slice(0,2).map(v=>v[0]),['inspect','inspect']);
 assert.deepEqual(o.calls[2],['listen',0,'127.0.0.1']);
 const client=new PassThrough();o.listener.connection(client);
 assert.deepEqual(o.calls[3],['connect','172.28.0.2',38127]);await relay.close();
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
  assert.throws(()=>o.api.inspectTarget('docker',name,name,image),/invalid owned Library relay/);
  assert.equal(o.calls.length,0);}
 for(const change of [{internal:false},{driver:'host'},{id:'d'.repeat(64)},{members:[]},
  {members:[{...network.members[0],id:'d'.repeat(64)}]},{subnets:['0.0.0.0/0']}, {subnets:['172.0.0.0/8']}, {subnets:['172.28.0.0/16','172.29.0.0/16']}]){
  const o=owner({execFileSync:(_engine,args)=>JSON.stringify(args[0]==='inspect'?container:{...network,...change})});
  assert.throws(()=>o.api.inspectTarget('docker',name,name,image),/invalid owned Library relay/);assert.equal(o.calls.length,0);}
});
test('invalid owner CLI arguments reject before daemon effects',()=>{
 for(const args of [['curl',name,name,image],['docker','unrelated',name,image],['docker',name,name,'unknown'],['docker',name,name+'x',image]]){
  const o=owner();assert.throws(()=>o.api.inspectTarget(...args),/invalid owned Library relay/);assert.equal(o.calls.length,0);
 }
});
test('asynchronous listen failure preserves sanitized rejection and closes owned listener',async()=>{
 const failure=new Error('private daemon data');const listener=new EventEmitter();listener.listening=false;
 listener.listen=()=>queueMicrotask(()=>listener.emit('error',failure));listener.close=cb=>cb();
 const o=owner({createServer:()=>listener});await assert.rejects(o.api.startRelay({ip:'172.28.0.2'}),/owned Library relay listen failed/);
 assert.equal(listener.listenerCount('error'),0);
});

test('relay operation rejects malformed or external targets before socket effects',()=>{
 for(const target of [{ip:'8.8.8.8'},null,{}, {ip:'127.0.0.1'}, {ip:'172.28.0.2',port:80}, {ip:'172.028.0.2'}]){
  const o=owner();assert.throws(()=>o.api.startRelay(target),/invalid owned Library relay/);
  assert.equal(o.calls.length,0);
 }
});
