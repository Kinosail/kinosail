import {expect, test} from '@playwright/test';
import {createServer} from 'node:http';
import {readFile, writeFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';

for (const scenario of ['pending', 'probing', 'negotiating', 'empty-target', 'self-target', 'superseded', 'restored', 'ready']) {
  test(`startup speculation stops during ${scenario} watch navigation @smoke`, async ({page}, info) => {
    const script = await readFile(new URL('../internal/server/static/startup-preparation.js', import.meta.url));
    const journal: {method: string, path: string, afterNavigation: boolean}[] = [];
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.message));
    let navigationStarted = false;
    let observed = false, controlAfterNavigation = false;
    page.on('console', message => {if (message.text() === 'qa-startup-control-released') {observed = true; controlAfterNavigation = navigationStarted;}});
    let releaseNavigation = () => {};
    const heldNavigation = new Promise<void>(resolve => {releaseNavigation = resolve;});
    const server = createServer(async (request, response) => {
      const path = new URL(request.url!, 'http://localhost').pathname;
      if (path === '/startup.js') {response.setHeader('Content-Type', 'text/javascript'); response.end(script); return;}
      if (path === '/favicon.ico') {response.writeHead(204); response.end(); return;}
      if (path === '/watch/0123456789abcdef') {
        navigationStarted = true;
        await heldNavigation;
        response.setHeader('Content-Type', 'text/html'); response.end('<!doctype html><h1>Watch</h1>'); return;
      }
      if (path.startsWith('/api/') || path === '/static/hls.min.js') {
        journal.push({method: request.method!, path, afterNavigation: navigationStarted});
        response.setHeader('Content-Type', path.endsWith('.js') ? 'text/javascript' : 'application/json');
        response.end(path.endsWith('/playback') ? JSON.stringify({
          plan: {mode: 'transcode', allowed: true}, compatiblePlan: {mode: 'transcode'},
          compatible: '/hls/0123456789abcdef/p/t-a0-s0-none-t0-b0/index.m3u8', direct: '/media/0123456789abcdef',
          directType: 'video/x-matroska', start: 0, duration: 60,
        }) : path.endsWith('.js') ? '/* HTTP fixture */' : '{}'); return;
      }
      response.setHeader('Content-Type', 'text/html');
      response.end(`<!doctype html><h1>Library</h1><meta name="kinosail-csrf" content="fixture-csrf">
        <div class="home-feature"><a data-feature-action href="/watch/0123456789abcdef">Play</a></div>
        <script>window.kinosailPlaybackCapabilities={policy:()=>"compatible",initialCompatible:()=>true,needsAdapter:()=>true,
          codecs:[[${JSON.stringify(scenario === 'negotiating' ? 'hevc' : 'h264')}]],supports:()=>new Promise(resolve=>{window.releaseCodec=()=>resolve(true);})};</script><script src="/startup.js"></script>`);
    });
    await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
    const address = server.address();
    if (!address || typeof address === 'string') throw new Error('HTTP control did not bind');
    let result = 'failed';
    let clicked: Promise<void> | undefined;
    try {
      await page.goto(`http://127.0.0.1:${address.port}`);
      if (scenario === 'empty-target' || scenario === 'self-target') await page.evaluate(empty => {
        const base = document.createElement('base'); base.target = empty ? '_self' : '_blank'; document.head.append(base);
        document.querySelector('a')!.target = empty ? '' : '_SELF';
      }, scenario === 'empty-target');
      if (scenario !== 'pending') await page.waitForFunction(() => typeof Reflect.get(window, 'releaseCodec') === 'function');
      if (scenario === 'ready') {
        await page.evaluate(() => Reflect.get(window, 'releaseCodec')());
        await expect.poll(() => journal.filter(row => row.method === 'POST').length).toBe(1);
      }
      // Signal through the console: WebKit blocks even fixture fetches once
      // native navigation starts. Node checks the actual server accepted it.
      await page.evaluate(restored => {
        window.addEventListener('click', () => {setTimeout(() => {void (async () => {
          if (restored) {
            // Model a persisted return after stopping the actual held departure.
            // This exercises departing=true; it is not native BFcache proof.
            window.stop();
            window.dispatchEvent(new PageTransitionEvent('pagehide', {persisted: true}));
            window.dispatchEvent(new PageTransitionEvent('pageshow', {persisted: true}));
            await new Promise(resolve => setTimeout(resolve, 750));
          }
          Reflect.get(window, 'releaseCodec')?.();
          document.querySelector('a')!.dispatchEvent(new Event('focusin', {bubbles: true}));
          console.info('qa-startup-control-released');
        })();}, 100);}, {once: true});
      }, scenario === 'restored');
      if (scenario === 'superseded') await page.evaluate(() => {
        const link = document.querySelector('a')!;
        link.addEventListener('click', () => {
          window.addEventListener('click', event => event.preventDefault(), {once: true});
          link.dispatchEvent(new MouseEvent('click', {bubbles: true, cancelable: true}));
        }, {once: true});
      });
      clicked = page.getByRole('link', {name: 'Play', exact: true}).click({noWaitAfter: scenario === 'restored'});
      await expect.poll(() => observed).toBe(true);
      expect(navigationStarted).toBe(true);
      expect(controlAfterNavigation).toBe(true);
      // Observe one complete 600ms scheduling window after releasing the probe.
      await new Promise(resolve => setTimeout(resolve, 900));
      if (scenario === 'restored') {
        await clicked;
        await expect.poll(() => journal.filter(row => row.afterNavigation && row.method === 'POST').length).toBe(1);
        await expect(page.getByRole('heading', {name: 'Library', exact: true})).toBeVisible();
        expect(errors).toEqual([]);
        result = 'passed';
        return;
      }
      expect(journal.filter(row => row.afterNavigation)).toEqual([]);
      expect(journal.filter(row => row.method === 'DELETE')).toHaveLength(0);
      releaseNavigation();
      await clicked;
      await expect(page.getByRole('heading', {name: 'Watch', exact: true})).toBeVisible();
      if (scenario === 'ready') await expect.poll(() => journal.filter(row => row.method === 'DELETE').length).toBe(1);
      if (scenario === 'ready') {
        await page.goBack();
        await expect(page.getByRole('heading', {name: 'Library', exact: true})).toBeVisible();
        await page.waitForFunction(() => typeof Reflect.get(window, 'releaseCodec') === 'function');
        await page.evaluate(() => Reflect.get(window, 'releaseCodec')());
        await expect.poll(() => journal.filter(row => row.method === 'POST').length).toBe(2);
      }
      expect(errors).toEqual([]);
      result = 'passed';
    } finally {
      releaseNavigation();
      await writeFile(info.outputPath('startup-navigation-receipt.json'), JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION || process.env.GITHUB_SHA,
        command: 'playwright test startup-navigation.spec.ts', scenario, result, navigationStarted, controlAfterNavigation, observationMs: 900, scriptSHA256: createHash('sha256').update(script).digest('hex'), journal, errors,
        boundary: 'Actual production startup script and delayed native HTTP navigation; synthetic capability and persisted-return events, no native BFcache, decoder or real authentication proof.'}, null, 2));
      server.closeAllConnections();
      await new Promise<void>(resolve => server.close(() => resolve()));
      await clicked?.catch(() => {});
    }
  });
}

