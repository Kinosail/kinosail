// Preparation and playback must select the same decoder and rendition.
window.kinosailPlaybackCapabilities = (() => {
  const codecs = [
    ["av1", 'video/mp4; codecs="av01.0.08M.08"'],
    ["hevc", 'video/mp4; codecs="hvc1.1.6.L123.B0"'],
    ["vp9", 'video/mp4; codecs="vp09.00.10.08"'],
    ["h264", 'video/mp4; codecs="avc1.64002a"'],
  ];
  // Safari keeps its native HLS path. Chromium's HLS canPlayType result alone
  // does not establish the compatible-stream decoder used by this player.
  const needsAdapter = element => !navigator.vendor.includes("Apple") && typeof MediaSource !== "undefined" || !element.canPlayType("application/vnd.apple.mpegurl");
  const positive = (value, fallback) => Number.isFinite(Number(value)) && Number(value) > 0 ? Number(value) : fallback;
  const video = (facts, contentType, compatible = false) => {
    const width = positive(facts.width, 1920), height = positive(facts.height, 1080);
    const bitrate = positive(facts.bitrate, 8000000), framerate = positive(facts.framerate, 30);
    const scale = compatible ? Math.min(1, 1920 / width, 1080 / height) : 1;
    return {contentType, width: Math.floor(width * scale), height: Math.floor(height * scale),
      bitrate: compatible ? Math.min(bitrate, 6128000) : bitrate, framerate: compatible ? Math.min(framerate, 60) : framerate};
  };
  async function supports(element, facts, [codec, contentType]) {
    const mediaSource = needsAdapter(element) || (typeof Hls !== "undefined" && Hls.isSupported());
    try {
      if (navigator.mediaCapabilities?.decodingInfo) {
        const result = await navigator.mediaCapabilities.decodingInfo({type: mediaSource ? "media-source" : "file", video: video(facts, contentType, true)});
        return result.supported && result.smooth && (codec === "h264" || result.powerEfficient);
      }
    } catch (_) {}
    return mediaSource ? typeof MediaSource !== "undefined" && MediaSource.isTypeSupported(contentType) : element.canPlayType(contentType) !== "";
  }
  const audioIncompatible = (policy, mode, required) => policy === "direct-first" && (required === true || mode === "audio-transcode");
  const appleMatroska = type => Boolean(type && navigator.vendor.includes("Apple") && /^video\/(x-)?matroska(?:;|$)/i.test(type));
  const policy = (saved, server, compatible) => {
    const fallback = {automatic: "direct-first", direct: "direct-only", compatible: "compatible"}[server] || "direct-first";
    return compatible ? ["direct-first", "direct-only", "compatible"].includes(saved) ? saved : fallback : "direct-only";
  };
  const initialCompatible = (policy, mode, type, direct, planned, audioRequired) => policy === "compatible" || !direct ||
    audioIncompatible(policy, mode, audioRequired) || !planned && policy === "direct-first" && appleMatroska(type);
  return {codecs, video, supports, needsAdapter, audioIncompatible, appleMatroska, policy, initialCompatible};
})();
