// Preserve one safe failure artifact even when more errors arrive during cleanup.
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
