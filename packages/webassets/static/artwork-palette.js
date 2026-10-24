(() => {
  function bind() {
    const main = document.querySelector("main.last-light");
    if (!main || main.dataset.paletteBound) return;
    main.dataset.paletteBound = "true";
    main.querySelectorAll(".media-hero > .media-backdrop, .home-feature > img").forEach((image) => {
      image.addEventListener("error", () => {
        image.hidden = true;
        image.parentElement?.classList.remove("has-media-backdrop");
      });
      image.addEventListener("load", () => {
        if (!image.hidden || !image.naturalWidth) return;
        image.hidden = false;
        image.parentElement?.classList.add("has-media-backdrop");
      });
    });
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", bind);
  else bind();
  document.addEventListener("htmx:after:swap", bind);
})();
