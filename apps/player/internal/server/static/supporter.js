function ensureSupporterSignature(main) {
  createSupporterSignature(main, {hide: true});
}

function revealSupporterSignature(main) {
  const signature = main?.querySelector(".supporter-signature");
  signature?.removeAttribute("data-supporter-pending");
  signature?.style.removeProperty("visibility");
}

function supporterLevelLogo(badge) {
  const logo = document.createElement("span");
  logo.className = "supporter-level-logo";
  logo.dataset.rank = String(badge.rank);
  logo.dataset.family = badge.family;
  logo.setAttribute("aria-hidden", "true");
  const image = document.createElement("img");
  image.src = `/static/supporter/badges/${badge.family}-${badge.rank}-small.svg?v=4`;
  image.alt = "";
  image.decoding = "async";
  image.setAttribute("aria-hidden", "true");
  logo.append(image);
  return logo;
}

function primarySupporterBadge(status, display = "automatic") {
  if (display === "hidden") return null;
  const preferred = display === "patron-order" ? status?.patronOrder : display === "living-standard" ? status?.livingStandard : null;
	const badge = preferred || (status?.livingStandard?.active ? status.livingStandard : status?.patronOrder);
	return badge && badge.name && Number.isInteger(badge.rank) && badge.rank >= 1 && badge.rank <= 10 && ["living-standard", "patron-order"].includes(badge.family) ? badge : null;
}

function resetSupporterRecognition(main, archived = false) {
  delete document.body.dataset.supporterRank;
  document.querySelectorAll(".supporter-edition-mark").forEach((mark) => mark.remove());
  const signature = main?.querySelector(".supporter-signature");
  if (!signature) return;
  signature.classList.remove("is-active");
  const copy = document.createElement("p");
  copy.textContent = archived ? "Community edition · Living Standard archived." : "Community edition · Free and supporter-funded.";
  const link = document.createElement("a");
  link.href = "/supporter";
  link.textContent = archived ? "View supporter archive" : "Support Kinosail";
  signature.replaceChildren(copy, link);
}

function showSupporterRecognition(main, status, display) {
  const signature = main?.querySelector(".supporter-signature");
  const badge = primarySupporterBadge(status, display);
	if (!signature || !badge) return;
  signature.classList.add("is-active");
  const copy = document.createElement("p");
  const title = document.createElement("strong");
  title.textContent = badge.name;
  const family = badge.family === "living-standard" ? " Living Standard" : " Patron Order";
  copy.append(title, document.createTextNode(family));
  const link = document.createElement("a");
  link.href = "/supporter";
  link.textContent = "View passport";
  link.setAttribute("aria-label", `${badge.name}${family} supporter passport`);
  signature.replaceChildren(supporterLevelLogo(badge), copy, link);
}

async function bindSupporterRecognition() {
  const main = document.querySelector(".library-shell");
  ensureSupporterSignature(main);
  try {
    const response = await fetch("/api/v1/supporter", { headers: { accept: "application/json" } });
    if (!response.ok) return;
    const status = await response.json();
    const preference = await fetch("/api/v1/supporter/display", { headers: { accept: "application/json" } });
    if (!preference.ok) return;
    const { display } = await preference.json();
    if (!["automatic", "hidden", "patron-order", "living-standard"].includes(display)) return;
    const badge = primarySupporterBadge(status, display);
	resetSupporterRecognition(main, Boolean(status.livingStandard?.expired));
    const signature = main?.querySelector(".supporter-signature");
    if (signature) signature.hidden = display === "hidden";
    document.querySelectorAll(".nav-main-supporter, .nav-supporter").forEach((link) => { link.textContent = "Supporter"; link.removeAttribute("aria-label"); });
    if (badge) {
      document.body.dataset.supporterRank = String(badge.rank);
      showSupporterRecognition(main, status, display);
      document.querySelectorAll(".nav-main-supporter, .nav-supporter").forEach((navigation) => {
        navigation.replaceChildren(supporterLevelLogo(badge), document.createTextNode(badge.name));
        navigation.setAttribute("aria-label", `${badge.name} · ${badge.family === "living-standard" ? "Living Standard" : "Patron Order"} · Supporter`);
      });
      const anchor = main ? null : document.querySelector(".settings-intro");
      if (!anchor || anchor.querySelector(".supporter-edition-mark")) return;
      const mark = document.createElement("a");
      mark.className = "supporter-edition-mark";
      mark.href = "/supporter";
      mark.append(supporterLevelLogo(badge));
      const label = document.createElement("span");
      label.textContent = `${badge.name} ${badge.family === "living-standard" ? "Living Standard" : "Patron Order"}`;
      mark.append(label);
      anchor.append(mark);
      return;
    }
  } catch (_) {} finally {
    revealSupporterSignature(main);
  }
}

async function supporterShareFile(family) {
  if (!["living-standard", "patron-order"].includes(family)) throw new Error("certificate unavailable");
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
