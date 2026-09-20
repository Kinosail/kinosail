(() => {
  if (window.kinosailOfflineIdentity) return;
  const key = "kinosail.offline-identity-v1";
  let selected = {profile: "", revision: 0};
  const state = () => {
    try {
      const saved = JSON.parse(localStorage.getItem(key) || "null");
      if (saved && typeof saved.profile === "string" && /^[A-Za-z0-9_-]{0,128}$/.test(saved.profile) && Number.isSafeInteger(saved.revision) && saved.revision > selected.revision && saved.revision <= Date.now() + 60000) return saved;
    } catch {}
    return selected;
  };
  const update = (next) => {
    if (next.revision <= state().revision) return;
    selected = next;
    try { localStorage.setItem(key, JSON.stringify(next)); localStorage.setItem("kinosail.offline-profile", next.profile); } catch {}
    window.dispatchEvent(new Event("kinosail:offline-profile"));
  };
  const select = (profile) => update({profile, revision: Math.max(Date.now(), state().revision + 1)});
  window.kinosailOfflineIdentity = {current: () => state().profile, state};
  const profile = document.body.dataset.viewerProfile || document.querySelector("[data-nav-profile]")?.dataset.navProfile || document.querySelector("#downloads")?.dataset.viewerProfile;
  if (profile) select(profile);
  navigator.serviceWorker?.addEventListener("message", ({data}) => {
    if (data?.type === "offline-profile" && typeof data.profile === "string" && Number.isSafeInteger(data.revision)) update({profile: data.profile, revision: data.revision});
  });
  document.addEventListener("submit", ({target}) => {
    if (target instanceof HTMLFormElement && new URL(target.action, location.href).pathname === "/logout") {
      select("");
      navigator.serviceWorker?.controller?.postMessage({type: "logout", ...state()});
    }
  }, true);
  window.addEventListener("storage", (event) => {
    if (event.key === key) window.dispatchEvent(new Event("kinosail:offline-profile"));
  });
})();
