import {installLayoutFailureReporter, layoutFailureLocations, layoutFailureNavigation} from "./layout-stability-failure.mjs";
import {layoutResponseHandler} from "./layout-stability-routing.mjs";
import {navigationDiagnostics} from "./navigation-diagnostics.mjs";
import {createRequire} from "node:module";
import {writeFile} from "node:fs/promises";
import {join} from "node:path";
import {createHmac} from "node:crypto";
import {measureFlows} from "./layout-stability-flows.mjs";
import {bookmarkSnapshot} from "./layout-stability-bookmarks.mjs";
const require = createRequire(new URL("../../apps/player/e2e/package.json", import.meta.url));
const {chromium, webkit, firefox} = require("@playwright/test");
const AxeBuilder = require("@axe-core/playwright").default;
const baseURL = process.env.KINOSAIL_E2E_URL, app = process.env.KINOSAIL_LAYOUT_APP, run = process.env.KINOSAIL_LAYOUT_RUN;
const engine = process.env.KINOSAIL_LAYOUT_BROWSER || "chromium";
let phase = "browser-launch", operationPhase, activePage, browser, authContext, activeCase, navigation;
const loginResponses = [];
let requestStatus, requestContentType;
const reports = [], flows = [], flowProbe = {stage: "not-started"};
installLayoutFailureReporter(async error => {
  const currentNavigation = await layoutFailureNavigation(phase, flowProbe, navigation, error);
  return {result: "failed", app, engine, stage: phase, operationPhase: phase === "measure-flows" ? flowProbe.operationPhase : operationPhase, activeCase: phase === "measure-flows" ? undefined : activeCase, flowElapsedMs: flowProbe.elapsedMs, flowStage: flowProbe.stage,
    locations: layoutFailureLocations(error),
    errorClass: ["TimeoutError", "TypeError", "ReferenceError", "SyntaxError"].includes(error.name) ? error.name : "Error",
    completedCases: reports.length, completedFlows: flows.length, media: flowProbe.media, probe: flowProbe.geometry, loginResponses,
    requestStatus: phase === "measure-flows" ? undefined : requestStatus, requestContentType: phase === "measure-flows" ? undefined : requestContentType, navigation: currentNavigation,
    requestErrorCode: ["CERT_HAS_EXPIRED", "DEPTH_ZERO_SELF_SIGNED_CERT", "SELF_SIGNED_CERT_IN_CHAIN", "UNABLE_TO_VERIFY_LEAF_SIGNATURE", "UNABLE_TO_GET_ISSUER_CERT_LOCALLY"].find(code => error.code === code || String(error.message).includes(code)) ||
      (/unable to verify|self.signed certificate|unable to get local issuer/i.test(String(error.message)) ? "UNTRUSTED_CERTIFICATE" : undefined),
    authCookieCount: authContext ? await authContext.cookies().then(c=>c.length).catch(()=>undefined) : undefined,
    pageState: currentNavigation ? currentNavigation.identity : phase === "measure-flows" ? "not-recorded" : activePage ? (new URL(activePage.url()).pathname === "/login" ? "login" : "other") : "not-created"};
}, failure => writeFile(join(run, "failure.json"), JSON.stringify(failure, null, 2)), () => browser?.close());
browser = await ({chromium, webkit, firefox}[engine]).launch(engine === "chromium" && process.platform === "darwin" ? {channel: "chrome"} : {});
const context = authContext = await browser.newContext({baseURL, ignoreHTTPSErrors: false, reducedMotion: "reduce"});
const page = activePage = await context.newPage();
navigation = navigationDiagnostics(page,baseURL);
page.on("response", response => {if(response.request().method()==="POST"&&new URL(response.url()).pathname==="/login")loginResponses.push(response.status());});
phase = "login-page";
await page.goto("/login");
await page.getByLabel("Name", {exact: true}).fill("Owner");
await page.getByLabel("Password", {exact: true}).fill("synthetic-layout-password");
const bits = [...process.env.KINOSAIL_LAYOUT_TOTP].map(c => "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567".indexOf(c).toString(2).padStart(5, "0")).join("");
const secret = Buffer.from(bits.match(/.{8}/g).map(byte => parseInt(byte, 2)));
const counter = Buffer.alloc(8); counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000)));
const digest = createHmac("sha1", secret).update(counter).digest(), offset = digest[19] & 15;
const factor = page.getByLabel(/Authentication or recovery code|6-digit code/);
const code = String((digest.readUInt32BE(offset) & 0x7fffffff) % 1000000).padStart(6, "0");
if (await factor.isVisible()) await factor.fill(code);
phase = "login-submit";
await page.getByRole("button", {name: "Sign in", exact: true}).click();
if (app === "subtitles" && new URL(page.url()).pathname === "/login" && await factor.isVisible()) {
  await factor.fill(code);
  await page.getByRole("button", {name: "Sign in", exact: true}).click();
}
phase = "login-redirect";
await page.waitForURL(url => url.pathname !== "/login");
if (await page.getByRole("link", {name: "Not now"}).isVisible()) await page.getByRole("link", {name: "Not now"}).click();
phase = "library-request";
const response = await page.request.get("/api/v1/library");
requestStatus = response.status();
const contentType = response.headers()["content-type"]?.split(";")[0];
requestContentType = ["application/json", "text/html", "text/plain"].includes(contentType) ? contentType : "other";
const data = await response.json();
const item = data.items.find(candidate => candidate.title === "Layout Example") || data.items[0];
const auth = await context.storageState();
const navProfile = app === "player" ? await page.locator("[data-mobile-tabs]").getAttribute("data-nav-profile") : undefined;
navigation.stop();
await context.close();
function cls(shifts) {
  let maximum=0, sum=0, start=0, last=0;
  for(const e of shifts){if(e.time-last>1000||e.time-start>5000){sum=0;start=e.time;}sum+=e.value;maximum=Math.max(maximum,sum);last=e.time;}
  return maximum;
}
const boxesChanged=(a,b)=>a.filter(first=>{const last=b.find(v=>v.id===first.id);return last&&["x",first.pinned&&last.pinned?"y":"documentY","width","height"].some(key=>Math.abs(first[key]-last[key])>1);});
const inspect = () => ({rootFontSize: getComputedStyle(document.documentElement).fontSize, rootScale:document.documentElement.style.fontSize, ready: document.querySelector(".settings-shell")?.hasAttribute("data-settings-ready"), category: document.documentElement.dataset.settingsCategory, scrollX, scrollY,
  timeouts: ["#session-timeouts","#security"].includes(location.hash)?(()=>{const anchor=document.getElementById(location.hash.slice(1)),target=document.getElementById("session-timeouts");return {anchorPresent:Boolean(anchor),visible:Boolean(target&&target.getBoundingClientRect().height&&(!target.checkVisibility||target.checkVisibility())),access:[...target?.querySelectorAll("form[data-timeout-access]")||[]].map(n=>n.dataset.timeoutAccess).sort()};})():undefined,
  sections: [...document.querySelectorAll(".settings-flow>section")].filter(n=>n.getBoundingClientRect().height).map(n=>({id:n.id,category:n.dataset.settingsCategory,heading:n.querySelector("h2")?.textContent})),
  nativeOptions: document.querySelectorAll(".player-native-options").length, settingsButtons: document.querySelectorAll("[data-player-settings]").length,
  overflowNodes: [...document.querySelectorAll("body *:not(option):not(optgroup)")].filter(n => {const r=n.getBoundingClientRect();return r.height>0&&r.right+(getComputedStyle(n).position==="fixed"?0:scrollX)>innerWidth+1;}).slice(0,30).map(n=>({node:n.id||n.className||n.tagName,rect:n.getBoundingClientRect().toJSON(),minWidth:getComputedStyle(n).minWidth,whiteSpace:getComputedStyle(n).whiteSpace})),
  scrollContainers: [...document.querySelectorAll("body *:not(option):not(optgroup)")].filter(n=>n.clientWidth>0&&n.scrollWidth>n.clientWidth+1).slice(0,30).map(n=>({node:n.id||n.className||n.tagName,rect:n.getBoundingClientRect().toJSON(),clientWidth:n.clientWidth,scrollWidth:n.scrollWidth,overflowX:getComputedStyle(n).overflowX}))});
