player.addEventListener("kinosail:navigation", () => {
  if (isPictureInPicture()) return;
  adaptiveGeneration += 1;
  cancelNetworkRecovery();
  adaptiveOriginController?.abort();
  clearBufferingRecovery();
  pendingResume?.cancel();
  hls?.stopLoad();
});
const streaming = {destroy: () => {
  destroyed = true;
  adaptiveOriginController?.abort();
  adaptiveGeneration += 1;
  adaptiveStarting = false;
  pendingResume?.cancel();
  hls?.destroy();
  hls = undefined;
}};
