import { parseProgress, type Progress, type InputValue } from './contract';
import { exactObject } from './media-preferences';

const invalid = () => new Error('Saved playback progress could not be read.');
export function validateSyncProgress(
  value: unknown,
  required: boolean,
): Progress {
  const raw = exactObject(value, ['seconds', 'watched', 'session', 'revision']);
  if (Object.keys(raw).length !== 4) throw invalid();
  const progress = parseProgress(raw as InputValue);
  if (
    progress.seconds > 31_536_000 ||
    /[\u0000-\u001f\u007f]/u.test(progress.session) ||
    (required && (!progress.session || progress.revision < 1))
  )
    throw invalid();
  return progress;
}
