import type { Page, Frame, CDPSession } from "@playwright/test";
import { installFetchWitness, equalDiagnosticHeaders } from "./subtitle-restore-causal-witness.mjs";
import { requestFailureCode } from "./subtitle-restore-recovery-network";
import { bounded, pause, type JSONValue } from "./subtitle-restore-recovery-fixture";
import { validCausal, validNative, validServer, type Diagnostic, type Mode, type Native, type Network } from "./subtitle-restore-causal-schema";

type Witness = { read: () => Record<Mode | "restore", Native> & { valid: boolean; completion: {requestMatched:boolean;responseMatched:boolean} }; probe: (mode: Mode) => Promise<Response>; cancelHeld: () => void; close: () => void };
type OwnedWindow = Window & { __r06CausalWitness: Witness };
const emptyNative = (): Native => ({outcome:"unreached",status:0,signalPresent:false,signalAborted:false,pagehide:false});
const emptyNetwork = (): Network => ({terminal:"unreached",failureCode:"none",resourceType:"unreached",cancelled:false,navigation:false});
const emptyServer = () => ({seen:false,held:false,delivered:false,cancelled:false,timedOut:false,settled:false});
export function observeCausal(page: Page, origin: string, item: string, caseEnd: number, setupEnd: number, caseID: string) {
  let session: CDPSession | undefined, allocation: Promise<CDPSession> | undefined, installationStarted = false;
  const keys = ["direct","captured","held","restore"] as const;
  const rows = Object.fromEntries(keys.map(key => [key,emptyNetwork()])) as Record<Mode | "restore",Network>;
  const matched = new Map<string,Mode | "restore">(), responseHeaders = new Map<Mode,(string | null)[]>();
  const result: Diagnostic & {schema:"r06-causal-v2"} = {schema:"r06-causal-v2",valid:true,reason:"none",headersEqual:false,framingEqual:false,stopped:false,
    probes:["direct","captured","held"].map(mode => ({mode:mode as Mode,native:emptyNative(),network:emptyNetwork(),server:emptyServer()})),
    restoreNative:emptyNative(),restoreNetwork:emptyNetwork(),completion:{scope:"r06-native-bodyless-204-v1",caseID,browserVersion:"153.0.8010.12",
      pinnedBrowserMatched:false,requestMatched:false,responseMatched:false,bodyless:false,fixtureMatched:false}};
  let closure: Promise<Diagnostic> | undefined;
  const invalidate = (reason: Diagnostic["reason"]) => {result.valid=false;if(result.reason==="none")result.reason=reason;};
  let eventCount=0;
  const event = () => {if(++eventCount>8)invalidate("overflow");};
  const request = (value: {requestId:string;type?:string;request:{url:string;method:string;postData?:string}}) => {
    if (typeof value.request.url!=="string" || value.request.url.length>2048 || value.request.method!=="POST") return;
    let key: Mode | "restore" | undefined;
    if(value.request.url===origin+"/api/v1/subtitle-library/"+item+"/restore")key="restore";
    else if(value.request.url===origin+"/__r06_restore/probe" && typeof value.request.postData==="string" && value.request.postData.length<=256){
      for(const mode of ["direct","captured","held"] as const)if(value.request.postData===JSON.stringify({mode}))key=mode;
    }
    if(!key)return;
    if(matched.size>=4 || rows[key].terminal!=="unreached" || typeof value.requestId!=="string" || value.requestId.length>128){invalidate("overflow");return;}
    matched.set(value.requestId,key);rows[key].terminal="pending";
    rows[key].resourceType=["Fetch","XHR","Document"].includes(value.type||"")?value.type as Network["resourceType"]:"Other";
  };
  const response = (value:{requestId:string;response:{status:number;headers:Record<string,string>}}) => {
    const key=matched.get(value.requestId);if(key!=="direct"&&key!=="captured")return;
    const fields=["cache-control","content-type","date","content-length","transfer-encoding","connection"];
    const headers:(string | null)[]=fields.map(name=>{
      const entry=Object.entries(value.response.headers).find(([key])=>key.toLowerCase()===name);
      if(entry && (typeof entry[1]!=="string" || entry[1].length>128)){invalidate("overflow");return "invalid";}
      return entry?entry[1]:null;
    });
    if(value.response.status!==204)invalidate("mismatch");responseHeaders.set(key,headers);
  };
  const finished = (value:{requestId:string}) => {
    const key=matched.get(value.requestId);if(!key)return;event();
    if(rows[key].terminal!=="pending"){invalidate("mismatch");return;}rows[key].terminal="finished";
  };
  const failed = (value:{requestId:string;errorText:string;canceled?:boolean}) => {
    const key=matched.get(value.requestId);if(!key)return;event();
    if(rows[key].terminal!=="pending"){invalidate("mismatch");return;}
    rows[key].terminal="request-failed";rows[key].cancelled=value.canceled===true;
    if(typeof value.errorText!=="string"||value.errorText.length>256)invalidate("overflow");
    rows[key].failureCode=requestFailureCode(typeof value.errorText==="string"&&value.errorText.length<=256?value.errorText:undefined);
  };
  const navigated=(frame:Frame)=>{if(frame.parentFrame()===null)for(const row of Object.values(rows))if(row.terminal==="pending")row.navigation=true;};
  const remaining = (end: number, cap = 1000) => {const n=end-performance.now();if(n<=0)throw Error("fixed-diagnostic-boundary");return Math.min(n,cap);};
  const start = async () => {
    try {
      const browser=page.context().browser();
      result.completion.pinnedBrowserMatched=browser?.browserType().name()==="chromium"&&browser.version()==="153.0.8010.12";
      const pending=page.context().newCDPSession(page); allocation=pending;
      void pending.then(value=>{session=value;},()=>{});
      session=await bounded(pending,remaining(setupEnd,2000));
      session.on("Network.requestWillBeSent",request);session.on("Network.responseReceived",response);
      session.on("Network.loadingFinished",finished);session.on("Network.loadingFailed",failed);page.on("framenavigated",navigated);
      await bounded(session.send("Network.enable"),remaining(setupEnd));
      installationStarted=true;
      await bounded(page.evaluate(installFetchWitness,{origin,item}),remaining(setupEnd));
    } catch {invalidate("timeout");throw Error("fixed-diagnostic-boundary");}
  };
  const server = async (end: number) => {
    const response=await bounded(page.request.get(origin+"/__r06_restore/probe-witness",{timeout:remaining(end)}),remaining(end));
    if(response.status()!==200)throw Error("fixed-diagnostic-boundary");
    const raw=await bounded(response.body(),remaining(end));if(raw.length>2048)throw Error("fixed-diagnostic-boundary");
    const value:JSONValue=JSON.parse(raw.toString("utf8"));
    if(!Array.isArray(value)||value.length!==3||!value.every(validServer))throw Error("fixed-diagnostic-boundary");
    return value;
  };
  const witness = async (end: number) => {
    const value=await bounded(page.evaluate(()=>(window as OwnedWindow).__r06CausalWitness.read()),remaining(end));
    if(typeof value.valid!=="boolean" || !keys.every(key=>validNative(value[key])) || !value.completion ||
      Object.keys(value.completion).sort().join(",")!=="requestMatched,responseMatched" ||
      typeof value.completion.requestMatched!=="boolean" || typeof value.completion.responseMatched!=="boolean")throw Error("fixed-diagnostic-boundary");
    if(!value.valid)invalidate("mismatch");return value;
  };
  const probe = async () => {
    const end=Math.min(caseEnd,setupEnd,performance.now()+10000);
    try{
      for(const mode of ["direct","captured","held"] as const){
        const deadline=Math.min(end,performance.now()+(mode==="held"?5000:2000));
        await bounded(page.evaluate(mode=>{void(window as OwnedWindow).__r06CausalWitness.probe(mode).then(()=>{},()=>{});},mode),deadline-performance.now());
        if(mode==="held"){
          while(!(await server(deadline))[2].held && performance.now()<deadline)await pause(20);
          if(!(await server(deadline))[2].held)throw Error("fixed-diagnostic-boundary");
          await bounded(page.evaluate(()=>(window as OwnedWindow).__r06CausalWitness.cancelHeld()),remaining(deadline));
        }
        while(performance.now()<deadline){
          const native=await witness(deadline),current=await server(deadline);
          const index=["direct","captured","held"].indexOf(mode);
          if(!["unreached","pending"].includes(rows[mode].terminal)&&!["unreached","pending"].includes(native[mode].outcome)&&current[index].settled)break;
          await pause(20);
        }
        if(performance.now()>=deadline)throw Error("fixed-diagnostic-boundary");
      }
      const direct=responseHeaders.get("direct"),captured=responseHeaders.get("captured");
      result.headersEqual=!!direct&&!!captured&&equalDiagnosticHeaders(direct.slice(0,3),captured.slice(0,3));
      result.framingEqual=!!direct&&!!captured&&equalDiagnosticHeaders(direct.slice(3),captured.slice(3));
      if(!result.headersEqual||!result.framingEqual)invalidate("mismatch");
    }catch{invalidate("timeout");}
  };
  const close = async (cleanupEnd: number) => {
    try{
      let native=await witness(cleanupEnd);
      while(result.valid&&(["unreached","pending"].includes(native.restore.outcome)||["unreached","pending"].includes(rows.restore.terminal))){
        await pause(remaining(cleanupEnd,20));
        native=await witness(cleanupEnd);
      }
      const current=await server(cleanupEnd);
      for(const [index,row]of result.probes.entries()){row.native=native[row.mode];row.network={...rows[row.mode]};row.server=current[index];}
      result.restoreNative=native.restore;result.restoreNetwork={...rows.restore};
      Object.assign(result.completion,native.completion);
      const response=await bounded(page.request.get(origin+"/__r06_restore/completion-witness",{headers:{"X-R06-Item":item},timeout:remaining(cleanupEnd)}),remaining(cleanupEnd));
      if(response.status()!==200)throw Error("fixed-diagnostic-boundary");
      const raw=await bounded(response.body(),remaining(cleanupEnd));if(raw.length>2048)throw Error("fixed-diagnostic-boundary");
      const body:JSONValue=JSON.parse(raw.toString("utf8"));
      if(body===null||typeof body!=="object"||Array.isArray(body)||Object.keys(body).sort().join(",")!=="bodyless,matched"||
        typeof body.bodyless!=="boolean"||typeof body.matched!=="boolean")throw Error("fixed-diagnostic-boundary");
      result.completion.bodyless=body.bodyless;result.completion.fixtureMatched=body.matched;
      if(!["fulfilled","rejected"].includes(native.restore.outcome)||!["finished","request-failed"].includes(rows.restore.terminal)||
        !["Fetch","XHR"].includes(rows.restore.resourceType)||native.restore.signalAborted&&!native.restore.signalPresent||
        native.restore.outcome==="fulfilled"&&native.restore.status<100||native.restore.outcome==="rejected"&&native.restore.status!==0)invalidate("control");
      const held=result.probes[2];
      if(held.native.outcome!=="rejected"||!held.native.signalPresent||!held.native.signalAborted||held.native.pagehide||
        held.network.terminal!=="request-failed"||held.network.failureCode!=="aborted"||!held.network.cancelled||held.network.navigation||
        !held.server.seen||!held.server.held||!held.server.cancelled||held.server.delivered||held.server.timedOut||!held.server.settled)invalidate("control");
      for(const row of result.probes.slice(0,2))if(row.native.outcome!=="fulfilled"||row.native.status!==204||row.native.signalPresent||row.native.signalAborted||row.native.pagehide||
        row.network.navigation||!["Fetch","XHR"].includes(row.network.resourceType)||!row.server.seen||!row.server.delivered||row.server.cancelled||row.server.timedOut||!row.server.settled)invalidate("control");

    }catch{invalidate("timeout");}
    let wrapperStopped=!installationStarted, detached=false;
    if(installationStarted)try{await bounded(page.evaluate(()=>(window as OwnedWindow).__r06CausalWitness.close()),remaining(cleanupEnd));wrapperStopped=true;}catch{invalidate("control");}
    try{if(!session&&allocation)session=await bounded(allocation,remaining(cleanupEnd));}catch{invalidate("control");}
    session?.off("Network.requestWillBeSent",request);session?.off("Network.responseReceived",response);
    session?.off("Network.loadingFinished",finished);session?.off("Network.loadingFailed",failed);page.off("framenavigated",navigated);
    try{if(session)await bounded(session.detach(),remaining(cleanupEnd));detached=!!session||!allocation;}catch{invalidate("control");}
    result.stopped=wrapperStopped&&detached;
    if(!result.stopped)invalidate("control");
    if(!validCausal(result)||Buffer.byteLength(JSON.stringify(result))>131072)throw Error("fixed-diagnostic-boundary");
    return result;
  };
  const finish=(end:number)=>{closure??=close(end);return closure;};
  return {start,probe,finish,invalid:()=>{invalidate("control");return result;}};
}
