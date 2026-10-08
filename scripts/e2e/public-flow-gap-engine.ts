import { web } from '@e2e-dev/web';
// One finite target; the public surfaceOf API receives this exact engine handle.
export const playerDeepEngine = web({ browser: 'chromium', viewport: { width: 1440, height: 900 } });
