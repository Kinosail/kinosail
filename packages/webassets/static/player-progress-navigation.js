// Keep explicit browse return navigation alive until its latest owned checkpoint is acknowledged.
// Browser Back and tab close still use the separate pagehide keepalive fallback.
function progressNavigationAllowed() {
  return progressChanged() && !playbackPreparation && !isPictureInPicture() && player.dataset.castActive !== "true" &&
    player.dataset.offline !== "true" && player.readyState >= HTMLMediaElement.HAVE_METADATA &&
    !player.ended && !pendingProgress?.watched && Boolean(progressItem()) && progressProfile().length <= 128 &&
    Number.isFinite(player.currentTime) && player.currentTime >= 0 && player.currentTime <= 31536000;
}
function ownsProgressNavigation() {
  return progressNavigation && progressNavigation.profile === progressProfile() &&
    progressNavigation.item === progressItem() && progressNavigation.source === (player.currentSrc || player.src) &&
    progressNavigation.request === playbackRequest &&
    player.paused && progressNavigationAllowed();
}
function cancelProgressNavigation() {
  const navigation = progressNavigation;
  if (!navigation) return;
  progressNavigation = undefined;
  if (progressContinuation === navigation.leave) progressContinuation = undefined;
  if (progressContinue) progressContinue.hidden = !progressContinuation;
}
for (const event of ["play", "seeking", "emptied", "loadstart", "enterpictureinpicture"]) {
  player.addEventListener(event, cancelProgressNavigation);
}
player.addEventListener("webkitpresentationmodechanged", () => {
  if (isPictureInPicture()) cancelProgressNavigation();
});
for (const selector of ["[data-audio-track]", "[data-playback-mode]", "[data-quality]"]) {
  document.querySelector(selector)?.addEventListener("change", cancelProgressNavigation, {capture: true});
}
document.addEventListener("click", event => {
  const link = event.target.closest?.('a[href]');
  if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey ||
      !link || link.hasAttribute("download") || link.target && link.target !== "_self") return;
  if (progressNavigation?.target && ownsProgressNavigation() &&
      link.origin === location.origin && link.pathname === "/" && !link.hash) {
    event.preventDefault();
    progressNavigation.target = link.href;
    return;
  }
  if (link.origin !== location.origin || link.pathname !== "/" || (link.search && !link.hasAttribute("data-browse-return")) || link.hash) { cancelProgressNavigation(); return; }
  if (!progressNavigationAllowed()) return;
  event.preventDefault();
  if (progressNavigation && !ownsProgressNavigation()) cancelProgressNavigation();
  if (progressNavigation) return;
  requestPause();
  const navigation = {profile: progressProfile(), item: progressItem(), source: player.currentSrc || player.src,
    request: playbackRequest, target: link.href, deadline: performance.now() + 8000, leave: undefined};
  navigation.leave = () => {
    if (progressNavigation !== navigation) return;
    if (!ownsProgressNavigation()) { cancelProgressNavigation(); return; }
    cancelProgressNavigation();
    player.dispatchEvent(new Event("kinosail:navigation"));
    location.assign(navigation.target);
  };
  progressNavigation = navigation;
  progressContinuation = navigation.leave;
  if (progressNotice) progressNotice.hidden = false;
  void save(false, true);
  // An existing authentication/policy failure can return before sendProgress refreshes the notice.
  if (progressFailure && !retryableProgress()) showProgressFailure();
});
// Wait for every listener's cancellation decision before stopping an accepted
// departure. Watched submissions replay here after their checkpoint is saved.
document.addEventListener("click", event => {
  const link = event.target.closest?.('a[href]');
  if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey ||
      !link || link.hasAttribute("download") || link.target && link.target !== "_self" ||
      link.origin !== location.origin || link.pathname !== "/" || (link.search && !link.hasAttribute("data-browse-return")) || link.hash) return;
  setTimeout(() => { if (!event.defaultPrevented) player.dispatchEvent(new Event("kinosail:navigation")); });
});
document.addEventListener("submit", event => {
  const form = event.target;
  if (!(form instanceof HTMLFormElement)) return;
  const submitter = event.submitter;
  if (submitter && !(submitter instanceof HTMLButtonElement || submitter instanceof HTMLInputElement)) return;
  let action;
  try { action = new URL(submitter?.hasAttribute("formaction") ? submitter.formAction : form.action); }
  catch (_) { return; }
  const target = submitter?.hasAttribute("formtarget") ? submitter.formTarget : form.target;
  const method = submitter?.hasAttribute("formmethod") ? submitter.formMethod : form.method;
  if (method !== "post" || target && target !== "_self" ||
      action.origin !== location.origin || action.pathname !== `/watched/${progressItem()}`) return;
  watchedDeparture ||= watchedSubmission?.eventPhase === Event.NONE && !watchedSubmission.defaultPrevented;
  watchedSubmission = event;
  setTimeout(() => { if (!event.defaultPrevented) player.dispatchEvent(new Event("kinosail:navigation")); });
});
// Manual watched status must follow the current page's final position write.
document.addEventListener("submit", event => {
  if (watchedSubmission !== event || event.defaultPrevented || watchedReplay || watchedDeparture || !progressNavigationAllowed()) return;
  const form = event.target;
  event.preventDefault();
  if (progressNavigation) return;
  requestPause();
  const navigation = {profile: progressProfile(), item: progressItem(), source: player.currentSrc || player.src,
    request: playbackRequest, deadline: performance.now() + 8000, leave: undefined};
  navigation.leave = () => {
    if (progressNavigation !== navigation || !ownsProgressNavigation()) { cancelProgressNavigation(); return; }
    cancelProgressNavigation();
    if (!form.checkValidity()) return;
    watchedReplay = true;
    try { form.requestSubmit(event.submitter); }
    finally { watchedReplay = false; }
  };
  progressNavigation = navigation;
  progressContinuation = navigation.leave;
  if (progressNotice) progressNotice.hidden = false;
  void save(false, true);
  if (progressFailure && !retryableProgress()) showProgressFailure();
});
