async function bindSupporterRecognition() {
  if (document.body.classList.contains("auth")) return;
  try {
    const response = await fetch("/api/v1/supporter/collection", {headers: {accept: "application/json"}});
    if (!response.ok) return;
    const collection = await response.json();
    if (!Array.isArray(collection.badges) || collection.badges.length > 4) return;
    const badges = collection.badges.filter(badge => Number.isInteger(badge.rank) && badge.rank >= 1 && badge.rank <= 10 &&
      ["one-time", "monthly", "yearly", undefined].includes(badge.edition) && ["patron-order", "living-standard"].includes(badge.family));
    const hidden = collection.display === "hidden";
    document.querySelectorAll(".supporter-signature, .supporter-edition-mark").forEach(mark => mark.remove());
    document.querySelectorAll(".header-supporter").forEach(link => {
      link.hidden = hidden;
      link.replaceChildren();
      link.classList.toggle("supporter-trio", badges.length > 0);
      if (!badges.length) { link.textContent = "Support Kinosail"; link.removeAttribute("aria-label"); return; }
      link.setAttribute("aria-label", "Your supporter collection");
      badges.forEach(badge => {
        const image = document.createElement("img");
        image.src = `/static/supporter/badges/${badge.edition || badge.family}-small.svg`;
        if (!badge.edition) image.src = `/static/supporter/badges/${badge.family}-${badge.rank}-small.svg?v=4`;
        image.alt = `${badge.name} · ${badge.edition || "Legacy recurring"}${badge.archived ? " · past support" : ""}`;
        image.width = 24; image.height = 24;
        link.append(image);
      });
    });
    const setting = document.querySelector("[data-supporter-visibility]");
    if (setting) {
      setting.checked = !hidden;
      setting.disabled = false;
      if (!setting.dataset.bound) {
        setting.dataset.bound = "true";
        setting.addEventListener("change", async () => {
          setting.disabled = true;
          const output = document.querySelector("[data-supporter-visibility-status]");
          try {
            const csrf = document.querySelector('meta[name="kinosail-csrf"]')?.content;
            const saved = await fetch("/api/v1/supporter/display", {method: "PUT", headers: {"Content-Type": "application/json", ...(csrf ? {"X-Kinosail-CSRF": csrf} : {})}, body: JSON.stringify({display: setting.checked ? "automatic" : "hidden"})});
            if (!saved.ok) throw new Error();
            output.textContent = "Saved. Your collection stays visible on Supporter.";
            await bindSupporterRecognition();
          } catch { setting.checked = !setting.checked; output.textContent = "Could not save. Please try again."; }
          finally { setting.disabled = false; }
        });
      }
    }
  } catch (_) {}
}

async function supporterShareFile(family) {
  if (!["living-standard", "patron-order", "one-time", "monthly", "yearly"].includes(family)) throw new Error("certificate unavailable");
  const response = await fetch(`/api/v1/supporter/certificates/${family}.svg`, { headers: { accept: "image/svg+xml" } });
  if (!response.ok) throw new Error("certificate unavailable");
  const certificate = await response.blob();
  if (certificate.size > 262144 || certificate.type.split(";")[0] !== "image/svg+xml") throw new Error("certificate unavailable");
  const blob = await renderSupporterPNG(certificate, "image unavailable");
  return new File([blob], `kinosail-player-${family}.png`, { type: "image/png" });
}

function downloadSupporterShare(file, status) {
	const link = document.createElement("a");
	link.href = URL.createObjectURL(file);
	link.download = file.name;
	link.click();
	setTimeout(() => URL.revokeObjectURL(link.href), 0);
	if (status) status.textContent = "Share image downloaded.";
}

function bindSupporterShare() {
  document.querySelectorAll("[data-supporter-share]").forEach((button) => {
    if (button.dataset.bound) return;
    button.dataset.bound = "true";
    button.hidden = false;
    button.addEventListener("click", async () => {
      const family = button.dataset.supporterShare;
      const status = button.closest(".supporter-certificate-actions")?.querySelector("output") || document.querySelector(`[data-supporter-share-status="${family}"]`);
      button.disabled = true;
      if (status) status.textContent = "Preparing share image…";
      try {
        const file = await supporterShareFile(family);
		try {
		  if (navigator.share && (!navigator.canShare || navigator.canShare({ files: [file] }))) {
			await navigator.share({ title: `My Kinosail Player ${button.textContent.replace("Share ", "")}`, files: [file] });
			if (status) status.textContent = "Certificate shared.";
			return;
		  }
		} catch (error) {
		  if (error?.name === "AbortError") {
			if (status) status.textContent = "";
			return;
		  }
        }
		downloadSupporterShare(file, status);
      } catch {
		if (status) status.textContent = "Could not prepare the share image. Download the certificate instead.";
      } finally {
        button.disabled = false;
      }
    });
  });
}

bindSupporterRecognition();
bindSupporterShare();
document.addEventListener("htmx:afterSwap", bindSupporterRecognition);

window.addEventListener("pageshow", (event) => { if (event.persisted) bindSupporterRecognition(); });
