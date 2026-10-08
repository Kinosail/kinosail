import {constants,openSync,closeSync,fstatSync,lstatSync,readSync} from 'node:fs';
import {resolve,dirname,basename,join} from 'node:path';
import {decodeFixtureJSON} from '../../../scripts/e2e/fixture-response.mjs';

// Cooperative owned startup run: descriptor/parent checks precede context load.
function readOwnedPresentationFile(run,parent,name,limit) {
 if(typeof parent!=='string'||parent.length>4096||resolve(parent)!==parent)throw new Error('invalid presentation parent');
 if(typeof run!=='string'||run.length>4096||resolve(run)!==run||dirname(run)!==resolve(parent)
  ||!/^\d{8}T\d{6}Z$/.test(basename(run)))throw new Error('invalid presentation run');
 const directories=[parent,run].map(path=>{
  const stat=lstatSync(path);if(!stat.isDirectory()||stat.isSymbolicLink()||stat.uid!==process.getuid())throw new Error('invalid presentation parent');return stat;
 });
 if((directories[1].mode&0o777)!==0o700)throw new Error('presentation run is not private');
 const path=join(run,name),fd=openSync(path,constants.O_RDONLY|constants.O_NOFOLLOW|constants.O_NONBLOCK);
 try{
  const stat=fstatSync(fd);
  if(!stat.isFile()||stat.uid!==process.getuid()||(stat.mode&0o777)!==0o600||stat.size<1||stat.size>limit)throw new Error('invalid private presentation state');
  const buffer=Buffer.alloc(stat.size+1);let size=0,count;
  while(size<buffer.length&&(count=readSync(fd,buffer,size,buffer.length-size,null))>0)size+=count;
  if(size!==stat.size)throw new Error('presentation state changed');
  const raw=buffer.subarray(0,size);
  const after=lstatSync(path);
  if(after.isSymbolicLink()||after.dev!==stat.dev||after.ino!==stat.ino||after.size!==stat.size
   ||![parent,run].every((path,i)=>{const s=lstatSync(path);return !s.isSymbolicLink()&&s.dev===directories[i].dev&&s.ino===directories[i].ino;}))throw new Error('presentation state ownership changed');
  return decodeFixtureJSON(new TextDecoder('utf-8',{fatal:true}).decode(raw));
 }finally{closeSync(fd);}
}
export function readPresentationMap(run,parent) {
 return readOwnedPresentationFile(run,parent,'source-frame-map.json',65536);
}
export function readPresentationState(run,parent,origin) {
 const state=readOwnedPresentationFile(run,parent,'presentation-auth.json',131072);
  const keys=(v,names)=>v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).sort().join(',')===[...names].sort().join(',');
  if(!keys(state,['cookies','origins'])||!Array.isArray(state.cookies)||state.cookies.length>32||!Array.isArray(state.origins)||state.origins.length>4)throw new Error('invalid presentation state');
  for(const cookie of state.cookies){
   const allowed=['name','value','domain','path','expires','httpOnly','secure','sameSite','partitionKey'];
   if(!cookie||Object.keys(cookie).some(k=>!allowed.includes(k))||allowed.slice(0,8).some(k=>!Object.hasOwn(cookie,k))
    ||!['name','value','domain','path'].every(k=>typeof cookie[k]==='string'&&Buffer.byteLength(cookie[k])<=8192)
    ||!['localhost','.localhost'].includes(cookie.domain)||!cookie.path.startsWith('/')||!Number.isFinite(cookie.expires)
    ||typeof cookie.httpOnly!=='boolean'||typeof cookie.secure!=='boolean'||!['Strict','Lax','None'].includes(cookie.sameSite)
    ||Object.hasOwn(cookie,'partitionKey'))throw new Error('invalid presentation cookie');
  }
  for(const entry of state.origins){
   if(!keys(entry,['origin','localStorage'])||entry.origin!==origin||!Array.isArray(entry.localStorage)||entry.localStorage.length>64
    ||entry.localStorage.some(v=>!keys(v,['name','value'])||![v.name,v.value].every(t=>typeof t==='string'&&Buffer.byteLength(t)<=8192)))throw new Error('invalid presentation storage');
  }
  return state;
}
