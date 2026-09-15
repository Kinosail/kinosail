export type DeviceApproval = {
  code: string;
  device: string;
  expiresAt: string;
};

export function approvalCode(value: unknown): string {
  if (typeof value !== 'string' || !/^[0-9]{6}$/.test(value))
    throw new Error('Enter the six-digit code shown on your TV.');
  return value;
}

export function parseDeviceApproval(value: unknown): DeviceApproval {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    throw new Error('The device request is invalid.');
  const entry = value as Record<string, unknown>;
  if (
    Object.keys(entry).length !== 3 ||
    Object.keys(entry).some(
      (key) => !['code', 'device', 'expiresAt'].includes(key),
    ) ||
    typeof entry.device !== 'string' ||
    entry.device.length > 80 ||
    /[\u0000-\u001f\u007f-\u009f]/.test(entry.device) ||
    typeof entry.expiresAt !== 'string' ||
    !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,9})?Z$/.test(
      entry.expiresAt,
    ) ||
    !Number.isFinite(Date.parse(entry.expiresAt)) ||
    new Date(entry.expiresAt).toISOString().slice(0, 19) !==
      entry.expiresAt.slice(0, 19)
  )
    throw new Error('The device request is invalid.');
  return {
    code: approvalCode(entry.code),
    device: entry.device || 'Kinosail device',
    expiresAt: entry.expiresAt,
  };
}

export function parsePendingTVs(value: unknown): DeviceApproval[] {
  if (!Array.isArray(value) || value.length > 8)
    throw new Error('The device requests are invalid.');
  const result = value.map(parseDeviceApproval);
  if (new Set(result.map((entry) => entry.code)).size !== result.length)
    throw new Error('The device requests are invalid.');
  return result;
}
