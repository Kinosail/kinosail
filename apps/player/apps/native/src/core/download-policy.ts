export const downloadQuota = 20 * 1024 ** 3;
// Zero is an explicit unlimited storage preference; byte counts stay exact.
export const maxDownloadBytes = Number.MAX_SAFE_INTEGER;
export const maxDownloadLimitGiB = Math.floor(maxDownloadBytes / 1024 ** 3);
export const downloadReserve = 512 * 1024 ** 2;
export function downloadStorageLimits(
  total: number | undefined,
  selected: number,
): number[] {
  const capacity =
    typeof total === 'number' && Number.isSafeInteger(total) && total >= 0
      ? Math.max(0, Math.floor((total - downloadReserve) / 1024 ** 3))
      : 20;
  const choices = [
    1, 2, 5, 10, 20, 50, 100, 250, 500, 1000, 2000, 4000, 8000,
  ].filter((value) => value <= capacity);
  if (
    Number.isInteger(selected) &&
    selected > 0 &&
    selected <= maxDownloadLimitGiB &&
    !choices.includes(selected)
  )
    choices.push(selected);
  return [...choices.sort((a, b) => a - b), 0];
}
export const downloadLimit = 1000;
export function downloadSize(
  value: string | null,
  used: number,
  free: number,
  quota: number = downloadQuota,
): number {
  if (!value || !/^[1-9][0-9]{0,15}$/.test(value))
    throw new Error('The Server did not provide a valid download size.');
  const size = Number(value);
  if (
    !Number.isSafeInteger(size) ||
    !Number.isSafeInteger(used) ||
    used < 0 ||
    !Number.isSafeInteger(free) ||
    free < 0 ||
    !Number.isSafeInteger(quota) ||
    (quota !== 0 && quota < 1024 ** 3) ||
    quota > maxDownloadBytes ||
    size > maxDownloadBytes - used ||
    (quota !== 0 && size > quota - used) ||
    size > free - downloadReserve
  )
    throw new Error(
      'Not enough download space. Remove a downloaded title and try again.',
    );
  return size;
}
export function validateDownloadResponse(
  status: number,
  length: string | null,
  range: string | null,
  offset: number,
  total: number,
) {
  const expected = total - offset;
  if (
    !Number.isSafeInteger(total) ||
    total <= 0 ||
    !Number.isSafeInteger(offset) ||
    offset < 0 ||
    offset >= total ||
    status !== (offset ? 206 : 200) ||
    length !== String(expected) ||
    (offset === 0 && range !== null) ||
    (offset > 0 && range !== `bytes ${offset}-${total - 1}/${total}`)
  )
    throw new Error(
      'The download changed. Remove it and download the title again.',
    );
}

export function validateDownloadID(id: string): string {
  if (
    typeof id !== 'string' ||
    !id ||
    id.length > 2048 ||
    /[\u0000-\u001f\u007f\ud800-\udfff]/u.test(id)
  )
    throw new Error('The downloaded title is invalid.');
  return id;
}
export function downloadModification(value: string | null): string {
  if (
    !value ||
    value.length > 64 ||
    !Number.isFinite(Date.parse(value)) ||
    new Date(value).toUTCString() !== value
  )
    throw new Error('The Server did not provide a resumable file revision.');
  return value;
}