function observe() {
  const ids = new WeakMap(); let nextID = 0;
  const identify = node => {if(!ids.has(node))ids.set(node, ++nextID);return ids.get(node);};
  const label = node => node?.id || (typeof node?.className === "string" ? node.className : "") || node?.nodeName;
  window.layoutAudit = {shifts: [], frames: [], paints: [], support: PerformanceObserver.supportedEntryTypes};
  if (PerformanceObserver.supportedEntryTypes.includes("layout-shift")) new PerformanceObserver(list => {
    for (const entry of list.getEntries()) window.layoutAudit.shifts.push({time: entry.startTime, value: entry.value,
      recentInput: entry.hadRecentInput, sources: entry.sources?.map(source => ({node: label(source.node),
        previous: source.previousRect.toJSON(), current: source.currentRect.toJSON()}))});
  }).observe({type: "layout-shift", buffered: true});
  if(PerformanceObserver.supportedEntryTypes.includes("paint")) new PerformanceObserver(list => window.layoutAudit.paints.push(...list.getEntries().map(e => ({name: e.name, time: e.startTime})))).observe({type: "paint", buffered: true});
  let last = 0;
  const sample = time => {
    if (time - last > 80) {
      last = time;
      const boxes = [...document.querySelectorAll(".app-header, .mobile-navigation, .subtitle-dashboard .app-header nav, main, h1, h2, .card, .home-feature, .media-stage, .player-stage-toolbar, .player-optional-action, .settings-nav, .settings-flow, .settings-category-description, button")].slice(0, 60)
        .filter(node => node.getBoundingClientRect().height > 0 && (!node.checkVisibility || node.checkVisibility()) && ![...document.querySelectorAll("details:not([open])")].some(d=>d.contains(node)&&!d.querySelector(":scope>summary")?.contains(node))).map(node => ({pinned: (()=>{for(let n=node;n;n=n.parentElement)if(["fixed","sticky"].includes(getComputedStyle(n).position))return true;return false;})(), id: identify(node), documentY: node.getBoundingClientRect().y + scrollY, node: label(node), aria: node.getAttribute("aria-label"), text: node.textContent.trim().slice(0, 45), ...node.getBoundingClientRect().toJSON()}));
      window.layoutAudit.frames.push({time, scrollY, boxes});
    }
    if (time < 8000) requestAnimationFrame(sample);
  };
  requestAnimationFrame(sample);
}
const viewports = process.env.KINOSAIL_LAYOUT_QUICK ? [{width: 390, height: 844}, {width: 1440, height: 900}] :
  [{width: 320, height: 800}, {width: 390, height: 844}, {width: 844, height: 390}, {width: 768, height: 1024}, {width: 1024, height: 768}, {width: 1440, height: 900}, {width: 1920, height: 1080}];
