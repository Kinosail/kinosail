export type PlaybackCapabilities = {
  videoCodecs: string[];
  audioCodecs: string[];
  hdrFormats: string[];
  maxAudioChannels: number;
};

// A native module is a trust boundary too. Do not put unchecked bridge output
// into a URL, or cache display/audio-route capabilities across route changes.
export function playbackCapabilitiesQuery(input: unknown): string {
  const invalid = () => new Error('Invalid device playback capabilities.');
  if (!input || typeof input !== 'object' || Array.isArray(input))
    throw invalid();
  const value = input as Record<string, unknown>;
  if (
    Object.keys(value).some(
      (key) =>
        ![
          'videoCodecs',
          'audioCodecs',
          'hdrFormats',
          'maxAudioChannels',
        ].includes(key),
    )
  )
    throw invalid();
  const list = (name: string, allowed: string[]) => {
    const entries = value[name];
    if (
      !Array.isArray(entries) ||
      !entries.length ||
      entries.length > allowed.length ||
      entries.some(
        (entry) => typeof entry !== 'string' || !allowed.includes(entry),
      ) ||
      new Set(entries).size !== entries.length
    )
      throw invalid();
    return entries.join(',');
  };
  const video = list('videoCodecs', ['h264', 'hevc', 'av1', 'vp9']);
  const audio = list('audioCodecs', [
    'aac',
    'mp3',
    'opus',
    'vorbis',
    'ac3',
    'eac3',
  ]);
  const hdr = list('hdrFormats', ['sdr', 'hdr10', 'hlg']);
  if (
    !(value.hdrFormats as string[]).includes('sdr') ||
    !Number.isSafeInteger(value.maxAudioChannels) ||
    (value.maxAudioChannels as number) < 1 ||
    (value.maxAudioChannels as number) > 8
  )
    throw invalid();
  return `videoCodecs=${video}&audioCodecs=${audio}&hdrFormats=${hdr}&maxAudioChannels=${value.maxAudioChannels}`;
}
