(() => {
  "use strict";
  window.kinosailSubtitleSave = ({ item, base, csrf }) => {
    const tasks = new Set();
    let pending, restorePending, restoring = false, generation = 0;
    const now = () => performance.now();
    const message = "Save completion is unknown. Changes may have been written. Your edit is kept; check current subtitle and History before trying again.";
    const failure = () => new Error(message);
    const uncertain = (review, extra = {}) => ({ kind: "unconfirmed", message, review, ...extra });
    const restoreUncertain = (review, extra = {}) => ({ kind: "unconfirmed", review,
      message: "Restore completion is unknown. Changes may have been written. Your edit is kept; check Restore status and History before trying again.", ...extra });
    async function json(path, end, body, operation) {
      if (end <= now()) throw failure();
      const controller = typeof AbortController === "function" ? new AbortController() : undefined;
      let timer, cancel;
      const interrupted = new Promise((_, reject) => {
        cancel = () => { controller?.abort(); reject(failure()); };
        timer = setTimeout(cancel, end - now());
      });
      tasks.add(cancel);
      const options = { credentials: "same-origin", cache: "no-store", signal: controller?.signal };
      if (body !== undefined) Object.assign(options, { method: "POST", body,
        headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf(),
          ...(operation ? { "X-Kinosail-Operation": operation } : {}) } });
      try {
        return await Promise.race([interrupted, (async () => {
          const response = await fetch(path, options);
          const data = await response.json();
          if (now() >= end) throw failure();
          if (!response.ok) {
            const error = failure(); error.status = response.status;
            error.stepUpRequired = data?.stepUpRequired === true; throw error;
          }
          return { status: response.status, data };
        })()]);
      } finally { clearTimeout(timer); tasks.delete(cancel); }
    }
    function validateReceipt(data, id, action = "apply") {
      if (!data || typeof data !== "object" || data.action !== action || data.item !== item ||
        typeof data.id !== "string" || !/^[a-f0-9]{64}$/.test(data.id) || id && data.id !== id ||
        !["prepared", "running", "completed", "unknown"].includes(data.state)) throw failure();
      if (data.state === "completed" && (!["success", "failed"].includes(data.outcome) ||
        !Number.isInteger(data.status) || data.status < 200 || data.status > 599 ||
        (data.outcome === "success") !== (data.status >= 200 && data.status < 300))) throw failure();
      if (data.state === "unknown" && (data.outcome !== "uncertain" || data.status !== undefined) ||
        data.state === "prepared" && (data.outcome !== undefined || data.status !== undefined) ||
        data.state === "running" && (data.status !== undefined || data.outcome !== undefined && data.outcome !== "cancellation-requested")) throw failure();
      return data;
    }
    async function receipt(operation, end) {
      try {
        const result = await json("/api/v1/subtitle-operations/" + operation.id, end);
        if (result.status !== 200) throw failure();
        return validateReceipt(result.data, operation.id, operation.action);
      } catch (error) {
        if (error.status === 404) return { state: "unavailable" };
        throw error;
      }
    }
    function validCues(cues) {
      return Array.isArray(cues) && cues.every(cue => cue && Number.isFinite(cue.start) &&
        Number.isFinite(cue.end) && cue.start >= 0 && cue.end > cue.start && typeof cue.text === "string" &&
        (cue.warnings === undefined || Array.isArray(cue.warnings) && cue.warnings.every(value => typeof value === "string")));
    }
    function validDocument(document) {
      if (!document || !validCues(document.cues) || !document.quality) return false;
      const quality = document.quality;
      return Number.isInteger(quality.cueCount) && quality.cueCount === document.cues.length &&
        ["fastCues", "overlaps", "longLines"].every(key => Number.isInteger(quality[key]) && quality[key] >= 0 && quality[key] <= quality.cueCount) &&
        ["maxCPS", "firstCue", "lastCue"].every(key => Number.isFinite(quality[key]) && quality[key] >= 0) &&
        typeof quality.timing === "string" && typeof quality.completeness === "string";
    }
    function validateReview(data, language) {
      if (!data || data.id !== item || data.language !== language ||
        typeof data.fingerprint !== "string" || !/^(?:missing|[a-f0-9]{64})$/.test(data.fingerprint) ||
        !["translation", "captions"].includes(data.role) || !["title", "source", "matchEvidence"].every(key => typeof data[key] === "string") ||
        typeof data.originalAvailable !== "boolean" || typeof data.restorable !== "boolean" ||
        !Array.isArray(data.warnings) || !data.warnings.every(value => typeof value === "string") ||
        !Number.isFinite(data.duration) || data.duration < 0 || data.proposed !== undefined ||
        data.current !== undefined && !validDocument(data.current)) throw failure();
      return data;
    }
    async function inspect(language, end) {
      const result = await json(base + "/inspect?language=" + encodeURIComponent(language), end);
      if (result.status !== 200) throw failure();
      return validateReview(result.data, language);
    }
    async function snapshot(operation, end) {
      const [review, history] = await Promise.all([
        inspect(operation.language, end), json("/api/v1/subtitle-library?view=history", end),
      ]);
      const data = history.data;
      if (history.status !== 200 || !data || data.pageSize !== 20 || !Number.isInteger(data.matched) ||
        data.matched < 0 || data.history !== undefined && !Array.isArray(data.history)) throw failure();
      return review;
    }
    function matches(current, proposed) {
      return !!current && !!proposed && current.cues.length === proposed.cues.length &&
        current.cues.every((cue, index) => {
          const expected = proposed.cues[index];
          return Math.round(cue.start * 1000) === Math.round(expected.start * 1000) &&
            Math.round(cue.end * 1000) === Math.round(expected.end * 1000) && cue.text === expected.text;
        });
    }
    async function wait(end) {
      if (end <= now()) throw failure();
      let timer, cancel;
      try {
        await new Promise((resolve, reject) => {
          cancel = () => reject(failure()); tasks.add(cancel);
          timer = setTimeout(resolve, Math.min(250, end - now()));
        });
      } finally { clearTimeout(timer); tasks.delete(cancel); }
    }
    async function confirm(operation, end, proposed, poll = false) {
      const restoring = operation.action === "restore", unknown = restoring ? restoreUncertain : uncertain;
      let state = await receipt(operation, end);
      while (poll && state.state === "running") {
        await wait(end);
        state = await receipt(operation, end);
      }
      if (state.state === "running") return unknown(undefined, {
        message: `${restoring ? "Restore" : "Save"} is still running on the Server; completion is not confirmed. Your edit is kept; check status before trying again.`,
      });
      if (state.state === "completed" && state.outcome === "success") {
        if (restoring && state.status !== 204) throw failure();
        const review = await inspect(operation.language, end);
        if (proposed !== undefined && !matches(review.current, proposed)) return {
          kind: "review-required", review,
          message: "The Save completed, but the current subtitle has changed. Your edit is kept; preview again before saving.",
        };
        if (restoring) return { kind: "restored", review, message: "Previous subtitle restored. Your edit is kept." };
        operation.success = true;
        if (proposed !== undefined && operation.epoch === generation && operation.priorRestore === restorePending &&
          restorePending?.language === operation.language) restorePending = undefined;
        return { kind: "saved", review, message: "Subtitle saved. Your edit is protected from automatic upgrades." };
      }
      return unknown(await snapshot(operation, end));
    }
    async function retryGuard(values, end) {
      const previous = pending;
      if (!previous?.dispatched) return null;
      const state = await receipt(previous, end);
      const review = await snapshot({ ...previous, language: values.language }, end);
      if (state.state === "running") return uncertain(review, {
        message: "The previous Save is still running; completion is not confirmed. Your edit is kept; check Save status before trying again.",
      });
      if (values.fingerprint === review.fingerprint &&
        (previous.success || previous.retryFingerprint === review.fingerprint)) return null;
      previous.retryFingerprint = review.fingerprint;
      return { kind: "review-required", review,
        message: "Current subtitle and History checked. Your edit is kept; preview again before explicitly saving." };
    }
    async function save(values, proposed, release) {
      const started = now(), epoch = generation;
      const prepareEnd = started + 15000, releaseEnd = started + 44000;
      let operation, released = false, checkingPrevious = true;
      const finish = result => {
        if (!released) { released = true; release(result); }
        return result;
      };
      try {
        const guarded = await retryGuard(values, prepareEnd);
        checkingPrevious = false;
        if (guarded) return finish(guarded);
        if (epoch !== generation || !proposed || !validCues(proposed.cues)) throw failure();
        const prepared = await json("/api/v1/subtitle-operations", prepareEnd,
          JSON.stringify({ action: "apply", item }));
        const issued = validateReceipt(prepared.data);
        if (prepared.status !== 201 || issued.state !== "prepared" || epoch !== generation) throw failure();
        operation = { id: issued.id, language: values.language, dispatched: false, success: false, epoch, priorRestore: restorePending };
        pending = operation;
        const body = JSON.stringify(values);
        operation.dispatched = true;
        const activationEnd = Math.min(now() + 30000, releaseEnd);
        const accepted = await json(base + "/apply", activationEnd, body, operation.id);
        validateReceipt(accepted.data, operation.id);
        if (accepted.status !== 202) throw failure();
        return finish(await confirm(operation, activationEnd, proposed, true));
      } catch (error) {
        if (checkingPrevious) return finish(uncertain(undefined, { stepUpRequired: error.stepUpRequired === true }));
        if (!operation?.dispatched) return finish({
          kind: "not-submitted", stepUpRequired: error.stepUpRequired === true,
          message: "Save could not be submitted. Your edit is kept; sign in if needed, then try again.",
        });
        finish(uncertain(undefined, { stepUpRequired: error.stepUpRequired === true }));
        if (epoch !== generation) return uncertain();
        try { return await confirm(operation, now() + 15000, proposed); }
        catch (readError) { return uncertain(undefined, { stepUpRequired: readError.stepUpRequired === true }); }
      }
    }
    async function check(language) {
      if (!pending?.dispatched || pending.language !== language) return null;
      try { return await confirm(pending, now() + 15000); }
      catch (error) { return uncertain(undefined, { stepUpRequired: error.stepUpRequired === true }); }
    }
    async function checkRestore(language) {
      if (!restorePending?.dispatched || restorePending.language !== language) return null;
      try { return await confirm(restorePending, now() + 15000); }
      catch (error) { return restoreUncertain(undefined, { stepUpRequired: error.stepUpRequired === true }); }
    }
    async function restore(language, release) {
      const started = now(), epoch = generation, prepareEnd = started + 15000, releaseEnd = started + 44000;
      let operation, released = false, owned = false;
      const finish = result => { if (!released) { released = true; release(result); } return result; };
      try {
        if (restoring) return finish(restoreUncertain());
        if (restorePending?.dispatched) return finish(await checkRestore(language) || restoreUncertain());
        restoring = owned = true;
        const prepared = await json("/api/v1/subtitle-operations", prepareEnd, JSON.stringify({ action: "restore", item }));
        const issued = validateReceipt(prepared.data, undefined, "restore");
        if (prepared.status !== 201 || issued.state !== "prepared" || epoch !== generation) throw failure();
        operation = { id: issued.id, language, action: "restore", dispatched: true };
        restorePending = operation;
        const activationEnd = Math.min(now() + 30000, releaseEnd);
        const accepted = await json(base + "/restore", activationEnd, JSON.stringify({ language }), operation.id);
        validateReceipt(accepted.data, operation.id, "restore");
        if (accepted.status !== 202) throw failure();
        return finish(await confirm(operation, activationEnd, undefined, true));
      } catch (error) {
        return finish(operation?.dispatched ? restoreUncertain(undefined, { stepUpRequired: error.stepUpRequired === true }) : {
          kind: "not-submitted", stepUpRequired: error.stepUpRequired === true,
          message: "Restore could not be submitted. Your edit is kept; sign in if needed, then try again.",
        });
      } finally { if (owned) restoring = false; }
    }
    function stop() { generation++; for (const cancel of [...tasks]) cancel(); }
    return { save, check, restore, checkRestore, stop };
  };
})();
