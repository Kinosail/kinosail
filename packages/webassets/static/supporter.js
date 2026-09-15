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

async function renderSupporterPNG(certificate, errorMessage) {
  const source = await new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.addEventListener("load", () => resolve(reader.result), {once: true});
    reader.addEventListener("error", () => reject(new Error(errorMessage)), {once: true});
    reader.readAsDataURL(certificate);
  });
  const image = new Image();
  image.src = source;
  await image.decode();
  const canvas = document.createElement("canvas");
  canvas.width = 1200;
  canvas.height = 630;
  const context = canvas.getContext("2d");
  if (!context) throw new Error(errorMessage);
  context.drawImage(image, 0, 0, canvas.width, canvas.height);
  return new Promise((resolve, reject) => canvas.toBlob((result) => result ? resolve(result) : reject(new Error(errorMessage)), "image/png"));
}
