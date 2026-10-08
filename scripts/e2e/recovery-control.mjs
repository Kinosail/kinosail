import {closeSync,constants,fstatSync,lstatSync,openSync,readFileSync,unlinkSync,writeFileSync} from "node:fs";
import {join} from "node:path";

const pid=value=>Number.isInteger(value)&&value>1&&value<=2147483647;
const owns=(path,identity)=>{try{const current=lstatSync(path);return !current.isSymbolicLink()&&current.dev===identity.dev&&current.ino===identity.ino;}catch(error){if(error.code!=="ENOENT")throw error;return false;}};
const portValue=port=>typeof port==="string"&&/^[1-9]\d{3,4}$/.test(port)&&+port>=1024&&+port<=65535;

export function decodeRecoveryRequest(bytes,childPID,generation){
 if(!Buffer.isBuffer(bytes)||bytes.length>192||!pid(childPID)||![0,1].includes(generation))throw new Error("invalid recovery request");
 let raw,value;try{raw=new TextDecoder("utf-8",{fatal:true}).decode(bytes);value=JSON.parse(raw);}catch{throw new Error("invalid recovery request");}
 if(!value||Array.isArray(value)||typeof value!=="object"||raw!==JSON.stringify(value)
    ||Object.keys(value).sort().join(",")!=="childPID,generation,operation"||value.childPID!==childPID
    ||value.generation!==generation||value.operation!==(generation===0?"backup":"restore"))throw new Error("invalid recovery request");
 return value.operation;
}

// Synchronous only: pin the actual cwd inode before opening a relative mailbox.
// No asynchronous work runs while process cwd is temporarily changed.
function inControlDirectory(port,callback){
 if(!portValue(port))throw new Error("invalid fixture port");
 const previous=process.cwd(),paths=[join(previous,".e2e"),join(previous,".e2e/fixtures")];
 const identities=paths.map(path=>{const stat=lstatSync(path);if(stat.isSymbolicLink()||!stat.isDirectory())throw new Error("invalid fixture control parent");return stat;});
 const fd=openSync(paths[1],constants.O_RDONLY|constants.O_DIRECTORY|constants.O_NOFOLLOW);
 let entered=false;
 const check=()=>paths.every((path,index)=>owns(path,identities[index]));
 try{
  const directory=fstatSync(fd);if(!directory.isDirectory()||directory.dev!==identities[1].dev||directory.ino!==identities[1].ino||!check())throw new Error("fixture parent changed");
  process.chdir(paths[1]);entered=true;
  if(!owns(".",directory)||!check())throw new Error("fixture parent changed");
  return callback(check);
 }finally{if(entered)process.chdir(previous);closeSync(fd);}
}

function receipt(port){
 const fd=openSync(port+".json",constants.O_RDONLY|constants.O_NOFOLLOW|constants.O_NONBLOCK);
 try{
  const stat=fstatSync(fd);if(!stat.isFile()||stat.size<1||stat.size>4096)throw new Error("invalid fixture receipt");
  const bytes=readFileSync(fd);if(bytes.length!==stat.size||bytes.length>4096)throw new Error("invalid fixture receipt");
  let raw,value;try{raw=new TextDecoder("utf-8",{fatal:true}).decode(bytes);value=JSON.parse(raw);}catch{throw new Error("invalid fixture receipt");}
  const keys=Object.keys(value??{}).sort().join(",");
  if(!value||Array.isArray(value)||raw!==JSON.stringify(value)||!["app,childPID,generation,port,supervisorPID","app,childPID,generation,port,recovery,supervisorPID"].includes(keys)
     ||!["player","subtitles"].includes(value.app)||value.port!==port||!pid(value.supervisorPID)||!pid(value.childPID)
     ||!Number.isInteger(value.generation)||value.generation<0||value.generation>3)throw new Error("invalid fixture receipt");
  if(value.recovery!==undefined){
   const v=value.recovery,restore=v?.operation==="restore";
   const expected=restore?"archiveBytes,archiveSHA256,corruptDataUnchanged,corruptRejected,operation,restored,verified":"archiveBytes,archiveSHA256,operation,verified";
   if(!v||typeof v!=="object"||Array.isArray(v)||Object.keys(v).sort().join(",")!==expected
      ||!["backup","restore"].includes(v.operation)||v.verified!==true||typeof v.archiveSHA256!=="string"||!/^[a-f0-9]{64}$/.test(v.archiveSHA256)
      ||!Number.isSafeInteger(v.archiveBytes)||v.archiveBytes<1||v.archiveBytes>33554432
      ||value.generation<(restore?2:1)||restore&&(v.corruptDataUnchanged!==true||v.corruptRejected!==true||v.restored!==true))throw new Error("invalid fixture recovery receipt");
  }
  if(!owns(port+".json",stat))throw new Error("fixture receipt changed");
  return value;
 }finally{closeSync(fd);}
}
export function readFixtureReceipt(port){
 return inControlDirectory(port,check=>{const value=receipt(port);if(!check())throw new Error("fixture parent changed");return value;});
}
export function requestRecovery(port,request){
 if(!request||typeof request!=="object"||Array.isArray(request)
    ||Object.keys(request).sort().join(",")!=="childPID,generation,operation"||!pid(request.childPID)
    ||![0,1].includes(request.generation)||request.operation!==(request.generation===0?"backup":"restore"))throw new Error("invalid recovery request");
 const bytes=Buffer.from(JSON.stringify(request));decodeRecoveryRequest(bytes,request.childPID,request.generation);
 return inControlDirectory(port,check=>{
  const state=receipt(port);
  if(state.app!=="player"||state.childPID!==request.childPID||state.generation!==request.generation||!check())throw new Error("recovery receipt mismatch");
  const name=port+".restart";let fd,identity;
  try{
   fd=openSync(name,constants.O_WRONLY|constants.O_CREAT|constants.O_EXCL|constants.O_NOFOLLOW,0o600);identity=fstatSync(fd);
   if(!check())throw new Error("fixture parent changed");
   writeFileSync(fd,bytes);
   if(!check()||!owns(name,identity))throw new Error("fixture control changed");
  }catch(error){
   if(identity&&owns(name,identity))unlinkSync(name);
   throw error;
  }finally{if(fd!==undefined)closeSync(fd);}
 });
}
