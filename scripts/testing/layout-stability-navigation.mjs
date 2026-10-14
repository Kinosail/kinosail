// Measure rendered text in the real Player page without changing its layout.
export function personalNavigationSnapshot() {
  const nav = document.querySelector("[data-mobile-tabs]");
  if (!nav) return {present: false, entries: []};
  const visible = node => {
    const box = node.getBoundingClientRect();
    return box.width > 0 && box.height > 0 && (!node.checkVisibility || node.checkVisibility());
  };
  const entries = [...nav.querySelectorAll(":scope > a, :scope > .nav-more > summary")]
    .filter(visible).map(link => {
      const box = link.getBoundingClientRect(), words = [];
      const walker = document.createTreeWalker(link, NodeFilter.SHOW_TEXT);
      for (let node = walker.nextNode(); node; node = walker.nextNode()) {
        for (const match of node.textContent.matchAll(/\S+/g)) {
          const range = document.createRange();
          range.setStart(node, match.index);
          range.setEnd(node, match.index + match[0].length);
          const rects = [...range.getClientRects()].filter(rect => rect.width > 0 && rect.height > 0);
          const lines = new Set(rects.map(rect => Math.round(rect.top))).size;
          const contained = rects.length > 0 && rects.every(rect => rect.left >= box.left - 1 &&
            rect.right <= box.right + 1 && rect.top >= box.top - 1 && rect.bottom <= box.bottom + 1);
          words.push({text: match[0], lines, contained});
        }
      }
      return {label: link.textContent.trim(), words, box: box.toJSON()};
    });
  return {present: visible(nav), ready: nav.classList.contains("has-personal-tabs"), entries};
}

export function personalNavigationReadable(initial, settled, expected) {
  return settled.present && settled.ready &&
    JSON.stringify(settled.entries.map(entry => entry.label)) === JSON.stringify(expected) &&
    [initial, settled].every(state => state.entries.every(entry => entry.words.length > 0 &&
      entry.words.every(word => word.lines === 1 && word.contained)));
}
