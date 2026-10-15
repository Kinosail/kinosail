const copiedHLSClock = (Hls, offset) => {
  const limit = 31622400;
  const invalid = () => { throw new Error("Copied media timing is unsupported"); };
  const inspect = (bytes, maximum) => {
    if (!(bytes instanceof Uint8Array) || !bytes.byteLength || bytes.byteLength > maximum) invalid();
    const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
    let count = 0;
    const boxes = (start, end, visit) => {
      for (let at = start; at < end;) {
        if (++count > 4096 || at + 8 > end) invalid();
        const size = view.getUint32(at);
        if (size < 8 || at + size > end) invalid();
        visit(String.fromCharCode(...bytes.subarray(at + 4, at + 8)), at + 8, at + size);
        at += size;
      }
    };
    return {view, boxes};
  };
  const parse = (bytes) => {
    const {view, boxes} = inspect(bytes, 2 * 1024 * 1024);
    const tracks = new Map(), patches = [];
    let movies = 0, movieScale;
    const timedField = (start, end) => {
      if (start + 4 > end || view.getUint8(start) > 1) invalid();
      const at = start + (view.getUint8(start) === 1 ? 20 : 12);
      if (at + 4 > end) invalid();
      return view.getUint32(at);
    };
    const readTrack = (start, end) => {
      let id, scale, handler, edit = 0, delay = 0;
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
            const version = view.getUint8(first), width = version === 1 ? 20 : 12, count = view.getUint32(first + 4);
            if (version > 1 || view.getUint32(first) !== version * 16777216 ||
                ![1, 2].includes(count) || first + 8 + width * count !== last) invalid();
            const mediaTime = (at) => version === 1 ? Number(view.getBigInt64(at + 8)) : view.getInt32(at + 4);
            if (count === 2) {
              delay = version === 1 ? Number(view.getBigUint64(first + 8)) : view.getUint32(first + 8);
              if (!movieScale || !Number.isSafeInteger(delay) || delay <= 0 || delay / movieScale > limit ||
                  mediaTime(first + 8) !== -1 || view.getUint32(first + 8 + width - 4) !== 65536) invalid();
              patches.push({first, width});
            }
            edit = mediaTime(first + 8 + (count - 1) * width);
            if (!Number.isSafeInteger(edit) || edit < 0 || view.getUint32(last - 4) !== 65536) invalid();
          }
        });
      });
      if (!id || !scale || tracks.has(id) || tracks.size >= 32 || edit / scale > limit ||
          ![0x76696465, 0x736f756e].includes(handler) || seen.has("edts") && !seen.has("elst")) invalid();
      const product = BigInt(delay) * BigInt(scale), divisor = BigInt(movieScale || 1);
      if (product % divisor || product / divisor > BigInt(Number.MAX_SAFE_INTEGER)) invalid();
      const lead = Number(product / divisor);
      tracks.set(id, {scale, edit: (edit - lead) / scale, lead, video: handler === 0x76696465});
    };
    boxes(0, bytes.length, (kind, start, end) => {
      if (kind !== "moov") return;
      if (++movies !== 1) invalid();
      boxes(start, end, (kind, from, to) => {
        if (kind === "mvhd") { if (movieScale !== undefined) invalid(); movieScale = timedField(from, to); }
      });
      boxes(start, end, (kind, start, end) => { if (kind === "trak") readTrack(start, end); });
    });
    if (movies !== 1 || [...tracks.values()].filter(track => track.video).length !== 1) invalid();
    let normalized = bytes;
    if (patches.length) {
      normalized = new Uint8Array(bytes);
      const target = new DataView(normalized.buffer, normalized.byteOffset, normalized.byteLength);
      for (const {first, width} of patches) {
        normalized.copyWithin(first + 8, first + 8 + width, first + 8 + 2 * width);
        target.setUint32(first - 8, 16 + width); target.setUint32(first + 4, 1);
        target.setUint32(first + 8 + width, width);
        normalized.set([102, 114, 101, 101], first + 12 + width);
      }
    }
    return {tracks, normalized};
  };
  const normalizeMedia = (bytes, tracks) => {
    const {view, boxes} = inspect(bytes, 256 * 1024 * 1024), patches = [];
    let movies = 0;
    boxes(0, bytes.length, (kind, start, end) => {
      if (kind !== "moof") return;
      movies++;
      const seen = new Set();
      boxes(start, end, (kind, start, end) => {
        if (kind !== "traf") return;
        let id, clock;
        boxes(start, end, (kind, from, to) => {
          if (kind === "tfhd") {
            if (id !== undefined || from + 8 > to) invalid();
            id = view.getUint32(from + 4);
          }
          if (kind === "tfdt") {
            if (clock || from + 4 > to) invalid();
            const version = view.getUint8(from);
            if (version > 1 || view.getUint32(from) !== version * 16777216 || from + (version ? 12 : 8) !== to) invalid();
            clock = {at: from + 4, version};
          }
        });
        const track = tracks.get(id);
        if (!track || !clock || seen.has(id)) invalid();
        seen.add(id);
        if (track.lead) {
          const time = (clock.version ? view.getBigUint64(clock.at) : BigInt(view.getUint32(clock.at))) + BigInt(track.lead);
          if (time > (clock.version ? 18446744073709551615n : 4294967295n)) invalid();
          patches.push({...clock, time});
        }
      });
      if (!seen.size) invalid();
    });
    if (!movies) invalid();
    const normalized = new Uint8Array(bytes), target = new DataView(normalized.buffer, normalized.byteOffset, normalized.byteLength);
    for (const {at, version, time} of patches) {
      if (version) target.setBigUint64(at, time); else target.setUint32(at, Number(time));
    }
    return normalized;
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
      let updated;
      try {
        const remux = result.remuxResult, init = remux.initSegment;
        let nextEdits = edits;
        updated = {...remux};
        const initializations = Object.entries(init?.tracks || {}).filter(([, track]) => track.initSegment !== undefined);
        if (initializations.length > 1) invalid();
        if (initializations.length) {
          const [name, track] = initializations[0], parsed = parse(track.initSegment);
          nextEdits = parsed.tracks;
          updated.initSegment = {...init, tracks: {...init.tracks, [name]: {...track, initSegment: parsed.normalized}}};
        }
        if (!init && (remux.video || remux.audio)) invalid();
        if (init && (remux.video || remux.audio || Number.isFinite(init.initPTS))) {
          const selected = nextEdits?.get(init.trackId), video = nextEdits && [...nextEdits.values()].find(track => track.video);
          if (!selected || !video || selected.scale !== init.timescale || !Number.isFinite(init.initPTS) ||
              !Number.isFinite(offset) || offset < 0 || offset > limit) invalid();
          const delta = offset + init.initPTS / init.timescale - selected.edit;
          updated.initSegment = {...updated.initSegment, initPTS: -offset * init.timescale};
          for (const kind of ["video", "audio"]) if (remux[kind]) {
            const track = {...remux[kind]};
            if ([...nextEdits.values()].some(track => track.lead)) {
              if (track.data2 !== undefined) invalid();
              track.data1 = normalizeMedia(track.data1, nextEdits);
            }
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
              const cueDelta = kind === "text" ? offset + init.initPTS / init.timescale - video.edit : delta;
              for (const key of ["pts", "dts"]) if (sample[key] !== undefined) shifted[key] = shift(sample[key], cueDelta);
              return shifted;
            })};
          }
        }
        edits = nextEdits;
      } catch (_) {
        this.hls.stopLoad();
        this.hls.trigger(Hls.Events.ERROR, {type: Hls.ErrorTypes.MEDIA_ERROR,
          details: Hls.ErrorDetails.FRAG_PARSING_ERROR, fatal: true, reason: "copied-media-timing", error: new Error("Copied media timing is unsupported")});
        return;
      }
      return super._handleTransmuxComplete({...result, remuxResult: updated});
    }
  };
};
