// Preserve one safe failure artifact even when more errors arrive during cleanup.
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
