import type { MediaItem, PlaybackSource } from './contract';
import type { PlaybackPreferences } from './media-preferences';
import { nativeContentType } from './video-source';

// Local decoding still reads the original bytes. Never send a Server rendition
// through the original-file transport or infer permission to convert video.
export function preferLocalPlayback(
  item: MediaItem,
  source: PlaybackSource,
  preferences: PlaybackPreferences,
  available: boolean,
): boolean {
  if (
    !available ||
    source.plan.mode !== 'direct' ||
    nativeContentType(source.contentType) !== 'progressive'
  )
    return false;
  if (
    item.kind !== 'video' ||
    preferences.nightMode ||
    preferences.dialogueBoost ||
    preferences.volumeBoost > 1
  )
    return true;
  const media = source.details?.media;
  const mime = source.contentType.split(';', 1)[0].trim().toLowerCase();
  const mimeContainers: Record<string, string> = {
    'video/mp4': 'mp4',
    'video/x-matroska': 'mkv',
    'video/x-msvideo': 'avi',
    'video/x-flv': 'flv',
    'video/ogg': 'ogg',
  };
  const container = (
    media?.container ||
    mimeContainers[mime] ||
    item.container ||
    ''
  ).toLowerCase();
  // AVPlayer does not demux these containers. Unknown formats retain the
  // platform path; a pessimistic hint must not authorize a conversion.
  return (
    ['mkv', 'matroska', 'avi', 'flv', 'ogg', 'ogv'].includes(container) ||
    (media?.codec === 'h264' &&
      (media.bitDepth > 8 || /high (10|4:2:2|4:4:4)/i.test(media.profile)))
  );
}

// Expo exposes a message, not a stable cross-platform native error code. Only
// explicit decoder failures justify automatic local recovery. Unknown and
// network errors retain a user-controlled retry; messages are never retained.
export function playbackFailure(error: unknown) {
  const unknown = {
    message: 'Playback could not continue on this device.',
    local: true,
    decode: false,
  };
  if (
    !error ||
    typeof error !== 'object' ||
    !('message' in error) ||
    typeof error.message !== 'string' ||
    error.message.length > 4096
  )
    return unknown;
  const message = error.message.toLowerCase();
  if (/authoriz|forbidden|permission denied|\b401\b|\b403\b/.test(message))
    return {
      message:
        'Your Server denied access to this title. Check your Viewer permissions or reconnect.',
      local: false,
      decode: false,
    };
  if (/\b404\b|not found|no such file/.test(message))
    return {
      message:
        'This file is no longer available. Return to details and check your library.',
      local: false,
      decode: false,
    };
  if (
    /network|http|timed?\s*out|connection|certificate|cancel|offline/.test(
      message,
    )
  )
    return {
      message:
        'Playback was interrupted. Check your connection to the Server and try again.',
      local: false,
      decode: false,
    };
  const decode =
    /error_code_decod(?:er_init|ing)_failed|decoderinitializationexception|mediacodec(?:video|audio)renderer|cannot decode|failed to decode|unsupported (?:media|container|codec)|decoder (?:initialization|initialisation) failed/.test(
      message,
    );
  return { ...unknown, decode };
}
