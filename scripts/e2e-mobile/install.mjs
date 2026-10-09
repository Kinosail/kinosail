// The pinned SDK preserves COMMAND_FAILED causes but omits their output from reports.
function observation(error) {
 const unknown=reason=>({stage:'install_app',category:'unqualified',reason});
 const seen=new Set();
 try {
  for(let depth=0;depth<8 && error && typeof error==='object';depth++,error=error.cause){
   if(seen.has(error))return unknown('cycle');
   seen.add(error);
   if(error.code!=='COMMAND_FAILED')continue;
   let data=error.details;
   const strings=['hint','diagnosticId','logPath','logPathUnavailable','supportedOn'];
   const keys=['stdout','stderr','exitCode',...strings,'diagnosticsRecord','retriable','processExitError'];
   if(!data || typeof data!=='object' || Array.isArray(data))return unknown('invalid_fields');
   const descriptors=Object.getOwnPropertyDescriptors(data),present=Reflect.ownKeys(descriptors);
   if(present.length>keys.length || present.some(key=>!keys.includes(key)))return unknown('unknown_fields');
   if(present.some(key=>!Object.hasOwn(descriptors[key],'value')))return unknown('opaque');
   data=Object.fromEntries(present.map(key=>[key,descriptors[key].value]));
   if(!['stdout','stderr','exitCode'].some(key=>Object.hasOwn(data,key) && data[key]!==undefined))return unknown('missing_fields');
   const text=value=>typeof value==='string' && value.length<=2048 && value.isWellFormed() && Buffer.byteLength(value)<=2048;
   if(strings.some(key=>data[key]!==undefined && !text(data[key])))return unknown('invalid_fields');
   if(['retriable','processExitError'].some(key=>data[key]!==undefined && typeof data[key]!=='boolean'))return unknown('invalid_fields');
   if(data.diagnosticsRecord!==undefined){
    const record=data.diagnosticsRecord;
    if(!record || typeof record!=='object' || Array.isArray(record))return unknown('invalid_fields');
    const fields=Object.getOwnPropertyDescriptors(record);
    if(Reflect.ownKeys(fields).length!==2 || !['session','requestId'].every(key=>text(fields[key]?.value) && fields[key].value.length>0))return unknown('invalid_fields');
   }
   if(data.exitCode!==undefined && data.exitCode!==null && (!Number.isInteger(data.exitCode) || data.exitCode===0 || data.exitCode < -128 || data.exitCode > 255))return unknown('invalid_fields');
   for(const [field,max] of [['stdout',65536],['stderr',8192]]){
    if(data[field]!==undefined && (typeof data[field]!=='string' || data[field].length>max || !data[field].isWellFormed() || Buffer.byteLength(data[field])>max))return unknown('invalid_fields');
   }
   return {stage:'install_app',category:'command_failed',exitCode:data.exitCode??null,stdoutBytes:data.stdout===undefined?null:Buffer.byteLength(data.stdout),stderrBytes:data.stderr===undefined?null:Buffer.byteLength(data.stderr),packageAbsentText:/unknown package|not installed/i.test((data.stdout??'')+'\n'+(data.stderr??''))};
  }
 } catch {return unknown('opaque');}
 return unknown(error && typeof error==='object'?'depth_limit':'unsupported_error');
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
