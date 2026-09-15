function bindSupporterPlans() {
  const controls = document.querySelector(".supporter-billing");
  if (!controls) return;
  const plans = {
    monthly: { amounts: [3, 5, 8, 12, 18, 25, 35, 45, 60, 75], period: "/month", note: "Monthly support · Living Standard badge, active while your support is current." },
    annual: { amounts: [12, 40, 64, 96, 144, 200, 280, 360, 480, 600], period: "/year", note: "Annual support · One payment per year, with a Living Standard badge." },
    once: { amounts: [5, 15, 30, 60, 100, 150, 250, 400, 550, 750], period: " once", note: "One-time support · Patron Order badge, yours permanently. No subscription." },
  };
  function render() {
    const frequency = controls.querySelector("input:checked")?.value;
    if (!Object.hasOwn(plans, frequency)) return;
    const plan = plans[frequency];
    document.querySelector("[data-supporter-billing-note]").textContent = plan.note;
    const family = frequency === "once" ? "patron-order" : "living-standard";
    document.querySelectorAll("[data-supporter-family]").forEach((panel) => {
      panel.hidden = panel.dataset.supporterFamily !== family;
      if (panel.hidden) return;
      panel.querySelectorAll("[data-supporter-price]").forEach((amount) => {
        const rank = Number(amount.dataset.supporterPrice);
        if (!Number.isInteger(rank) || rank < 1 || rank > plan.amounts.length) return;
        const period = document.createElement("small");
        period.textContent = plan.period;
        amount.replaceChildren(document.createTextNode(`$${plan.amounts[rank - 1]}`), period);
      });
    });
  }

  controls.addEventListener("change", render);
  render();
  controls.hidden = false;
}

bindSupporterPlans();

const supporterReplay = document.querySelector("[data-supporter-replay]");
if (supporterReplay) {
  supporterReplay.hidden = false;
  supporterReplay.addEventListener("click", () => {
    const art = document.querySelector(".supporter-honor-art .supporter-badge");
    if (!art || matchMedia("(prefers-reduced-motion: reduce)").matches || !art.animate) return;
    art.getAnimations().forEach((animation) => animation.cancel());
    art.animate([{ opacity: 0, transform: "translateY(8px)" }, { opacity: 1, transform: "none" }], { duration: 240, easing: "ease-out" });
  });
}
