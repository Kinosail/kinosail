import {createServer, type ServerResponse} from 'node:http';
import {execFileSync} from 'node:child_process';
import {mkdtemp, readFile, readdir, rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createHash} from 'node:crypto';
import {playerSource, readStaticSource} from './static-sources';

// Failure matrix before implementation: HLS retries outlive committed departure;
// an unacknowledged checkpoint, cancelled submit, or PiP must keep its owner alive.
// Real H264/transport/Hls.js isolates lifecycle; it does not exercise Go storage.
const adapter = await readStaticSource(['../internal/server/static/hls.min.js']);

export async function hlsNavigationPeer() {
  const directory = await mkdtemp(join(tmpdir(), 'kinosail-hls-navigation-'));
  try {
    execFileSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-f', 'lavfi',
      '-i', 'testsrc2=size=160x90:rate=8', '-t', '16', '-an', '-c:v', 'libx264',
      '-preset', 'ultrafast', '-pix_fmt', 'yuv420p', '-g', '16', '-keyint_min', '16',
      '-sc_threshold', '0', '-bf', '0', '-f', 'hls', '-hls_time', '2',
      '-hls_playlist_type', 'vod', '-hls_segment_filename', join(directory, 'segment%02d.ts'),
      join(directory, 'media.m3u8')], {timeout: 20_000, stdio: 'pipe'});
  } catch (error) {
    await rm(directory, {recursive: true, force: true});
    throw error;
  }
  const media = new Map(await Promise.all((await readdir(directory)).map(async name =>
    [name, await readFile(join(directory, name))] as const)));
  const facts = {
    ffmpeg: execFileSync('ffmpeg', ['-version'], {encoding: 'utf8', timeout: 5000}).split('\n')[0].slice(0, 256),
    codec: 'libx264/yuv420p/160x90/8fps/16s/no-audio/closed-GOP16/TS',
    playerSourceSHA256: createHash('sha256').update(playerSource).digest('hex'),
    hlsAdapterSHA256: createHash('sha256').update(adapter).digest('hex'),
    mediaSHA256: Object.fromEntries([...media].map(([name, data]) => [name, createHash('sha256').update(data).digest('hex')])),
  };
  const waitingSegments = new Set<ServerResponse>();
  const waitingDestinations = new Set<ServerResponse>();
  const waitingProgress = new Set<ServerResponse>();
  const requests: Array<{kind: string; afterDestination: boolean}> = [];
  let departing = false, retry = false, holdDestination = true, holdProgress = false;
  const web = createServer((request, response) => {
    const path = request.url?.split('?')[0];
    if (path === '/watch/movie') {
      response.writeHead(200, {'Content-Type': 'text/html'});
      response.end(`<body data-viewer-profile="hls-navigation-profile">
        <a href="/">Library</a><form action="/watched/movie" method="post"><button>Mark watched</button></form>
        <div class="media-stage"><video controls muted playsinline data-hls="/hls/index.m3u8"
          data-compatibility-mode="remux" data-playback-policy="compatible" data-progress="/progress/movie"
          data-playback-session="hls-navigation-session" data-duration="16" data-start="0"></video></div>
        <label data-quality-control>Quality<select data-quality></select><small data-quality-state></small></label>
        <div data-progress-notice hidden><span data-progress-status></span><button data-progress-retry>Retry saving position</button>
          <button data-progress-continue hidden>Continue without saving</button></div>
        <script src="/static/hls.min.js"></script><script src="/static/player.js"></script></body>`);
      return;
    }
    if (path === '/static/player.js' || path === '/static/hls.min.js') {
      response.writeHead(200, {'Content-Type': 'text/javascript'});
      response.end(path.endsWith('hls.min.js') ? adapter : playerSource);
      return;
    }
    if (path === '/progress/movie') {
      requests.push({kind: 'progress', afterDestination: departing});
      if (holdProgress) waitingProgress.add(response);
      else {response.writeHead(204); response.end();}
      return;
    }
    if (path === '/' || path === '/watched/movie') {
      departing = true;
      requests.push({kind: 'destination', afterDestination: true});
      if (holdDestination) waitingDestinations.add(response);
      else {response.writeHead(200, {'Content-Type': 'text/html'}); response.end('<h1>Destination</h1>');}
      return;
    }
    if (path === '/hls/index.m3u8') {
      response.writeHead(200, {'Content-Type': 'application/vnd.apple.mpegurl'});
      response.end('#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=300000,RESOLUTION=160x90,CODECS="avc1.42c00a"\nmedia.m3u8\n');
      return;
    }
    const name = path?.startsWith('/hls/') ? path.slice(5) : '';
    const body = media.get(name);
    if (!body) {response.writeHead(404); response.end(); return;}
    requests.push({kind: name, afterDestination: departing});
    if (name === 'segment02.ts') {
      if (retry) {response.writeHead(503); response.end();}
      else {
        waitingSegments.add(response);
        response.on('close', () => waitingSegments.delete(response));
      }
      return;
    }
    response.writeHead(200, {'Content-Type': name.endsWith('.m3u8') ? 'application/vnd.apple.mpegurl' : 'video/mp2t'});
    response.end(body);
  });
  await new Promise<void>(resolve => web.listen(0, '127.0.0.1', resolve));
  const address = web.address();
  if (!address || typeof address === 'string') throw new Error('missing isolated HLS address');
  return {
    origin: `http://127.0.0.1:${address.port}`,
    facts,
    snapshot: () => ({requests: [...requests], waitingSegments: waitingSegments.size,
      waitingProgress: waitingProgress.size, departing}),
    holdCheckpoint() {holdProgress = true;},
    releaseCheckpoint() {
      holdProgress = false;
      for (const response of waitingProgress) {response.writeHead(204); response.end();}
      waitingProgress.clear();
    },
    releaseRetry() {
      retry = true;
      for (const response of waitingSegments) {response.writeHead(503); response.end();}
      waitingSegments.clear();
    },
    releaseDestination() {
      holdDestination = false;
      for (const response of waitingDestinations) {
        response.writeHead(200, {'Content-Type': 'text/html'}); response.end('<h1>Destination</h1>');
      }
      waitingDestinations.clear();
    },
    discardDestination() {
      for (const response of waitingDestinations) {response.writeHead(204); response.end();}
      waitingDestinations.clear();
    },
    async close() {
      for (const response of [...waitingSegments, ...waitingDestinations, ...waitingProgress]) response.destroy();
      web.closeAllConnections();
      await new Promise<void>((resolve, reject) => web.close(error => error ? reject(error) : resolve()));
      await rm(directory, {recursive: true, force: true});
    },
  };
}
