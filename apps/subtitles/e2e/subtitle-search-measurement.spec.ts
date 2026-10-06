import {expect,test} from "@playwright/test";
import {createServer,request as httpRequest} from "node:http";
import {readFile} from "node:fs/promises";
// Root measurement support is used by hosted layout gates, not product code.
// @ts-expect-error JavaScript measurement support has no TypeScript declaration.
import {measureSubtitleSearch,subtitleSearchGeometry,subtitleSearchUnchanged} from "../../../scripts/testing/layout-stability-subtitle-search.mjs";
const source=await readFile(new URL("../internal/server/static/subtitle-status.js",import.meta.url),"utf8");

// Real sticky DOM plus the shipped native search controller. Only response HTML
// is a local external fixture; pending work remains held by the actual helper.
test("sticky native search measures settled input actionability @smoke",async({page},testInfo)=>{
 await page.setViewportSize({width:320,height:800});
 const markup=(query="")=>`<!doctype html><html lang="en"><style>
 html{font-size:200%}body{margin:0;width:320px;min-height:4000px}.app-header{position:sticky;top:0;height:295.390625px;background:#222;z-index:2}
 .lead{height:1008.03125px}#subtitle-list-title{height:57.59375px;margin:0}.gap{height:163.9375px}
 .subtitle-filters{position:sticky;top:72px;height:308.78125px;background:#333}input{display:block;position:relative;top:240px;height:60px;width:200px}
 #subtitle-content{height:1665.625px;margin-left:40px;width:240px}#subtitle-feedback{position:fixed;bottom:0}
 </style><body class="subtitle-dashboard"><header class="app-header"></header><main id="main" data-view="library" data-loading-label="Loading" data-error-label="Search failed" data-retry-label="Reload view">
 <div class="lead"></div><h2 id="subtitle-list-title">Library</h2><div class="gap"></div><form class="subtitle-filters" data-subtitle-search><input type="hidden" name="view" value="library"><input id="subtitle-search" name="q" value="${query}"></form>
 <div id="subtitle-content">${query && query!=="Layout"?'<div class="subtitle-empty"><h3>No files match.</h3></div>':'<details class="subtitle-file"><summary>Layout Example</summary><div class="subtitle-file-detail">Details</div></details>'}</div><div id="subtitle-feedback" hidden></div></main></body></html>`;
 const server=createServer((request,response)=>{
  const reject=()=>{response.writeHead(400,{"Content-Type":"text/plain"});response.end("invalid fixture request");};
  if(request.method!=="GET"||typeof request.url!=="string"||request.url.length>1024||
   !request.url.startsWith("/?")||/%(?![0-9a-f]{2})/i.test(request.url)){reject();return;}
  const url=new URL(request.url,"http://localhost"),query=url.searchParams.get("q")??"";
  if(url.origin!=="http://localhost"||url.pathname!=="/"||url.searchParams.getAll("view").length!==1||url.searchParams.get("view")!=="library"||
   url.searchParams.getAll("q").length>1||[...url.searchParams.keys()].some(key=>key!=="view"&&key!=="q")||
   !["","Layout","Missing","no-synthetic-match"].includes(query)){reject();return;}
  // Render only fixed literals. No HTTP value enters the HTML string.
  const literal=query==="Layout"?"Layout":query==="Missing"?"Missing":query==="no-synthetic-match"?"no-synthetic-match":"";
  response.setHeader("Content-Type","text/html");response.end(markup(literal));
 });
 await new Promise<void>(resolve=>server.listen(0,"127.0.0.1",resolve));
 const address=server.address() as {port:number};
 try {
 const base=`http://127.0.0.1:${address.port}`;
 for(const target of ["http://[/?view=library","//[/?view=library"]){
  const denied=await new Promise<{status:number,body:string}>((resolve,reject)=>{
   const call=httpRequest(base,{path:target},response=>{
    let body="";response.on("data",chunk=>{body+=chunk;});
    response.on("end",()=>resolve({status:response.statusCode!,body}));
   });call.on("error",reject);call.end();
  });
  expect(denied).toEqual({status:400,body:"invalid fixture request"});
 }
 for(const [path,method] of [
  ["/?view=library&q=%22%3E%3Csvg%20onload%3Dalert(1)%3E","GET"],
  ["/?view=library&q=unknown","GET"],["/?view=library&q=Layout&q=Missing","GET"],
  ["/?view=library&view=movies","GET"],["/?view=unknown","GET"],["/?q=Layout","GET"],
  ["/?view=library&unknown=Layout","GET"],["/?view=library&q=%","GET"],
  ["/?view=library&q="+"x".repeat(2049),"GET"],["/unknown?view=library","GET"],
  ["/?view=library","POST"],
 ]){
  const denied=await page.request.fetch(base+path,{method});
  expect(denied.status(),path.slice(0,100)).toBe(400);
  expect(await denied.text()).toBe("invalid fixture request");
  expect(denied.headers()["content-type"]).toBe("text/plain");
 }
 await page.goto(`http://127.0.0.1:${address.port}/?view=library`);
 await page.addScriptTag({content:source});
 await page.evaluate(()=>{window.scrollTo(0,1555);document.querySelector<HTMLInputElement>("input#subtitle-search")!.focus({preventScroll:true});});
 await page.locator(".subtitle-file summary").evaluate(node=>node.scrollIntoView({block:"center",behavior:"instant"}));
 const results:any[]=[],probe:any={};
 try {await measureSubtitleSearch(page,{width:320,height:800},results,probe);} catch(error){await testInfo.attach("probe",{contentType:"application/json",body:JSON.stringify(probe)});throw error;}
 for(const result of results){expect(result.stable,result.state||result.flow).toBe(true);if(result.flow==="subtitle-native-search"){expect(result.pendingStable,result.state).toBe(true);if(result.injectedFailure)expect(result.failureRetainsContent).toBe(true);}}
 const settled=await subtitleSearchGeometry(page);
 for(const css of ["#subtitle-content{width:242px !important}","#subtitle-content{height:1667.625px !important}","#subtitle-list-title{margin-top:2px !important}"]){
  const style=await page.addStyleTag({content:css});
  expect(subtitleSearchUnchanged(settled,await subtitleSearchGeometry(page)),css).toBe(false);
  await style.evaluate(node=>node.remove());
 }
 await testInfo.attach("sticky-search-pending-and-movement",{contentType:"application/json",body:JSON.stringify({viewport:{width:320,height:800,fontSize:"200%"},results,movementControls:3})});
 } finally {await new Promise<void>(resolve=>server.close(()=>resolve()));}
});
