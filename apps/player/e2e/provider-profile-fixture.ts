import { test, type BrowserContext, type Page, type Route } from '@playwright/test';

type ProviderFixture = {project: string; origin: string};
const invalid = () => new Error('invalid owned Provider profile');
function ownedOrigin(value: unknown, project: string) {
  if (typeof value !== 'string' || value.length > 2048) throw invalid();
  const matched = value.match(/^(http|https):\/\/(localhost|127\.0\.0\.1):([1-9][0-9]{0,4})$/);
  if (!matched || Number(matched[3]) > 65535 || (matched[1] === 'https') !== (project === 'webkit')) throw invalid();
  return new URL(value).origin;
}

export function providerProfile(environment: Record<string, string | undefined>): ProviderFixture | undefined {
  if (environment.KINOSAIL_PROVIDER_PROFILE === undefined) return undefined;
  const project = environment.KINOSAIL_BROWSER_PROJECT;
  if (environment.KINOSAIL_PROVIDER_PROFILE !== '1' || environment.KINOSAIL_TEST_INSTANCE !== '1'
      || environment.KINOSAIL_BROWSER_TEST !== '1' || !project
      || !['chromium', 'firefox', 'webkit'].includes(project)) throw invalid();
  return {project, origin: ownedOrigin(environment.KINOSAIL_E2E_URL, project)};
}
const fixture = providerProfile(process.env);
const writes = new Set([
  'POST /login', 'POST /settings/home-assistant', 'POST /settings/home-assistant/pair', 'POST /onboarding/home-assistant',
  'POST /onboarding/jellyfin', 'POST /onboarding/trusted-https', 'POST /api/v1/home-assistant/pairings',
  'POST /supporter/display', 'PUT /api/v1/supporter/display', 'PUT /api/v1/settings/home-assistant',
  'PUT /api/v1/settings/jellyfin', 'PUT /api/v1/settings/trusted-https',
  'POST /api/v1/settings/trusted-https/validate', 'POST /api/v1/settings/trusted-https/test',
  'DELETE /api/v1/settings/trusted-https',
]);
function admitted(route: Route) {
  const request = route.request(), raw = request.url(), method = request.method();
  if (typeof raw !== 'string' || raw.length > 8192 || /[\s\x00-\x1f]/.test(raw)) return false;
  try {
    const url = new URL(raw);
    return url.origin === fixture!.origin && !url.username && !url.password && !url.hash
      && (method === 'GET' || method === 'HEAD' || writes.has(`${method} ${url.pathname}`));
  } catch {return false;}
}

export async function isolateProvider(context: BrowserContext, project: string, baseURL: string | undefined) {
  if (!fixture || project !== fixture.project || ownedOrigin(baseURL, project) !== fixture.origin) throw invalid();
  await context.route('**/*', route => admitted(route) ? route.fallback() : route.abort('blockedbyclient'));
}

export function configureProviderProfile() {
  if (!fixture) return;
  test.use({serviceWorkers: 'block'});
  test.beforeEach(async ({context, baseURL}, info) => isolateProvider(context, info.project.name, baseURL));
}

export async function providerRoute(page: Page, matcher: Parameters<Page['route']>[0], handler: Parameters<Page['route']>[1]) {
  // Page routes precede context routes; admit before any synthetic callback can fetch or continue.
  await page.route(matcher, fixture ? (route, request) => admitted(route) ? handler(route, request) : route.abort('blockedbyclient') : handler);
}

export async function providerResponse(route: Route) {
  if (!fixture) return route.fetch();
  if (!admitted(route)) throw invalid();
  const response = await route.fetch({maxRedirects: 0});
  if (response.status() !== 200 || response.url() !== route.request().url()) throw invalid();
  return response;
}
