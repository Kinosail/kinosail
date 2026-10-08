import { constants, openSync, closeSync, fstatSync, lstatSync, ftruncateSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { readControl } from './control.mjs';
const stages = ['inventory','reinstall','open','foreground','use','close','owner','library','server_address','connect','approval','movies','movie_focus','play','decoded_frames','pause','menu','progress','relaunch','restored_connection','persisted_progress'];
function fields(value, expected) {
 if(!value || typeof value!=='object' || Array.isArray(value) || Object.keys(value).sort().join(',')!==expected.sort().join(','))throw Error('invalid TV action fields');
}
export function validateTvActions(value) {
 fields(value,['version','events']);
 if(value.version!==1 || !Array.isArray(value.events) || value.events.length>96)throw Error('invalid TV actions');
 const stack=[];
 for(const event of value.events){
  fields(event,event.status==='failed'?['stage','status','failure']:['stage','status']);
  if(!stages.includes(event.stage) || !['started','passed','failed'].includes(event.status))throw Error('invalid TV action stage');
  if(event.status==='started')stack.push(event.stage);
  else if(stack.pop()!==event.stage)throw Error('conflicting TV action order');
  if(event.status==='failed'){
   const failure=event.failure;fields(failure,['category','ownKeyCount','recognizedKeyMask']);
   if(failure.category!=='unqualified' || failure.ownKeyCount!==null && (!Number.isInteger(failure.ownKeyCount) || failure.ownKeyCount<0 || failure.ownKeyCount>128) || !Number.isInteger(failure.recognizedKeyMask) || failure.recognizedKeyMask<0 || failure.recognizedKeyMask>15 || failure.ownKeyCount===null && failure.recognizedKeyMask!==0)throw Error('invalid TV failure shape');
   if(failure.ownKeyCount!==null && failure.ownKeyCount<[1,2,4,8].filter(bit=>failure.recognizedKeyMask&bit).length)throw Error('conflicting TV failure shape');
  }
 }
 return value;
}
function failureShape(error) {
 // No error values, getters, messages, IDs, paths, arguments or SDK output are read.
 const unknown={category:'unqualified',ownKeyCount:null,recognizedKeyMask:0};
 try{
  if(!error || typeof error!=='object')return unknown;
  const keys=Object.getOwnPropertyNames(error);if(keys.length>128)return unknown;
  return {...unknown,ownKeyCount:keys.length,recognizedKeyMask:['code','details','cause','message'].reduce((mask,key,index)=>mask|(keys.includes(key)?1<<index:0),0)};
 }catch{return unknown;}
}
export function createTvActions(directory=process.cwd()) {
 const control=readControl(directory),root=join(directory,'.e2e'),rootID=lstatSync(root),path=join(root,'tv-actions.private.json');
 const events=[];
 const fd=openSync(path,constants.O_WRONLY|constants.O_CREAT|constants.O_EXCL|constants.O_NOFOLLOW,0o600);
 let fileID;
 try{fileID=fstatSync(fd);writeFileSync(fd,JSON.stringify({version:1,events}));}finally{closeSync(fd);}
 function save(){
  if(JSON.stringify(readControl(directory))!==JSON.stringify(control))throw Error('TV action owner changed');
  const current=lstatSync(root);if(current.dev!==rootID.dev || current.ino!==rootID.ino)throw Error('TV action root changed');
  const record=validateTvActions({version:1,events});
  const output=openSync(path,constants.O_WRONLY|constants.O_NOFOLLOW|constants.O_NONBLOCK);
  try{
   const info=fstatSync(output),named=lstatSync(path);
   if(!info.isFile() || info.nlink!==1 || info.uid!==process.getuid() || (info.mode&0o777)!==0o600 || info.dev!==fileID.dev || info.ino!==fileID.ino || named.dev!==info.dev || named.ino!==info.ino)throw Error('TV action file changed');
   ftruncateSync(output,0);writeFileSync(output,JSON.stringify(record));
  }finally{closeSync(output);}
 }
 return {/** @template T @param {string} stage @param {()=>Promise<T>} operation @returns {Promise<T>} */
  async step(stage,operation){
   if(!stages.includes(stage) || typeof operation!=='function')throw Error('invalid TV action');
   events.push({stage,status:'started'});save();
   let result;
   try{result=await operation();}catch(error){
    events.push({stage,status:'failed',failure:failureShape(error)});
    try{save();}catch{} // A frozen/opaque SDK failure remains the original thrown value.
    throw error;
   }
   events.push({stage,status:'passed'});save();return result;
  }
 };
}
