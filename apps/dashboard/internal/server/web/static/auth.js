const form = document.getElementById("auth-form");
const error = document.getElementById("auth-error");
const mode = document.body.dataset.authMode;

form.addEventListener("submit", async event => {
  event.preventDefault();
  error.hidden = true;
  const data = new FormData(form);
  if (mode === "setup" && data.get("password") !== data.get("confirmPassword")) {
    error.textContent = "Passwords do not match.";
    error.hidden = false;
    return;
  }
  const button = form.querySelector("button[type=submit]");
  button.disabled = true;
  try {
    const response = await fetch(mode === "setup" ? "/api/v1/setup" : "/api/v1/session", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: JSON.stringify({ name: data.get("name"), password: data.get("password"), device: navigator.userAgent.slice(0, 80) || "Browser" })
    });
    const result = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(result.error || "Could not complete the request. Check your details and try again.");
    location.replace(response.headers.get("X-Kinosail-Login-Next") || "/");
  } catch (failure) {
    error.textContent = failure.message;
    error.hidden = false;
  } finally { button.disabled = false; }
});
