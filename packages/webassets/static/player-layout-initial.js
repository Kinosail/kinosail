
// Project capability-dependent Apple controls while the HTML is being parsed,
// before the deferred player bundle. Keep one toolbar and one settings panel.
if (location.pathname.startsWith("/watch/")) {
  const appleTouch = /iPhone|iPad|iPod/.test(navigator.userAgent) || navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1;
  if (appleTouch && typeof document.createElement("video").webkitEnterFullscreen === "function") {
    const project = () => {
      const video = document.querySelector(".media-stage video");
      if (!video) return;
      video.autoplay = false;
      delete video.dataset.autoplay;
      video.controls = false;
      const stage = video.closest(".media-stage");
      stage.querySelector("[data-player-controls]")?.classList.add("player-native-controls");
      const existing = stage.nextElementSibling;
      if (existing?.classList.contains("player-native-options")) {
        if (existing.querySelector(".player-settings")) observer.disconnect();
        return;
      }
      const toolbar = stage.querySelector(".player-stage-toolbar");
      const settings = stage.querySelector(".player-settings");
      // The buffer follows the complete settings subtree in both app templates.
      if (!toolbar || !settings || !stage.querySelector("[data-player-status]")) return;
      const button = stage.querySelector("[data-player-settings]");
      if (button && !toolbar.contains(button)) toolbar.append(button);
      const options = document.createElement("div");
      options.className = "player-native-options";
      stage.after(options);
      options.append(toolbar, settings);
      observer.disconnect();
    };
    const observer = new MutationObserver(project);
    observer.observe(document, {childList: true, subtree: true});
    document.addEventListener("DOMContentLoaded", () => { project(); observer.disconnect(); }, {once: true});
  }
}
