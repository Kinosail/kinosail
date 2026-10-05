import {expect,test} from "@playwright/test";
import {login as dashboardLogin} from "./subtitle-dashboard-helpers";
import {login as instanceLogin} from "./test-instance-helpers";

for(const [name,login] of [["dashboard",dashboardLogin],["test-instance",instanceLogin]] as const) test(`The ${name} login submits real credentials while a decorative response is pending`,async ({page},info)=>{
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
  } finally {clearTimeout(timer);release();}
});
