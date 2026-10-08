// The pinned SDK preserves COMMAND_FAILED causes but omits their output from reports.
function observation(error) {
 const unknown={stage:'install_app',category:'unqualified'};
 const seen=new Set();
 try {
  for(let depth=0;depth<8 && error && typeof error==='object';depth++,error=error.cause){
   if(seen.has(error))return unknown;
   seen.add(error);
   if(error.code!=='COMMAND_FAILED')continue;
   const data=error.details;
   if(!data || typeof data!=='object' || Array.isArray(data) || Object.keys(data).length!==3 || !['stdout','stderr','exitCode'].every(key=>Object.hasOwn(data,key)))return unknown;
   if(!Number.isInteger(data.exitCode) || data.exitCode===0 || data.exitCode < -128 || data.exitCode > 255)return unknown;
   for(const [field,max] of [['stdout',65536],['stderr',8192]]){
    if(typeof data[field]!=='string' || data[field].length>max || !data[field].isWellFormed() || Buffer.byteLength(data[field])>max)return unknown;
   }
   return {stage:'install_app',category:'command_failed',exitCode:data.exitCode,stdoutBytes:Buffer.byteLength(data.stdout),stderrBytes:Buffer.byteLength(data.stderr),packageAbsentText:/unknown package|not installed/i.test(data.stdout+'\n'+data.stderr)};
  }
 } catch {}
 return unknown;
}
export async function installPhone(device) {
 try {
  await device.installApp(undefined,{reinstall:true});
 } catch(error) {
  // Diagnostic assignment can fail on opaque/frozen SDK errors; original failure wins.
  try { error.details={...error.details,observed:JSON.stringify(observation(error))}; } catch {}
  throw error;
 }
}
