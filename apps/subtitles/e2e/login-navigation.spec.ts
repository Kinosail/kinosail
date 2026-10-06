import {expect,test} from "@playwright/test";
import {login as dashboardLogin} from "./subtitle-dashboard-helpers";
import {login as instanceLogin, loginViewer, createViewer, removeViewer} from "./test-instance-helpers";

const logins=[
  ["dashboard",dashboardLogin],
  ["test-instance",instanceLogin],
  ["Viewer",(page: import("@playwright/test").Page)=>loginViewer(page,"Navigation Viewer","navigation-viewer-password")],
] as const;
for(const [name,login] of logins) test(`The ${name} login submits real credentials while a decorative response is pending`,async ({page,browser},info)=>{
  let viewerID:string|undefined,owner:import("@playwright/test").Page|undefined;
  if(name==="Viewer") {
    await instanceLogin(page);
    viewerID=await createViewer(page,"Navigation Viewer","navigation-viewer-password");
    owner=page;
    const context=await browser.newContext({baseURL:new URL(page.url()).origin});
    page=await context.newPage();
  }
  let release!:()=>void,completed!:()=>void;
  const held=new Promise<void>(resolve=>{release=resolve;}),finished=new Promise<void>(resolve=>{completed=resolve;});
  let responseStatus:number|undefined;
  await page.route("**/static/cinema-backdrop.jpg?login-navigation-test",async route=>{
    try {const response=await route.fetch();responseStatus=response.status();await held;await route.fulfill({response});}
    finally {completed();}
  });
  await page.addInitScript(()=>addEventListener("DOMContentLoaded",()=>{
    if(location.pathname!=="/login")return;
    const image=new Image();image.alt="";image.src="/static/cinema-backdrop.jpg?login-navigation-test";document.body.append(image);
  },{once:true}));
  page.on("request",request=>{if(request.method()==="POST"&&new URL(request.url()).pathname==="/login")release();});
  let timer:ReturnType<typeof setTimeout>|undefined;
  try {
    await Promise.race([login(page,info),new Promise((_,reject)=>{timer=setTimeout(()=>reject(new Error("Usable login must not await the decorative response")),12000);})]);
    await finished;
    expect(responseStatus).toBe(200);
    await expect(page).not.toHaveURL(/\/login(?:\?|$)/);
    expect(await page.evaluate(async()=> (await fetch("/api/v1/me")).status)).toBe(200);
  } finally {
    clearTimeout(timer);release();
    if(owner&&viewerID) {await page.context().close();await removeViewer(owner,viewerID);}
  }
});
