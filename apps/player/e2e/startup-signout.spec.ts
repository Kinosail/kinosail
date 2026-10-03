import {expect, test} from '@playwright/test';
import {createServer} from 'node:http';
import {readFile, writeFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';

for (const scenario of ['pending', 'active', 'failed', 'cancelled']) {
  test(`startup speculation stops before ${scenario} sign-out @smoke`, async ({page}, info) => {
    const script = await readFile(new URL('../internal/server/static/startup-preparation.js', import.meta.url));
    const journal: {method: string, route: string, authenticated: boolean, status: number}[] = [];
    let authenticated = true;
    let failNext = scenario === 'failed';
    const html = (title = 'Library') => `<!doctype html><title>${title}</title><meta name="kinosail-csrf" content="fixture-csrf"><h1>${title}</h1>
      <div class="home-feature"><a data-feature-action href="/watch/0123456789abcdef">Play</a></div>
      <form method="post" action="/logout"><button>Sign out</button></form>
      <script>window.kinosailPlaybackCapabilities={policy:()=>"direct-only",initialCompatible:()=>false};</script><script src="/startup.js"></script>`;
    const server = createServer(async (request, response) => {
      const route = new URL(request.url!, 'http://localhost').pathname;
      response.setHeader('Cache-Control', 'no-store');
      if (route === '/startup.js') { response.setHeader('Content-Type', 'text/javascript'); response.end(script); return; }
      if (route === '/logout') {
        const failed = failNext;
        failNext = false;
        if (!failed) authenticated = false;
        journal.push({method: request.method!, route, authenticated, status: failed ? 503 : 303});
        await new Promise(resolve => setTimeout(resolve, 850));
        response.writeHead(failed ? 503 : 303, failed ? {'Content-Type': 'text/html'} : {Location: '/login'});
        response.end(failed ? html('Sign out unavailable') : ''); return;
      }
      if (route.startsWith('/api/')) {
        const status = authenticated ? (request.method === 'POST' ? 202 : 200) : 401;
        journal.push({method: request.method!, route: route.replace('/0123456789abcdef/', '/{id}/'), authenticated, status});
        response.writeHead(status, {'Content-Type': 'application/json'});
        response.end(JSON.stringify(route.endsWith('/playback') ? {plan: {mode: 'direct', allowed: true}, direct: '/media/0123456789abcdef', start: 0, duration: 60} : {state: 'ready'})); return;
      }
      response.writeHead(200, {'Content-Type': 'text/html'});
      response.end(route === '/login' ? '<!doctype html><h1>Sign in</h1>' : html());
    });
    await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
    const address = server.address() as {port: number};
    const url = `http://127.0.0.1:${address.port}`;
    let result = 'failed';
    try {
      await page.goto(url);
      if (scenario === 'active') await expect.poll(() => journal.filter(row => row.route.endsWith('/playback-prepare') && row.method === 'POST').length).toBe(1);
      if (scenario === 'cancelled') await page.evaluate(() => document.querySelector('form')!.addEventListener('submit', event => event.preventDefault(), {once: true}));
      await page.getByRole('button', {name: 'Sign out'}).click();
      if (scenario === 'failed') {
        await expect(page.getByRole('heading', {name: 'Sign out unavailable'})).toBeVisible();
        await expect.poll(() => journal.filter(row => row.route.endsWith('/playback-prepare') && row.method === 'POST').length).toBe(1);
        await page.getByRole('button', {name: 'Sign out'}).click();
      } else if (scenario === 'cancelled') {
        await page.waitForTimeout(900);
        expect(journal.some(row => row.route === '/logout')).toBe(false);
        await page.reload();
        await expect.poll(() => journal.filter(row => row.route.endsWith('/playback-prepare') && row.method === 'POST').length).toBe(1);
        await page.getByRole('button', {name: 'Sign out'}).click();
      }
      await expect(page.getByRole('heading', {name: 'Sign in'})).toBeVisible();
      expect(journal.filter(row => row.route.startsWith('/api/') && !row.authenticated)).toEqual([]);
      result = 'passed';
    } finally {
      await writeFile(info.outputPath('startup-signout-receipt.json'), JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION || process.env.GITHUB_SHA, command: 'playwright test startup-signout.spec.ts', scenario, result, scriptSHA256: createHash('sha256').update(script).digest('hex'), journal, boundary: 'Actual startup script and HTTP form navigation; synthetic authentication state. Populated CI covers real server authentication.'}, null, 2));
      server.closeAllConnections();
      await new Promise<void>(resolve => server.close(() => resolve()));
    }
  });
}
