import {expect,test} from "@playwright/test";
import {createServer} from "node:http";
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
 const server=createServer((request,response)=>{response.setHeader("Content-Type","text/html");response.end(markup(new URL(request.url!,"http://localhost").searchParams.get("q")||""));});
 await new Promise<void>(resolve=>server.listen(0,"127.0.0.1",resolve));
 const address=server.address() as {port:number};
 try {
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
