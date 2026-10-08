import {navigationDiagnostics} from "./navigation-diagnostics.mjs";

// Keep the current flow's witness before closing its owned page. A previous
// measurement page cannot explain a navigation or action in a fresh context.
export async function observeLayoutFlow(context, baseURL, probe, operation) {
  delete probe.navigation;
  delete probe.operationPhase;
  delete probe.elapsedMs;
  delete probe.media;
  delete probe.geometry;
  let navigation;
  let failed = false;
  try {
    const page = probe.openPage ? await probe.openPage(context) : await context.newPage();
    navigation = navigationDiagnostics(page, baseURL);
    probe.operationPhase = "flow";
    return await operation(page, navigation);
  } catch (error) {
    failed = true;
    if (navigation) probe.navigation = await navigation.snapshot(error);
    await probe.captureFailure?.(error).catch(() => {});
    throw error;
  } finally {
    navigation?.stop();
    try { await context.close(); }
    catch (error) {
      if (!failed) {
        if (navigation) probe.navigation = await navigation.snapshot(error);
        throw error;
      }
    }
  }
}
