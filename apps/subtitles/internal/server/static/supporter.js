function createSupporterSignature(main, {append = false, hide = false, refreshExisting = false} = {}) {
  if (!main) return;
  const existing = main.querySelector(".supporter-signature");
  if (existing) {
    if (refreshExisting) existing.dataset.supporterPending = "true";
    return;
  }
  const signature = document.createElement("aside");
  signature.className = "supporter-signature";
  signature.dataset.supporterPending = "true";
  if (hide) signature.style.visibility = "hidden";
  signature.setAttribute("aria-label", "Kinosail supporter status");
  const copy = document.createElement("p");
  copy.textContent = "Community edition · Free and supporter-funded.";
  const support = document.createElement("a");
  support.href = "/supporter";
  support.textContent = "Support Kinosail";
  signature.append(copy, support);
  const masthead = main.querySelector(".library-masthead");
  if (masthead) {
    masthead.classList.add("has-supporter-signature");
    masthead.append(signature);
  } else main[append ? "append" : "prepend"](signature);
}

function ensureSupporterSignature(main) {
  createSupporterSignature(main, {append: true, refreshExisting: true});
}

function recognizedGrant(status) {
  if (status?.livingStandard?.active) return status.livingStandard;
  if (status?.patronOrder?.active) return status.patronOrder;
  return null;
}

async function bindSupporterRecognition() {
  const main = document.querySelector("main");
  ensureSupporterSignature(main);
  try {
    const response = await fetch("/api/v1/supporter", { headers: { accept: "application/json" } });
    if (!response.ok) {
      const signature = main?.querySelector(".supporter-signature");
      if (signature) delete signature.dataset.supporterPending;
      return;
    }
    const status = await response.json();
    delete document.body.dataset.supporterRank;
    main?.querySelector(".supporter-edition-mark")?.remove();
    const grant = recognizedGrant(status);
    if (grant) {
      document.body.dataset.supporterRank = String(grant.rank);
      const signature = main?.querySelector(".supporter-signature");
      signature?.remove();
      main?.querySelector(".library-masthead")?.classList.remove("has-supporter-signature");
      const anchor = main?.querySelector(".brand-lockup, .settings-intro");
      if (!anchor || anchor.querySelector(".supporter-edition-mark")) return;
      const mark = document.createElement("a");
      mark.className = "supporter-edition-mark";
      mark.href = "/supporter";
      mark.textContent = `${grant.title} · ${grant.name}`;
      mark.setAttribute("aria-label", `${grant.name} ${grant.title} supporter badge`);
      anchor.append(mark);
      return;
    }
    const signature = main?.querySelector(".supporter-signature");
    if (!signature) return;
    const archived = status?.livingStandard?.archived;
    signature.querySelector("p").textContent = archived ? "Community edition · Living Standard archived." : "Community edition · Free and supporter-funded.";
    signature.querySelector("a").textContent = archived ? "Refresh monthly support" : "Support Kinosail";
    delete signature.dataset.supporterPending;
  } catch {
    const signature = main?.querySelector(".supporter-signature");
    if (signature) delete signature.dataset.supporterPending;
  }
}

function certificateDownload(button) {
  const actions = button.closest(".certificate-actions");
  actions?.querySelector("a[download]")?.click();
  const status = actions?.nextElementSibling;
  if (status) status.textContent = "Certificate download started.";
}

async function pngCertificate(url) {
  const response = await fetch(url, { headers: { accept: "image/svg+xml" } });
  if (!response.ok) throw new Error("certificate unavailable");
  const svg = await response.blob();
  if (!svg.type.startsWith("image/svg+xml") || svg.size > 262144) throw new Error("certificate invalid");
  return renderSupporterPNG(svg, "certificate unavailable");
}

function downloadPNG(blob, filename) {
  const anchor = document.createElement("a");
  const objectURL = URL.createObjectURL(blob);
  anchor.href = objectURL;
  anchor.download = filename;
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(objectURL), 0);
}

async function shareCertificate(button) {
  const url = button.dataset.certificateUrl;
  const filename = button.dataset.certificateName || "kinosail-subtitles-supporter.png";
  const status = button.closest(".certificate-actions")?.nextElementSibling;
  if (!url) {
    certificateDownload(button);
    return;
  }
  if (button.disabled) return;
  button.disabled = true;
  if (status) status.textContent = "Preparing share image…";
  let png;
  try {
    png = await pngCertificate(url);
    const file = new File([png], filename, { type: "image/png" });
    if (typeof navigator.share !== "function" || typeof navigator.canShare === "function" && !navigator.canShare({ files: [file] })) {
      downloadPNG(png, filename);
      if (status) status.textContent = "PNG certificate download started.";
      return;
    }
    await navigator.share({ title: "Kinosail Subtitles supporter badge", text: "My Kinosail Subtitles supporter badge", files: [file] });
    if (status) status.textContent = "Certificate shared.";
  } catch (error) {
    if (error?.name === "AbortError") {
      if (status) status.textContent = "";
    } else {
      if (png) {
        downloadPNG(png, filename);
        if (status) status.textContent = "PNG certificate download started.";
      } else certificateDownload(button);
    }
  } finally {
    button.disabled = false;
  }
}

function bindCertificateSharing() {
  for (const button of document.querySelectorAll(".share-certificate:not([data-share-bound])")) {
    button.dataset.shareBound = "true";
    button.addEventListener("click", () => shareCertificate(button));
  }
}

bindSupporterRecognition();
bindCertificateSharing();
document.addEventListener("htmx:afterSwap", () => {
  bindSupporterRecognition();
  bindCertificateSharing();
});
