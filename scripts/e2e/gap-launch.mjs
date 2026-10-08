import {constants,openSync,closeSync,fstatSync,lstatSync,readFileSync,readSync,writeFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {spawnSync} from 'node:child_process';
import {pathToFileURL} from 'node:url';

const validRun=run=>typeof run==='string'&&/^\d{8}T\d{6}Z-[1-9]\d{0,9}$/.test(run);
const command=run=>['xvfb-run','-a','pnpm','exec','e2e','run','--config','public-flow-gap.config.ts','--headed','--target','player','--output','.e2e/runs/'+run+'/gap-runner'];
const configSHA=()=>createHash('sha256').update(readFileSync('public-flow-gap.config.ts')).digest('hex');
function directory(run){
 if(!validRun(run))throw new Error('invalid headed SDK run');
 const paths=['.e2e','.e2e/runs','.e2e/runs/'+run];
 const stats=paths.map(path=>{const s=lstatSync(path);if(s.isSymbolicLink()||!s.isDirectory())throw new Error('invalid SDK run parent');return s;});
 return ()=>paths.every((path,i)=>{const s=lstatSync(path);return !s.isSymbolicLink()&&s.dev===stats[i].dev&&s.ino===stats[i].ino;});
}
export function readGapLaunch(run){
 const owned=directory(run),path='.e2e/runs/'+run+'/gap-launch.json';
 const fd=openSync(path,constants.O_RDONLY|constants.O_NOFOLLOW|constants.O_NONBLOCK);
 try{
  const stat=fstatSync(fd);if(!stat.isFile()||stat.size<1||stat.size>4096||!owned())throw new Error('invalid headed SDK receipt');
  const buffer=Buffer.alloc(4097),size=readSync(fd,buffer,0,buffer.length,null);
  if(size!==stat.size||size>4096)throw new Error('invalid headed SDK receipt');
  const bytes=buffer.subarray(0,size);
  const raw=new TextDecoder('utf-8',{fatal:true}).decode(bytes);let value;try{value=JSON.parse(raw);}catch{throw new Error('invalid headed SDK receipt');}
  if(!value||Array.isArray(value)||raw!==JSON.stringify(value)||Object.keys(value).sort().join(',')!=='command,configSHA256,run,schema'
     ||value.schema!==1||value.run!==run||value.configSHA256!==configSHA()||JSON.stringify(value.command)!==JSON.stringify(command(run)))throw new Error('invalid headed SDK receipt');
  const final=lstatSync(path);if(final.isSymbolicLink()||stat.dev!==final.dev||stat.ino!==final.ino||!owned())throw new Error('headed SDK receipt changed');
  return value;
 }finally{closeSync(fd);}
}
function main(){
 const [run,...extra]=process.argv.slice(2);if(extra.length||!validRun(run)){process.stderr.write('invalid headed SDK run\n');return 2;}
 const owned=directory(run),args=command(run);
 const fd=openSync('.e2e/runs/'+run+'/gap-launch.json',constants.O_WRONLY|constants.O_CREAT|constants.O_EXCL|constants.O_NOFOLLOW,0o600);
 try{if(!owned())throw new Error('SDK run parent changed');writeFileSync(fd,JSON.stringify({schema:1,run,command:args,configSHA256:configSHA()}));if(!owned())throw new Error('SDK run parent changed');}finally{closeSync(fd);}
 const result=spawnSync(args[0],args.slice(1),{stdio:'inherit',env:{...process.env,KINOSAIL_E2E_GAP_RUN:run},timeout:300000});
 return Number.isInteger(result.status)&&result.status>=0&&result.status<=255?result.status:1;
}
if(process.argv[1]&&import.meta.url===pathToFileURL(process.argv[1]).href){try{process.exitCode=main();}catch{process.stderr.write('headed SDK launch failed\n');process.exitCode=1;}}