for (const control of ['prevented', 'prevented-late', 'persisted', 'missing', 'base-origin', 'base-credentials', 'foreign', 'query', 'hash', 'malformed', 'oversized', 'credentials', 'download', 'target', 'base-target', 'ctrl', 'meta', 'shift', 'alt', 'middle']) {
  test(`startup remains usable after ${control} watch click @smoke`, async ({page}, info) => {
    const script = await readFile(new URL('../internal/server/static/startup-preparation.js', import.meta.url));
    const journal: {method: string, path: string}[] = [];
    const server = createServer((request, response) => {
      const path = new URL(request.url!, 'http://localhost').pathname;
      if (path === '/startup.js') {response.setHeader('Content-Type', 'text/javascript'); response.end(script); return;}
      if (path.startsWith('/api/')) {
        journal.push({method: request.method!, path});
        response.setHeader('Content-Type', 'application/json');
        response.end(JSON.stringify(path.endsWith('/playback') ? {plan: {mode: 'transcode', allowed: true}, compatiblePlan: {mode: 'transcode'},
          compatible: '/hls/0123456789abcdef/p/t-a0-s0-none-t0-b0/index.m3u8', directType: 'video/x-matroska', start: 0, duration: 60} : {})); return;
      }
      response.setHeader('Content-Type', 'text/html');
      response.end(`<!doctype html><h1>Library</h1><div class="home-feature"><a data-feature-action href="/watch/0123456789abcdef">Play</a></div>
        <script>window.kinosailPlaybackCapabilities={policy:()=>"compatible",initialCompatible:()=>true,needsAdapter:()=>false,
          codecs:[["h264"]],supports:()=>new Promise(resolve=>{window.releaseCodec=()=>resolve(true);})};</script><script src="/startup.js"></script>`);
    });
    await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
    const address = server.address();
    if (!address || typeof address === 'string') throw new Error('HTTP control did not bind');
    let result = 'failed';
    try {
      await page.goto(`http://127.0.0.1:${address.port}`);
      await page.waitForFunction(() => typeof Reflect.get(window, 'releaseCodec') === 'function');
      await page.evaluate(control => {
        const link = document.querySelector('a')!;
        if (control === 'persisted') {
          window.dispatchEvent(new PageTransitionEvent('pagehide', {persisted: true}));
          window.dispatchEvent(new PageTransitionEvent('pageshow', {persisted: true}));
          Reflect.get(window, 'releaseCodec')();
          return;
        }
        if (control === 'missing') link.removeAttribute('href');
        if (control === 'base-origin' || control === 'base-credentials') {
          const base = document.createElement('base');
          base.href = control === 'base-origin' ? 'https://untrusted.invalid/' : location.origin.replace('://', '://user:pass@') + '/';
          document.head.append(base);
        }
        const paths: Record<string, string> = {foreign: 'https://untrusted.invalid/watch/0123456789abcdef', query: '/watch/0123456789abcdef?extra=1',
          hash: '/watch/0123456789abcdef#extra', malformed: '/watch/no-item', oversized: '/watch/' + 'a'.repeat(4096), credentials: 'https://user:pass@untrusted.invalid/watch/0123456789abcdef'};
        if (paths[control]) link.href = paths[control];
        if (control === 'download') link.setAttribute('download', '');
        if (control === 'target') link.target = '_blank';
        if (control === 'base-target') {const base = document.createElement('base'); base.target = '_blank'; document.head.append(base);}
        const event = new MouseEvent('click', {bubbles: true, cancelable: true, button: control === 'middle' ? 1 : 0,
          ctrlKey: control === 'ctrl', metaKey: control === 'meta', shiftKey: control === 'shift', altKey: control === 'alt'});
        if (control === 'prevented') event.preventDefault();
        // Stop the fixture's default navigation after the production listener.
        // Other-tab, download and malformed inputs must leave this job running.
        window.addEventListener('click', event => event.preventDefault(), {once: true});
        link.dispatchEvent(event);
        if (control === 'base-origin' || control === 'base-credentials') document.querySelector('base')!.remove();
        Reflect.get(window, 'releaseCodec')();
      }, control);
      if (control === 'prevented-late' || control === 'persisted') {
        // Allow a cancelled ordinary click to restart the existing 600ms scheduler.
        await page.waitForTimeout(750);
        await page.evaluate(() => Reflect.get(window, 'releaseCodec')());
      }
      await expect.poll(() => journal.filter(row => row.method === 'POST').length).toBe(1);
      expect(journal.filter(row => row.method === 'DELETE')).toEqual([]);
      if (control !== 'prevented-late' && control !== 'persisted') expect(journal.filter(row => row.path.endsWith('/playback'))).toHaveLength(1);
      result = 'passed';
    } finally {
      await writeFile(info.outputPath('startup-click-control.json'), JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION || process.env.GITHUB_SHA,
        command: 'playwright test startup-navigation.spec.ts', control, result, scriptSHA256: createHash('sha256').update(script).digest('hex'), journal,
        boundary: 'Production startup script, actual HTTP, synthetic capability, click and persisted lifecycle events; no external URL is followed. Persisted event control is not a real BFcache claim.'}, null, 2));
      server.closeAllConnections();
      await new Promise<void>(resolve => server.close(() => resolve()));
    }
  });
}
