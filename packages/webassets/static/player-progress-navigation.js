// Keep explicit Library navigation alive until its latest owned checkpoint is acknowledged.
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
  if (link.origin !== location.origin || link.pathname !== "/" || link.search || link.hash) { cancelProgressNavigation(); return; }
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
    location.assign(navigation.target);
  };
  progressNavigation = navigation;
  progressContinuation = navigation.leave;
  if (progressNotice) progressNotice.hidden = false;
  void save(false, true);
  // An existing authentication/policy failure can return before sendProgress refreshes the notice.
  if (progressFailure && !retryableProgress()) showProgressFailure();
});
// Manual watched status must follow the current page's final position write.
document.addEventListener("submit", event => {
  const form = event.target;
  if (!(form instanceof HTMLFormElement) || new URL(form.action).origin !== location.origin ||
      new URL(form.action).pathname !== `/watched/${progressItem()}` || !progressNavigationAllowed()) return;
  event.preventDefault();
  if (progressNavigation) return;
  requestPause();
  const navigation = {profile: progressProfile(), item: progressItem(), source: player.currentSrc || player.src,
    request: playbackRequest, deadline: performance.now() + 8000, leave: undefined};
  navigation.leave = () => {
    if (progressNavigation !== navigation || !ownsProgressNavigation()) { cancelProgressNavigation(); return; }
    cancelProgressNavigation();
    if (!form.checkValidity()) return;
    progressPlayedItem = undefined;
    form.requestSubmit(event.submitter);
  };
  progressNavigation = navigation;
  progressContinuation = navigation.leave;
  if (progressNotice) progressNotice.hidden = false;
  void save(false, true);
  if (progressFailure && !retryableProgress()) showProgressFailure();
});
