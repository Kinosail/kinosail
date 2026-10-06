import {readFile} from "node:fs/promises";
import {expect,test} from "@playwright/test";

const script=await readFile(new URL("../../subtitles/internal/server/static/subtitle-dock-initial.js",import.meta.url),"utf8");

test("Subtitle focus reveal leaves unrelated keys and unchanged or external focus without scroll",async({page})=>{
  await page.setViewportSize({width:320,height:800});
  await page.setContent(`<style>body{margin:0}.app-header{position:fixed;top:0;height:100px}nav{position:fixed;bottom:0;height:160px;width:100%}main{height:2500px}.target{position:absolute;top:1400px}</style><body class="subtitle-dashboard"><header class="app-header"><nav data-subtitle-dock></nav></header><main><button id="start">Start</button><button id="inside" class="target">Inside</button></main><button id="outside" class="target">Outside</button></body>`);
  await page.addScriptTag({content:script});
  for(const control of ["unchanged","non-tab","ctrl","meta","prevented","outside"]) {
    const result=await page.evaluate(control=>new Promise<{before:number,after:number}>(resolve=>{
      const start=document.getElementById("start")!,target=document.getElementById(control==="outside"?"outside":"inside")!;
      start.focus({preventScroll:true});scrollTo(0,0);const before=scrollY;
      const move=(event:KeyboardEvent)=>{if(control!=="unchanged")target.focus({preventScroll:true});if(control==="prevented")event.preventDefault();};
      document.addEventListener("keydown",move,{once:true});
      document.dispatchEvent(new KeyboardEvent("keydown",{key:control==="non-tab"?"x":"Tab",ctrlKey:control==="ctrl",metaKey:control==="meta",cancelable:true,bubbles:true}));
      requestAnimationFrame(()=>requestAnimationFrame(()=>resolve({before,after:scrollY})));
    }),control);
    expect(result.after,control).toBe(result.before);
  }
});
