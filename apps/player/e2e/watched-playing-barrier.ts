export function installWatchedPlayingBarrier() {
  const add = EventTarget.prototype.addEventListener, remove = EventTarget.prototype.removeEventListener;
  const pending: Array<() => void> = [];
  const listeners = new WeakMap<EventTarget, Map<EventListenerOrEventListenerObject, Map<boolean, EventListener>>>();
  const control = {pending, registered: 0, release() {
    const notifications = pending.splice(0);
    for (const notify of notifications) notify();
    return notifications.length;
  }};
  Object.assign(window, {watchedStartup: control});
  EventTarget.prototype.addEventListener = function(type, listener, options) {
    const capture = typeof options === "boolean" ? options : options?.capture ?? false;
    const existing = type === "playing" && listener ? listeners.get(this)?.get(listener)?.get(capture) : undefined;
    if (existing) return add.call(this, type, existing, options);
    const script = document.currentScript as HTMLScriptElement | null;
    const source = script?.src ? new URL(script.src) : undefined;
    // Delay only the page-owned callbacks installed by its application script.
    // HLS controller events and later registrations retain native dispatch.
    if (!(this instanceof HTMLVideoElement) || type !== "playing" || !listener ||
        source?.origin !== location.origin || source.pathname !== "/static/player.js") return add.call(this, type, listener, options);
    let targetListeners = listeners.get(this);
    if (!targetListeners) {targetListeners = new Map(); listeners.set(this, targetListeners);}
    let captures = targetListeners.get(listener);
    if (!captures) {captures = new Map(); targetListeners.set(listener, captures);}
    let wrapped = captures.get(capture);
    if (!wrapped) {
      const target = this;
      wrapped = event => pending.push(() => {
        if (typeof listener === "function") listener.call(target, event);
        else listener.handleEvent(event);
      });
      captures.set(capture, wrapped);
      control.registered++;
    }
    return add.call(this, type, wrapped, options);
  };
  EventTarget.prototype.removeEventListener = function(type, listener, options) {
    const capture = typeof options === "boolean" ? options : options?.capture ?? false;
    const wrapped = type === "playing" && listener ? listeners.get(this)?.get(listener)?.get(capture) : undefined;
    return remove.call(this, type, wrapped ?? listener, options);
  };
}
