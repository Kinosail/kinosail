// hls.js 1.7.3 normalizes fMP4 timestamps without applying MP4 edit lists.
// MSE applies those edits. Reconcile metadata before fragment tracking/appending.
const copiedHLSClock = (Hls, offset) => {
  const limit = 31622400;
  const invalid = () => { throw new Error("Copied media timing is unsupported"); };
  const parse = (bytes) => {
    if (!(bytes instanceof Uint8Array) || !bytes.byteLength || bytes.byteLength > 2 * 1024 * 1024) invalid();
    const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
    const tracks = new Map();
    let count = 0, movies = 0;
    const boxes = (start, end, visit) => {
      for (let at = start; at < end;) {
        if (++count > 4096 || at + 8 > end) invalid();
        const size = view.getUint32(at);
        if (size < 8 || at + size > end) invalid();
        visit(String.fromCharCode(...bytes.subarray(at + 4, at + 8)), at + 8, at + size);
        at += size;
      }
    };
    const timedField = (start, end) => {
      if (start + 4 > end || view.getUint8(start) > 1) invalid();
      const at = start + (view.getUint8(start) === 1 ? 20 : 12);
      if (at + 4 > end) invalid();
      return view.getUint32(at);
    };
    const readTrack = (start, end) => {
      let id, scale, handler, edit = 0;
      const seen = new Set();
      const once = (kind) => { if (seen.has(kind)) invalid(); seen.add(kind); };
      boxes(start, end, (kind, from, to) => {
        if (!["tkhd", "mdia", "edts"].includes(kind)) return;
        once(kind);
        if (kind === "tkhd") id = timedField(from, to);
        else boxes(from, to, (child, first, last) => {
          if (kind === "mdia" && child === "mdhd") { once(child); scale = timedField(first, last); }
          if (kind === "mdia" && child === "hdlr") {
            once(child);
            if (first + 12 > last || view.getUint32(first) !== 0) invalid();
            handler = view.getUint32(first + 8);
          }
          if (kind === "edts" && child === "elst") {
            once(child);
            if (first + 8 > last) invalid();
            const version = view.getUint8(first), width = version === 1 ? 20 : 12;
            if (version > 1 || view.getUint32(first) !== version * 16777216 ||
                first + 8 + width !== last || view.getUint32(first + 4) !== 1) invalid();
            edit = version === 1 ? Number(view.getBigInt64(first + 16)) : view.getInt32(first + 12);
            if (!Number.isSafeInteger(edit) || edit < 0 || view.getUint32(last - 4) !== 65536) invalid();
          }
        });
      });
      if (!id || !scale || tracks.has(id) || tracks.size >= 32 || edit / scale > limit ||
          ![0x76696465, 0x736f756e].includes(handler) || seen.has("edts") && !seen.has("elst")) invalid();
      tracks.set(id, {scale, edit: edit / scale, video: handler === 0x76696465});
    };
    boxes(0, bytes.length, (kind, start, end) => {
      if (kind !== "moov") return;
      if (++movies !== 1) invalid();
      boxes(start, end, (kind, start, end) => { if (kind === "trak") readTrack(start, end); });
    });
    if (movies !== 1 || [...tracks.values()].filter(track => track.video).length !== 1) invalid();
    return tracks;
  };
  const shift = (value, delta) => {
    if (!Number.isFinite(value) || !Number.isFinite(delta) || Math.abs(value + delta) > limit * 2) invalid();
    return value + delta;
  };
  let edits;
  return class extends Hls.DefaultConfig.streamController {
    _handleTransmuxComplete(result) {
      const context = this.getCurrentContext(result.chunkMeta);
      if (!context || this.fragContextChanged(context.frag)) return super._handleTransmuxComplete(result);
      try {
        const remux = result.remuxResult, init = remux.initSegment;
        const initializations = Object.values(init?.tracks || {}).filter(track => track.initSegment !== undefined);
        if (initializations.length > 1) invalid();
        if (initializations.length) edits = parse(initializations[0].initSegment);
        if (!init && (remux.video || remux.audio)) invalid();
        if (init && (remux.video || remux.audio || Number.isFinite(init.initPTS))) {
          const selected = edits?.get(init.trackId), video = edits && [...edits.values()].find(track => track.video);
          if (!selected || !video || selected.scale !== init.timescale || !Number.isFinite(init.initPTS) ||
              !Number.isFinite(offset) || offset < 0 || offset > limit) invalid();
          const delta = offset + init.initPTS / init.timescale - selected.edit;
          const updated = {...remux, initSegment: {...init, initPTS: -offset * init.timescale}};
          for (const kind of ["video", "audio"]) if (remux[kind]) {
            const track = {...remux[kind]};
            for (const key of ["startPTS", "endPTS", "startDTS", "endDTS"]) track[key] = shift(track[key], delta);
            if (track.firstKeyFramePTS !== undefined) {
              const ticks = track.firstKeyFramePTS * init.timescale + init.initPTS;
              track.firstKeyFramePTS = shift(ticks / video.scale, offset - video.edit);
            }
            updated[kind] = track;
          }
          for (const kind of ["id3", "text"]) if (remux[kind]) {
            const samples = remux[kind].samples;
            if (!Array.isArray(samples) || samples.length > 65536) invalid();
            updated[kind] = {...remux[kind], samples: samples.map(sample => {
              const shifted = {...sample};
              for (const key of ["pts", "dts"]) if (sample[key] !== undefined) shifted[key] = shift(sample[key], delta);
              return shifted;
            })};
          }
          Object.assign(remux, updated);
        }
      } catch (_) {
        this.hls.stopLoad();
        this.hls.trigger(Hls.Events.ERROR, {type: Hls.ErrorTypes.MEDIA_ERROR,
          details: Hls.ErrorDetails.FRAG_PARSING_ERROR, fatal: true, error: new Error("Copied media timing is unsupported")});
        return;
      }
      return super._handleTransmuxComplete(result);
    }
  };
};
