import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync,existsSync,rmSync,renameSync,symlinkSync} from "node:fs";
import {tmpdir} from "node:os";
import {join} from "node:path";
import {syncBuiltinESMExports} from "node:module";
import {decodeRecoveryRequest,requestRecovery,readFixtureReceipt} from "../recovery-control.mjs";

const port="49131", childPID=1234;
const request={childPID,generation:0,operation:"backup"};
function directory(t){
 const root=mkdtempSync(join(tmpdir(),"owned-recovery-control-")),previous=process.cwd();
 mkdirSync(join(root,".e2e/fixtures"),{recursive:true});
 writeFileSync(join(root,".e2e/fixtures",port+".json"),JSON.stringify({app:"player",port,supervisorPID:process.pid,childPID,generation:0}));
 process.chdir(root);
 t.after(()=>{process.chdir(previous);rmSync(root,{recursive:true,force:true});});
 return root;
}
test("canonical recovery request binds operation to current owned child generation",()=>{
 assert.equal(decodeRecoveryRequest(Buffer.from(JSON.stringify(request)),childPID,0),"backup");
 assert.equal(decodeRecoveryRequest(Buffer.from(JSON.stringify({...request,generation:1,operation:"restore"})),childPID,1),"restore");
});
for(const [name,bytes] of [
 ["missing",Buffer.from("{}")],["unknown",Buffer.from(JSON.stringify({...request,extra:true}))],
 ["duplicate",Buffer.from('{"childPID":1234,"childPID":1234,"generation":0,"operation":"backup"}')],
 ["UTF8",Buffer.from([0xff])],["oversized",Buffer.alloc(193)],["wrong PID",Buffer.from(JSON.stringify({...request,childPID:1}))],
 ["wrong generation",Buffer.from(JSON.stringify({...request,generation:1}))],
 ["unknown operation",Buffer.from(JSON.stringify({...request,operation:"clear"}))],
 ["operation conflicts",Buffer.from(JSON.stringify({...request,operation:"restore"}))]])
 test("strict request rejects "+name,()=>assert.throws(()=>decodeRecoveryRequest(bytes,childPID,0)));
test("matching receipt permits one exclusive mailbox write",t=>{
 const root=directory(t);requestRecovery(port,request);
 assert.deepEqual(JSON.parse(readFileSync(join(root,".e2e/fixtures",port+".restart"),"utf8")),request);
 assert.throws(()=>requestRecovery(port,request));
 assert.deepEqual(JSON.parse(readFileSync(join(root,".e2e/fixtures",port+".restart"),"utf8")),request);
});
test("wrong receipt child rejects with no control write",t=>{
 const root=directory(t);assert.throws(()=>requestRecovery(port,{...request,childPID:5678}));
 assert.equal(existsSync(join(root,".e2e/fixtures",port+".restart")),false);
});
test("symlink parent rejects before a mailbox write",t=>{
 const root=directory(t);renameSync(join(root,".e2e/fixtures"),join(root,"captured"));
 symlinkSync(join(root,"captured"),join(root,".e2e/fixtures"),"dir");
 assert.throws(()=>requestRecovery(port,request));assert.equal(existsSync(join(root,"captured",port+".restart")),false);
});
test("substituted parent cannot redirect the actual exclusive write",t=>{
 const root=directory(t),original=fs.openSync;
 let swapped=false;
 fs.openSync=(path,...args)=>{
  if(!swapped && String(path).endsWith(".restart")){
   swapped=true;renameSync(join(root,".e2e/fixtures"),join(root,"captured"));mkdirSync(join(root,".e2e/fixtures"));
  }
  return original(path,...args);
 };
 syncBuiltinESMExports();
 try{assert.throws(()=>requestRecovery(port,request));}
 finally{fs.openSync=original;syncBuiltinESMExports();}
 assert.equal(existsSync(join(root,".e2e/fixtures",port+".restart")),false,"foreign directory got a control file");
 assert.equal(existsSync(join(root,"captured",port+".restart")),false,"rejected control must be removed from captured directory");
});
for(const [name,raw] of [
 ["duplicate",'{"app":"player","app":"player","port":"49131","supervisorPID":123,"childPID":1234,"generation":0}'],
 ["unknown",JSON.stringify({app:"player",port,supervisorPID:123,childPID,generation:0,extra:true})],
 ["oversized"," ".repeat(4097)],["wrong port",JSON.stringify({app:"player",port:"49132",supervisorPID:123,childPID,generation:0})]])
 test("receipt rejects "+name+" before control effects",t=>{
  const root=directory(t);writeFileSync(join(root,".e2e/fixtures",port+".json"),raw);
  assert.throws(()=>readFixtureReceipt(port));assert.throws(()=>requestRecovery(port,request));
  assert.equal(existsSync(join(root,".e2e/fixtures",port+".restart")),false);
 });

test("unknown toJSON cannot bypass request key admission",t=>{
 const root=directory(t);let serialized=0;
 const input={...request,toJSON(){serialized++;return request;}};
 assert.throws(()=>requestRecovery(port,input));assert.equal(serialized,0);
 assert.equal(existsSync(join(root,".e2e/fixtures",port+".restart")),false);
});

test("lost own mailbox preserves original callback failure without unlinking foreign files",t=>{
 const root=directory(t),original=fs.writeFileSync;
 fs.writeFileSync=(fd,...args)=>{fs.unlinkSync(join(root,".e2e/fixtures",port+".restart"));throw new Error("owned callback failure");};
 syncBuiltinESMExports();
 try{assert.throws(()=>requestRecovery(port,request),/owned callback failure/);}
 finally{fs.writeFileSync=original;syncBuiltinESMExports();}
 assert.equal(existsSync(join(root,".e2e/fixtures",port+".restart")),false);
});

for(const input of [undefined,"","049131","../49131","80","65536","x".repeat(4097)])
 test("invalid fixture port has no mailbox effects: "+String(input).slice(0,20),t=>{
  const root=directory(t);assert.throws(()=>readFixtureReceipt(input));assert.throws(()=>requestRecovery(input,request));
  assert.equal(existsSync(join(root,".e2e/fixtures",port+".restart")),false);
 });
test("recovery evidence conflicts with initial generation before any control write",t=>{
 const root=directory(t);
 writeFileSync(join(root,".e2e/fixtures",port+".json"),JSON.stringify({app:"player",port,supervisorPID:123,childPID,generation:0,
  recovery:{operation:"backup",verified:true,archiveSHA256:"a".repeat(64),archiveBytes:18}}));
 assert.throws(()=>readFixtureReceipt(port));assert.throws(()=>requestRecovery(port,request));
 assert.equal(existsSync(join(root,".e2e/fixtures",port+".restart")),false);
});
