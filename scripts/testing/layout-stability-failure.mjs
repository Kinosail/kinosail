import {join} from "node:path";
// Preserve one safe failure artifact even when more errors arrive during cleanup.
export async function layoutFailureNavigation(phase, flowProbe, navigation, error) {
  // Flow snapshots belong to fresh contexts. The global recorder belongs to
  // the last layout case, which can already be closed when a flow fails.
  return phase === "measure-flows" ? flowProbe.navigation : await navigation?.snapshot(error);
}

export function layoutFailureLocations(error) {
  if (typeof error?.stack !== "string" || error.stack.length > 8192) return [];
  const locations = [];
  for (const frame of error.stack.split("\n").slice(1, 21)) {
    const match = frame.match(/^\s+at (?:[A-Za-z0-9_.$<> ]+ )?\(?((?:file:\/\/)?\/(?:[A-Za-z0-9_.-]+\/)*scripts\/testing\/(layout-stability-(?:local|routing|bookmarks|flows)\.mjs)):(\d{1,5}):(\d{1,5})\)?$/);
    if (match && +match[3] > 0 && +match[4] > 0) locations.push({file: match[2], line: +match[3], column: +match[4]});
    if (locations.length === 3) break;
  }
  return locations;
}

function bounded(operation, milliseconds) {
  let timer;
  return Promise.race([operation, new Promise((_, reject) => {
    timer = setTimeout(() => reject(new Error("Diagnostic deadline")), milliseconds);
  })]).finally(() => clearTimeout(timer));
}
export function installLayoutFailureReporter(collect, persist, close) {
  let reporting = false;
  process.on("uncaughtException", error => {
    process.exitCode = 1;
    if (reporting) return;
    reporting = true;
    void (async () => {
      let failure;
      try { failure = await bounded(Promise.resolve().then(() => collect(error)), 1000); }
      catch { failure = {result: "failed", errorClass: "Error", stage: "failure-collection"}; }
      try { await bounded(Promise.resolve().then(() => persist(failure)), 1000); }
      finally {
        try { await bounded(Promise.resolve().then(close), 500); } catch { /* Preserve the failed exit. */ }
        process.exit(1);
      }
    })().catch(() => process.exit(1));
  });
}

export async function layoutLoginDocument(page, expected, previous) {
  let timer;
  try {return await Promise.race([page.evaluate(({expected, previous}) => {
    const name=document.querySelector('input[name="name"]'), password=document.querySelector('input[name="password"][type="password"]');
    const editable=input=>Boolean(input&&!input.disabled&&!input.readOnly&&input.getBoundingClientRect().width&&input.getBoundingClientRect().height);
    return {urlMatchesExpected:location.href===expected, state:document.readyState, freshDocument:performance.timeOrigin!==previous,
      nameEditable:editable(name), passwordEditable:editable(password),
      nameLabelMatches:Boolean(name&&[...name.labels||[]].some(label=>label.textContent.trim()==="Name")),
      showSecretReady:Boolean(password?.closest(".password-control")?.querySelector('button.password-toggle[aria-label="Show secret"]'))};
  }, {expected, previous}), new Promise(resolve=>{timer=setTimeout(()=>resolve({unavailable:true}),1000);})]);}
  catch {return {unavailable:true};} finally {clearTimeout(timer);}
}

export async function captureLayoutFailure(page, run) {
  try {
    if (page) await page.screenshot({path: join(run, "failure.png"),
      mask: [page.locator('input[name="name"], input[name="username"], input[name="password"], input[name="code"], input[autocomplete="one-time-code"]')], maskColor: "#000000"});
  } catch { /* Never retry without the credential mask. */ }
  try { await page?.context().tracing.stop({path: join(run, "failure-trace.zip")}); } catch { /* Preserve the original failure. */ }
}
