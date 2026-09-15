function bindPasswordControls() {
  document.querySelectorAll('input[type="password"]:not([data-password-toggle])').forEach((input) => {
    input.dataset.passwordToggle = "";
    const label = input.closest("label");
    const wrapper = document.createElement("span");
    wrapper.className = "password-control";
    if (label) {
      wrapper.classList.add("has-label");
      label.before(wrapper);
      wrapper.append(label);
    } else {
      input.before(wrapper);
      wrapper.append(input);
    }
    const button = document.createElement("button");
    button.type = "button";
    button.className = "password-toggle";
    button.setAttribute("aria-label", "Show secret");
    button.setAttribute("aria-pressed", "false");
    button.disabled = input.disabled;
    button.addEventListener("click", () => {
      const shown = input.type === "text";
      input.type = shown ? "password" : "text";
      button.setAttribute("aria-label", shown ? "Show secret" : "Hide secret");
      button.setAttribute("aria-pressed", String(!shown));
    });
    wrapper.append(button);
  });
}

function bindCopyControls() {
  document.querySelectorAll("[data-copy-target]").forEach((button) => button.addEventListener("click", async () => {
    const input = document.getElementById(button.dataset.copyTarget);
    const status = button.parentElement?.querySelector(".copy-status");
    if (!input || !status) return;
    try {
      await navigator.clipboard.writeText(input.value);
      status.textContent = button.parentElement.querySelector("[data-copy-success]")?.textContent || "Copied.";
    } catch {
      input.focus();
      input.select();
      status.textContent = button.parentElement.querySelector("[data-copy-fallback]")?.textContent || "Select Copy in your browser.";
    }
  }));
}

function bindDisclosureControls() {
  const openHashDisclosure = () => {
    let id;
    try { id = decodeURIComponent(location.hash.slice(1)); } catch { return; }
    const disclosure = document.getElementById(id);
    if (disclosure instanceof HTMLDetailsElement) disclosure.open = true;
  };
  openHashDisclosure();
  addEventListener("hashchange", openHashDisclosure);
  document.querySelectorAll("[data-open-disclosure]").forEach((link) => link.addEventListener("click", (event) => {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || !(link instanceof HTMLAnchorElement)) return;
    const disclosure = document.getElementById(link.dataset.openDisclosure);
    const summary = disclosure?.querySelector(":scope > summary");
    if (!(disclosure instanceof HTMLDetailsElement) || !summary) return;
    event.preventDefault();
    disclosure.open = true;
    history[location.hash === link.hash ? "replaceState" : "pushState"](null, "", link.hash);
    disclosure.scrollIntoView({ block: "start" });
    summary.focus({ preventScroll: true });
  }));
}

function bindTrustedHTTPSControls() {
  document.querySelectorAll("[data-trusted-https-test]").forEach((button) => {
    const form = button.closest("form");
    const status = form?.querySelector("[data-trusted-https-test-status]");
    if (!form || !status) return;
    let generation = 0;
    let validationTimer;
    const csrf = document.querySelector('meta[name="kinosail-csrf"]')?.content;
    const fields = () => {
      const data = new FormData(form);
      return { provider: data.get("provider"), domain: data.get("domain"), token: data.get("token"), address: data.get("address"), termsAccepted: data.get("termsAccepted") === "true" };
    };
    const request = async (path, method = "POST") => {
      const response = await fetch(path, {
        method,
        headers: { "Content-Type": "application/json", ...(csrf ? { "X-Kinosail-CSRF": csrf } : {}) },
        body: JSON.stringify(fields()),
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.error?.replaceAll("\n", " — ") || "Trusted HTTPS validation failed");
      return result;
    };
    const complete = () => form.checkValidity() && [...form.querySelectorAll("input[required], select[required]")].every((field) => field.type === "checkbox" ? field.checked : field.value.trim());
    const show = (message, state) => {
      status.textContent = message;
      if (state) status.dataset.state = state;
      else delete status.dataset.state;
    };
    const validate = async () => {
      if (!complete()) return;
      const requestGeneration = generation;
      show("Checking these details…");
      try {
        const result = await request("/api/v1/settings/trusted-https/validate");
        if (requestGeneration === generation) show(`Details match this deployment for ${result.trustedHttps.hostname}. DNS has not been tested.`);
      } catch (error) {
        if (requestGeneration === generation) show(error instanceof Error ? error.message : "Trusted HTTPS validation failed", "failed");
      }
    };
    form.addEventListener("input", () => {
      generation++;
      clearTimeout(validationTimer);
      show(status.dataset.state === "passed" ? "Details changed. Test again." : "");
      if (complete()) validationTimer = setTimeout(validate, 450);
    });
    button.addEventListener("click", async () => {
      if (!form.reportValidity()) return;
      clearTimeout(validationTimer);
      const requestGeneration = ++generation;
      button.disabled = true;
      form.setAttribute("aria-busy", "true");
      show("Testing DNS connection…");
      try {
        const result = await request("/api/v1/settings/trusted-https/test");
        if (!result.trustedHttps?.hostname) throw new Error("DNS provider test failed");
        if (requestGeneration !== generation) return;
        show(`Connection verified for ${result.trustedHttps.hostname}. Nothing was saved.`, "passed");
      } catch (error) {
        if (requestGeneration !== generation) return;
        show(error instanceof Error ? error.message : "DNS provider test failed", "failed");
      } finally {
        button.disabled = false;
        form.removeAttribute("aria-busy");
      }
    });
    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      if (!form.reportValidity()) return;
      clearTimeout(validationTimer);
      generation++;
      const submit = event.submitter;
      if (submit instanceof HTMLButtonElement) submit.disabled = true;
      button.disabled = true;
      form.setAttribute("aria-busy", "true");
      show("Checking and saving…");
      try {
        await request("/api/v1/settings/trusted-https", "PUT");
        location.assign(form.action.includes("/onboarding/") ? "/onboarding/connection" : "/settings#trusted-https");
      } catch (error) {
        show(error instanceof Error ? error.message : "Trusted HTTPS could not be saved", "failed");
        if (submit instanceof HTMLButtonElement) submit.disabled = false;
        button.disabled = false;
        form.removeAttribute("aria-busy");
      }
    });
  });
}

function bindDocumentControls() {
  bindPasswordControls();
  bindCopyControls();
  bindDisclosureControls();
  bindTrustedHTTPSControls();
  document.querySelectorAll("[data-curation-menu]").forEach((menu) => {
    const search = menu.querySelector("[data-curation-search]");
    const empty = menu.querySelector("[data-curation-empty]");
    search?.addEventListener("input", () => {
      const query = search.value.trim().toLocaleLowerCase();
      let matches = 0;
      menu.querySelectorAll("[data-curation-option]").forEach((option) => {
        option.hidden = !option.querySelector("button span").textContent.toLocaleLowerCase().includes(query);
        if (!option.hidden) matches++;
      });
      menu.querySelectorAll(".curation-options section").forEach((section) => { section.hidden = !section.querySelector("[data-curation-option]:not([hidden])"); });
      empty.hidden = matches > 0;
    });
  });
}

if (document.readyState === "loading") addEventListener("DOMContentLoaded", bindDocumentControls, { once: true });
else bindDocumentControls();
