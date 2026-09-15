import type { CastSession, CastController } from './casting';
export const googleCastAvailable = false;
export function GoogleCastPicker(_props: { onConnected?: () => void }) {
  return null;
}
export async function connectGoogleCast(
  _session: CastSession,
): Promise<CastController> {
  throw new Error('Google Cast requires the iOS or Android app.');
}
export async function connectedGoogleTV(): Promise<string> {
  throw new Error('Google Cast requires the iOS or Android app.');
}
