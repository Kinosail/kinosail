export function enableDrag(handle, tile, getVersion, saveOrder, announce, restore, refresh) {
  let active = false;
  let moved = false;
  handle.addEventListener("pointerdown", event => {
    if (event.button !== 0) return;
    active = true;
    moved = false;
    tile.dataset.dragging = "true";
    handle.setPointerCapture(event.pointerId);
  });
  handle.addEventListener("pointermove", event => {
    if (!active) return;
    const target = document.elementsFromPoint(event.clientX, event.clientY)
      .map(node => node.closest(".app-tile"))
      .find(candidate => candidate && candidate !== tile && candidate.parentElement === tile.parentElement);
    if (!target || target === tile || target.parentElement !== tile.parentElement) return;
    const box = target.getBoundingClientRect();
    const after = event.clientY > box.top + box.height / 2 || Math.abs(event.clientY - box.top - box.height / 2) < box.height / 3 && event.clientX > box.left + box.width / 2;
    target.parentElement.insertBefore(tile, after ? target.nextSibling : target);
    handle.setPointerCapture(event.pointerId);
    moved = true;
  });
  const finish = async event => {
    if (!active) return;
    active = false;
    tile.dataset.dragging = "false";
    if (handle.hasPointerCapture(event.pointerId)) handle.releasePointerCapture(event.pointerId);
    if (!moved) return;
    const ids = [...tile.parentElement.querySelectorAll(".app-tile")].map(node => node.dataset.id);
    try {
      await saveOrder(ids, getVersion());
      announce("Application order was saved");
    } catch (error) {
      restore();
      if (error.refreshOnly) announce(error.message, "Refresh", refresh, true);
      else announce(error.message, "", null, true);
    }
  };
  handle.addEventListener("pointerup", finish);
  handle.addEventListener("pointercancel", event => {
    if (!active) return;
    active = false;
    tile.dataset.dragging = "false";
    if (handle.hasPointerCapture(event.pointerId)) handle.releasePointerCapture(event.pointerId);
    restore();
    announce("Move cancelled");
  });
}
