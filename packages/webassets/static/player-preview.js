const createSeekPreview = (seek, preview) => {
  preview?.setAttribute("aria-hidden", "true");
  const frame = preview?.querySelector("[data-seek-frame]");
  const time = preview?.querySelector("[data-preview-time]");
  const frames = new Map();
  let wanted = "";
  let loading = false;
  let scheduled;
  const display = (image) => {
    frame.replaceChildren(...(image ? [image] : []));
    frame.hidden = !image && !(wanted && loading);
    frame.setAttribute("aria-busy", String(Boolean(wanted && !image && loading)));
  };
  if (frame) display();
  const load = () => {
    scheduled = undefined;
    if (!wanted || loading) return;
    const url = wanted;
    const cached = frames.get(url);
    if (cached) {
      frames.delete(url);
      frames.set(url, cached);
      display(cached);
      return;
    }
    loading = true;
    display();
    const image = new Image();
    image.alt = "";
    const finish = (success) => {
      loading = false;
      if (success) {
        frames.set(url, image);
        if (frames.size > 12) frames.delete(frames.keys().next().value);
      }
      if (wanted === url) display(success ? image : null);
      else load();
    };
    image.onload = () => finish(true);
    image.onerror = () => finish(false);
    image.src = url;
  };
  const hide = () => {
    cancelAnimationFrame(scheduled);
    scheduled = undefined;
    wanted = "";
    if (preview) { preview.hidden = true; display(); }
  };
  const show = (position) => {
    const duration = Number(seek?.max);
    if (!preview || !Number.isFinite(position) || !Number.isFinite(duration) || !(duration > 0)) return;
    const target = Math.max(0, Math.min(position, duration));
    preview.hidden = false;
    time.textContent = formatTime(target);
    preview.style.setProperty("--preview-progress", `${target / duration * 100}%`);
    const url = target <= 43200 && seek.dataset.trickplay ? seek.dataset.trickplay.replace("{second}", String(Math.floor(target / 10) * 10)) : "";
    if (wanted === url) return;
    wanted = url;
    const cached = frames.get(url);
    display(cached);
    if (scheduled === undefined) scheduled = requestAnimationFrame(load);
  };
  return { show, hide };
};
