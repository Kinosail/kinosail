import type { Page } from '@playwright/test';

type CameraFixture = {project: string; origin: string};
const invalid = () => new Error('invalid owned Camera profile');
function ownedOrigin(value: unknown, project: string) {
  if (typeof value !== 'string' || value.length > 2048) throw invalid();
  const matched = value.match(/^(http|https):\/\/(localhost|127\.0\.0\.1):([1-9][0-9]{0,4})$/);
  if (!matched || Number(matched[3]) > 65535 || (matched[1] === 'https') !== (project === 'webkit')) throw invalid();
  return new URL(value).origin;
}

export function cameraProfile(environment: Record<string, string | undefined>): CameraFixture | undefined {
  if (environment.KINOSAIL_CAMERA_PROFILE === undefined) return undefined;
  const project = environment.KINOSAIL_BROWSER_PROJECT;
  if (environment.KINOSAIL_CAMERA_PROFILE !== '1' || environment.KINOSAIL_TEST_INSTANCE !== '1'
      || environment.KINOSAIL_BROWSER_TEST !== '1' || !project
      || !['chromium', 'firefox', 'webkit'].includes(project)) throw invalid();
  ownedOrigin(environment.KINOSAIL_E2E_URL, project);
  return {project, origin: environment.KINOSAIL_E2E_URL!};
}

export async function isolateCamera(page: Page, fixture: CameraFixture | undefined, project: string, baseURL: string | undefined) {
  if (!fixture || project !== fixture.project) throw invalid();
  const origin = ownedOrigin(fixture.origin, project);
  if (ownedOrigin(baseURL, project) !== origin) throw invalid();
  await page.route('**/*', route => {
    const request = route.request();
    let url;
    try {url = new URL(request.url());} catch {return route.abort('blockedbyclient');}
    const method = request.method();
    const allowed = method === 'GET' || method === 'HEAD' || method === 'POST' && url.pathname === '/login';
    return url.origin === origin && allowed ? route.fallback() : route.abort('blockedbyclient');
  });
}
