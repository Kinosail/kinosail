import { exactObject } from './media-preferences';
export type DownloadTrackSelection = { audio: number[]; subtitles: number[] };
export type DownloadTrackOptions = {
  audio: { index: number; label: string }[];
  subtitles: { index: number; label: string }[];
};
export function parseDownloadTracks(raw: unknown): DownloadTrackSelection {
  const value = exactObject(raw, ['audio', 'subtitles']);
  for (const [key, maximum] of [
    ['audio', 32],
    ['subtitles', 256],
  ] as const) {
    const indices = value[key];
    if (
      !Array.isArray(indices) ||
      indices.length > maximum ||
      new Set(indices).size !== indices.length ||
      indices.some(
        (index) =>
          !Number.isInteger(index) ||
          (index as number) < 0 ||
          (index as number) > 4095,
      )
    )
      throw new Error('The download track selection is invalid.');
  }
  return {
    audio: [...(value.audio as number[])],
    subtitles: [...(value.subtitles as number[])],
  };
}
export function parseDownloadTrackOptions(raw: unknown): DownloadTrackOptions {
  const value = exactObject(raw, ['audio', 'subtitles']);
  const result: DownloadTrackOptions = { audio: [], subtitles: [] };
  for (const [key, maximum] of [
    ['audio', 32],
    ['subtitles', 256],
  ] as const) {
    if (!Array.isArray(value[key]) || value[key].length > maximum)
      throw new Error('Download tracks are unavailable.');
    result[key] = value[key].map((raw) => {
      const track = exactObject(raw, ['index', 'label']);
      if (
        !Number.isInteger(track.index) ||
        (track.index as number) < 0 ||
        (track.index as number) > 4095 ||
        typeof track.label !== 'string' ||
        !track.label ||
        track.label.length > 256 ||
        /[\u0000-\u001f\u007f]/.test(track.label)
      )
        throw new Error('Download tracks are unavailable.');
      return { index: track.index as number, label: track.label };
    });
    if (
      new Set(result[key].map((track) => track.index)).size !==
      result[key].length
    )
      throw new Error('Download tracks are unavailable.');
  }
  return result;
}
