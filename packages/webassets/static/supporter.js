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
