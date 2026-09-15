const inputs = [...document.querySelectorAll("[data-quick-connect-digit]")];
const form = inputs[0]?.form;
const value = form?.querySelector('input[name="code"]');
let submitted = false;

function sync() {
  if (!value) return;
  value.value = inputs.map((input) => input.value).join("");
  if (form && value.value.length === inputs.length && !submitted) {
    submitted = true;
    form.requestSubmit();
  }
}

function setDigits(raw, start = 0) {
  const digits = raw.replace(/\D/g, "");
  const offset = digits.length > 1 ? 0 : start;
  if (offset === 0) inputs.forEach((input) => { input.value = ""; });
  digits.slice(0, inputs.length - offset).split("").forEach((digit, position) => {
    inputs[offset + position].value = digit;
  });
  sync();
  if (digits.length && value && value.value.length < inputs.length) {
    inputs[Math.min(offset + digits.length, inputs.length - 1)].focus();
  }
}

inputs.forEach((input, index) => {
  input.addEventListener("input", () => {
    input.value = input.value.replace(/\D/g, "").slice(-1);
    sync();
    if (input.value && index < inputs.length - 1) inputs[index + 1].focus();
  });
  input.addEventListener("keydown", (event) => {
    if (event.key === "Backspace" && !input.value && index > 0) inputs[index - 1].focus();
    if (event.key === "ArrowLeft" && index > 0) inputs[index - 1].focus();
    if (event.key === "ArrowRight" && index < inputs.length - 1) inputs[index + 1].focus();
  });
  input.addEventListener("paste", (event) => {
    event.preventDefault();
    setDigits(event.clipboardData?.getData("text") ?? "", index);
  });
});