let routes = ["/login", "/", "/?view=movies", "/settings", "/settings#access", "/account", `/watch/${item.id}?playback=direct`, "/?view=movies&q=no-synthetic-match", "/item/missing-layout-probe"];
if(app==="player")routes.push(`/item/${item.id}`);
else {routes=routes.map(path=>path.replace("view=movies","view=library").replace("#access","#provider"));routes.push("/?view=wanted","/?view=history");const sub=await pageRequestLibrary(); if(!sub)throw new Error("Synthetic subtitle library must include an inspector item");routes.push(`/subtitles/inspect/${sub}?language=en`);}
routes.push(app==="player"?"/settings#session-timeouts":"/settings#security");
async function pageRequestLibrary(){const c=await browser.newContext({baseURL, storageState:auth});try{const r=await c.request.get("/api/v1/subtitle-library?view=library");const d=await r.json();return (d.items?.find(i=>i.title==="Layout Example")||d.items?.[0])?.id;}finally{await c.close();}}
if(process.env.KINOSAIL_LAYOUT_PATHS)routes=process.env.KINOSAIL_LAYOUT_PATHS.split(",");
const cases=viewports.flatMap(viewport=>routes.map(path=>({viewport,path,variant:"default"})));
if(process.env.KINOSAIL_LAYOUT_VARIANTS)for(const path of [app==="player"?"/settings#access":"/settings#provider",app==="player"?"/settings#session-timeouts":"/settings#security",`/watch/${item.id}?playback=direct`,...routes.filter(path=>path.startsWith("/subtitles/inspect/"))]){
  cases.push({viewport:{width:390,height:844},path,variant:"text-200",scale:"200%"});
  cases.push({viewport:{width:390,height:844},path,variant:"motion",motion:"no-preference"});
  if(path.startsWith("/settings"))for(const viewport of [{width:320,height:800},{width:844,height:390}])cases.push({viewport,path,variant:"text-200",scale:"200%"});
}
if(process.env.KINOSAIL_LAYOUT_VARIANTS)for(const path of ["/settings#%61ccess","/settings#%E0%A4%A"])cases.push({viewport:{width:390,height:844},path,variant:"fragment"});
if(process.env.KINOSAIL_LAYOUT_VARIANTS)for(const path of routes.filter(path=>path.startsWith("/settings#")))for(const viewport of [{width:390,height:844},{width:320,height:800}])cases.push({viewport,path,variant:"slow-css",scale:viewport.width===320?"200%":undefined});
if(process.env.KINOSAIL_LAYOUT_VARIANTS&&app==="player")cases.push({viewport:{width:390,height:844},path:"/settings#access",variant:"saved-mobile-tabs",scale:"200%",savedTabs:true});
if(process.env.KINOSAIL_LAYOUT_APPLE_SHIM)cases.push({viewport:{width:768,height:1024},path:`/watch/${item.id}?playback=direct`,variant:"desktop-UA-iPad-shim",apple:true});
if(app==="subtitles"&&process.env.KINOSAIL_LAYOUT_VARIANTS)cases.push({viewport:{width:320,height:800},path:"/?view=library",variant:"text-200",scale:"200%"});
try {
  for (const {viewport,path,variant,scale,motion,apple,savedTabs} of cases) {
    phase = "measure-case";
    operationPhase = "context-create";
    activeCase = {viewport, path, variant};
    const context = await browser.newContext({baseURL, storageState: path === "/login" ? undefined : auth,
      viewport, ignoreHTTPSErrors: false, reducedMotion: motion||"reduce"});
    if(scale)await context.addInitScript(scale=>{const apply=()=>{if(!document.documentElement)return false;document.documentElement.style.fontSize=scale;return true;};if(!apply()){const observer=new MutationObserver(()=>{if(apply())observer.disconnect();});observer.observe(document,{childList:true});}},scale);
    if(savedTabs)await context.addInitScript(profile=>localStorage.setItem(`kinosail:tabs:v2:${profile}`,'["audiobooks","list","history"]'),navProfile);
    if(apple)await context.addInitScript(()=>{
      Object.defineProperty(navigator,"platform",{value:"MacIntel"});Object.defineProperty(navigator,"maxTouchPoints",{value:5});
      HTMLVideoElement.prototype.webkitEnterFullscreen=function(){this.dispatchEvent(new Event("webkitbeginfullscreen"));};
    });
    const traced=viewport.width===390&&(path==="/settings#access"||path.startsWith("/watch/"));
    operationPhase = "trace-start";
    if(traced)await context.tracing.start({screenshots:true,snapshots:true});
    await context.addInitScript(observe);
    const page = activePage = await context.newPage();
navigation = navigationDiagnostics(page,baseURL);
    // Delay real response bytes, without substituting mock markup or media.
    await page.route("**/*", layoutResponseHandler(context, variant));
    const name = `${viewport.width}-${variant}-${path.replace(/[^a-z0-9]+/gi, "-")}`;
    operationPhase = "navigate";
    const response=await page.goto(path, {waitUntil: "commit"});
    operationPhase = "body-visible";
    await page.locator("body").waitFor({state: "visible"});
    if(variant==="slow-css") {await page.waitForFunction(()=>[...document.querySelectorAll('link[rel~="stylesheet"]')].every(link=>link.sheet));await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));}
    await page.waitForTimeout(200);
    operationPhase = "initial-inspect";
    const initialState = await page.evaluate(inspect);
    operationPhase = "initial-bookmark";
    initialState.bookmark = await page.evaluate(bookmarkSnapshot);
    operationPhase = "initial-boxes";
    const initialBoxes=await page.evaluate(()=>window.layoutAudit.frames.at(-1)?.boxes||[]);
    if (engine === "chromium") {
      // CDP captures pixels without Playwright's font-readiness hook, which can
      // itself force an optional font swap and contaminate layout measurements.
      const cdp = await context.newCDPSession(page);
      const capture = await cdp.send("Page.captureScreenshot", {format: "png"});
      await writeFile(join(run, name + "-initial.png"), Buffer.from(capture.data, "base64"));
      await cdp.detach();
    }
    operationPhase = "domcontentloaded";
    await page.waitForLoadState("domcontentloaded");
    await page.waitForTimeout(2400);
    operationPhase = "settled-audit";
    const audit = await page.evaluate(() => ({...window.layoutAudit, overflow: document.documentElement.scrollWidth - innerWidth,
      font: document.fonts.check("15px Manrope"), skeleton: document.querySelectorAll(".request-skeleton").length}));
    const entries=audit.shifts.filter(s=>!s.recentInput);
    const unexpected=cls(entries), aggregateUnexpected=entries.reduce((sum,e)=>sum+e.value,0);
    const identifiedDOMCLS=cls(entries.filter(e=>e.sources?.some(s=>s.node)));
    const unattributedCLS=cls(entries.filter(e=>!e.sources?.some(s=>s.node)));
    const moved=boxesChanged(initialBoxes,audit.frames.at(-1)?.boxes||[]);
    operationPhase = "settled-inspect";
    const finalState=await page.evaluate(inspect);
    finalState.bookmark=await page.evaluate(bookmarkSnapshot);
    const categoryStable=!initialState.category||JSON.stringify(initialState.sections)===JSON.stringify(finalState.sections);
    const timeoutsPresent=!["/settings#session-timeouts","/settings#security"].includes(path)||[initialState,finalState].every(s=>s.timeouts?.anchorPresent&&s.timeouts.visible&&JSON.stringify(s.timeouts.access)==='["private","public"]');
    const bookmarkRequired=routes.filter(route=>route.startsWith("/settings#")).includes(path)||(app==="player"&&path==="/settings#%61ccess");
    const bookmarkVisible=[initialState,finalState].every(s=>bookmarkRequired?Boolean(s.bookmark?.resolved&&s.bookmark.visible):!s.bookmark?.resolved||s.bookmark.visible);
    reports.push({viewport,path,variant,scaleApplied:!scale||finalState.rootScale===scale,status:response.status(),unexpected,aggregateUnexpected,identifiedDOMCLS,unattributedCLS,moved,categoryStable,timeoutsPresent,bookmarkRequired,bookmarkVisible,initialState,finalState,...audit});
    console.log(JSON.stringify({viewport: viewport.width, path, unexpected, overflow: audit.overflow,
      sources: audit.shifts.flatMap(shift => shift.sources.map(source => source.node))}));
    if (engine === "chromium") {
      const cdp = await context.newCDPSession(page);
      const capture = await cdp.send("Page.captureScreenshot", {format: "png"});
      await writeFile(join(run, name + "-settled.png"), Buffer.from(capture.data, "base64"));
      await cdp.detach();
    } else await page.screenshot({path: join(run, name + "-settled.png")});
    if(path.startsWith("/watch/")&&viewport.width===390&&!apple){
      const settingsButton=page.getByRole("button",{name:"Settings",exact:true});
      const stage=await page.locator(".media-stage").boundingBox();await page.mouse.move(stage.x+10,stage.y+10);
      phase = "settings-open";
      await settingsButton.click();await page.locator(".player-settings").waitFor({state:"visible"});
      phase = "settings-keyboard-close";
      await page.keyboard.press("Escape");await page.locator(".player-settings").waitFor({state:"hidden"});
      reports.at(-1).settingsKeyboard={closed:true,focusRestored:await settingsButton.evaluate(n=>n===document.activeElement)};
    }
    if(viewport.width===390&&variant==="default"&&(path.startsWith("/settings")||path.startsWith("/watch/")||path.startsWith("/subtitles/inspect/")||path.startsWith("/?view=library"))){const audit=await new AxeBuilder({page}).analyze();reports.at(-1).accessibility={violations:audit.violations.map(v=>({id:v.id,impact:v.impact,nodes:v.nodes.map(n=>n.target)}))};}
    if(traced)await context.tracing.stop({path:join(run,name+"-trace.zip")});
    await context.close();
  }
  if(process.env.KINOSAIL_LAYOUT_FLOWS){phase="measure-flows";await measureFlows(browser,{baseURL,storageState:auth},`/watch/${item.id}?playback=direct`,routes.find(path=>path.startsWith("/subtitles/inspect/")),flows,flowProbe);}
} finally {
  await writeFile(join(run, "measurements.json"), JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION, app, engine,browserVersion:browser.version(),
    result: "measurement", command: "python3 scripts/testing/test-layout-stability-local.py", data: "Synthetic media and account; delayed real font/bundle/image responses", reports,flows}, null, 2));
  await browser.close();
}
if (process.env.KINOSAIL_LAYOUT_ENFORCE && reports.some(report => report.identifiedDOMCLS > 0.001 || report.overflow > 1 || !report.categoryStable || !report.timeoutsPresent || !report.bookmarkVisible || !report.scaleApplied || report.moved.length>0)) process.exitCode = 1;
if(flows.some(f=>f.pendingStable===false||f.failureRetainsContent===false||f.caretPreserved===false||f.focusRetained===false||f.scrollRetained===false||f.settled?.inert||f.settled?.skeleton||f.stable===false||f.overflow>1))process.exitCode=1;
