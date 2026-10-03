// Align a bookmarked section before the first body paint. Native fragment
// scrolling can otherwise wait for unrelated deferred scripts to finish.
if (location.hash) (() => {
  let id;
  try { id = decodeURIComponent(location.hash.slice(1)); } catch (_) { return; }
  if (!id) return;
  let frame, completed = false;
  const project = () => {
    frame = undefined;
    if (completed) return;
    const target = document.getElementById(id);
    if (!target) return;
    for (let ancestor = target; ancestor; ancestor = ancestor.parentElement) if (ancestor instanceof HTMLDetailsElement) ancestor.open = true;
    if (!target.getClientRects().length) return;
    target.scrollIntoView({block: "start", behavior: "instant"});
    completed = true;
    observer.disconnect();
  };
  const observer = new MutationObserver(() => { if (!frame) frame = requestAnimationFrame(project); });
  observer.observe(document, {childList: true, subtree: true});
  frame = requestAnimationFrame(project);
  document.addEventListener("DOMContentLoaded", () => {
    if (frame) cancelAnimationFrame(frame);
    project();
    observer.disconnect();
  }, {once: true});
})();
