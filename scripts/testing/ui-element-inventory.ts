import { writeFile } from "node:fs/promises";

// DOM evidence, not a substitute for an accessibility tree or human review.
// Deliberately excludes field values, URLs, and arbitrary status text.
export function uiElementInventory() {
  const selector =
    "button,input:not([type=hidden]),select,textarea,a[href],summary,dialog,nav,video,audio,progress,img,h1,h2,h3,[role],[tabindex]";
  return [...document.querySelectorAll(selector)].map((element, index) => {
    const box = element.getBoundingClientRect();
    const style = getComputedStyle(element);
    const labels = (element as HTMLInputElement).labels;
    const labelledBy = element
      .getAttribute("aria-labelledby")
      ?.split(/\s+/)
      .map((id) => document.getElementById(id)?.textContent ?? "")
      .join(" ");
    const text = element.matches("button,a,summary,h1,h2,h3")
      ? element.textContent
      : "";
    const label =
      labelledBy ||
      element.getAttribute("aria-label") ||
      (labels ? [...labels].map((label) => label.textContent).join(" ") : "") ||
      element.getAttribute("alt") ||
      text ||
      "";
    return {
      index,
      tag: element.tagName.toLowerCase(),
      id: element.id,
      role: element.getAttribute("role"),
      type: element.getAttribute("type"),
      label: label.trim().replace(/\s+/g, " ").slice(0, 160),
      visible:
        box.width > 0 &&
        box.height > 0 &&
        style.visibility !== "hidden" &&
        style.display !== "none" &&
        !element.closest("[hidden],dialog:not([open])"),
      disabled: element.matches(":disabled,[aria-disabled=true]"),
      checked: element.matches(":checked,[aria-checked=true]"),
      expanded: element.getAttribute("aria-expanded"),
      busy: element.getAttribute("aria-busy"),
      focused: element === document.activeElement,
      width: Math.round(box.width),
      height: Math.round(box.height),
    };
  });
}
export async function uiElementAttachment(
  path: string,
  elements: ReturnType<typeof uiElementInventory>,
) {
  await writeFile(path, JSON.stringify(elements, null, 2));
  return { path, contentType: "application/json" };
}
