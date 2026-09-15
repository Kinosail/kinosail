import { createReadStream, readFileSync, statSync } from 'node:fs';
import { createServer } from 'node:http';
import { extname, join, normalize, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const appRoot = resolve(fileURLToPath(new URL('..', import.meta.url)));
const repoRoot = resolve(appRoot, '../..');
const dist = join(appRoot, 'dist');
const port = Number(process.env.KINOSAIL_NATIVE_QA_PORT || 4173);
const media = join(
  repoRoot,
  '.kinosail-test-ui-check3/media/Movies/Example Movie.mp4',
);
const backdrop = join(repoRoot, 'internal/server/static/cinema-backdrop.jpg');
let revision = 2;

const baseProgress = {
  seconds: 0,
  watched: false,
  session: '',
  revision: 0,
};

const entries = [
  ['arrival', 'Arrival', '2016', 'Science Fiction · Drama', 1820],
  ['moon', 'Moon', '2009', 'Science Fiction', 0],
  ['bear', 'The Bear', '2024', 'Drama · Comedy', 960],
  ['dune', 'Dune: Part Two', '2024', 'Science Fiction · Adventure', 0],
  ['past-lives', 'Past Lives', '2023', 'Drama · Romance', 0],
  ['godzilla', 'Godzilla Minus One', '2023', 'Action · Drama', 0],
  ['blue-eye', 'Blue Eye Samurai', '2023', 'Animation · Drama', 0],
  ['holdovers', 'The Holdovers', '2023', 'Comedy · Drama', 0],
  [
    'everything',
    'Everything Everywhere All at Once',
    '2022',
    'Science Fiction · Comedy',
    0,
  ],
];

const items = entries.map(([id, title, year, genres, seconds], index) => ({
  id,
  kind: 'video',
  title,
  year,
  genres,
  plot:
    index === 0
      ? 'A linguist works to understand visitors from another world.'
      : `${title} is ready to play from your local Kinosail library.`,
  rating: index % 2 ? 'TV-14' : 'PG-13',
  tagline: index === 0 ? 'Why are they here?' : '',
  director: index === 0 ? 'Denis Villeneuve' : '',
  artwork: id === 'moon' ? '' : `/qa/poster/${id}.svg`,
  backdrop: '/qa/backdrop.jpg',
  container: 'mp4',
  progress: { ...baseProgress, seconds, revision: seconds ? 2 : 0 },
}));

const types = {
  '.css': 'text/css; charset=utf-8',
  '.html': 'text/html; charset=utf-8',
  '.ico': 'image/x-icon',
  '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.map': 'application/json; charset=utf-8',
  '.png': 'image/png',
};

const send = (
  response,
  status,
  body,
  type = 'application/json; charset=utf-8',
) => {
  const data = type.startsWith('application/json')
    ? JSON.stringify(body)
    : body;
  response.writeHead(status, {
    'Cache-Control': 'no-store',
    'Content-Length': Buffer.byteLength(data),
    'Content-Type': type,
    'X-Content-Type-Options': 'nosniff',
  });
  response.end(data);
};

const authorize = (request, response) => {
  if (request.headers.authorization === 'Bearer qa-viewer-token') return true;
  send(response, 401, { error: 'session required' });
  return false;
};

const poster = (id) => {
  const item = items.find((entry) => entry.id === id);
  const title = (item?.title || 'Kinosail').replace(/[&<>"']/g, '');
  const hue =
    Math.abs([...id].reduce((sum, value) => sum + value.charCodeAt(0), 0)) % 80;
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 900"><rect width="600" height="900" fill="hsl(${hue + 90} 18% 13%)"/><circle cx="430" cy="220" r="210" fill="none" stroke="#c8f169" stroke-opacity=".3" stroke-width="2"/><path d="M80 690h440" stroke="#c8f169" stroke-width="8"/><text x="80" y="755" fill="#f6f8ef" font-family="Arial" font-size="48" font-weight="700">${title}</text><text x="80" y="810" fill="#9ca391" font-family="Arial" font-size="20" letter-spacing="5">KINOSAIL LIBRARY</text></svg>`;
};

const streamFile = (request, response, file, type) => {
  const size = statSync(file).size;
  const range = request.headers.range;
  if (range) {
    const match = /^bytes=(\d+)-(\d*)$/.exec(range);
    if (!match) return send(response, 416, 'Invalid range', 'text/plain');
    const start = Number(match[1]);
    const end = match[2] ? Math.min(Number(match[2]), size - 1) : size - 1;
    response.writeHead(206, {
      'Accept-Ranges': 'bytes',
      'Content-Length': end - start + 1,
      'Content-Range': `bytes ${start}-${end}/${size}`,
      'Content-Type': type,
    });
    createReadStream(file, { start, end }).pipe(response);
    return;
  }
  response.writeHead(200, {
    'Accept-Ranges': 'bytes',
    'Content-Length': size,
    'Content-Type': type,
  });
  createReadStream(file).pipe(response);
};

const api = (request, response, url) => {
  if (request.method === 'POST' && url.pathname === '/api/v1/quick-connect') {
    return send(response, 201, { code: '381204', secret: 'qa-secret' });
  }
  if (
    request.method === 'POST' &&
    url.pathname === '/api/v1/quick-connect/token'
  ) {
    return send(response, 201, { token: 'qa-viewer-token' });
  }
  if (!authorize(request, response)) return;
  if (request.method === 'GET' && url.pathname === '/api/v1/me') {
    return send(response, 200, {
      serverId: 'qa-server',
      server: 'Den Server',
      viewer: { id: 'viewer-1', name: 'Mike', owner: false },
    });
  }
  if (request.method === 'GET' && url.pathname === '/api/v1/library') {
    const history = url.searchParams.get('view') === 'history';
    return send(response, 200, {
      items: history
        ? items.filter((item) => item.progress.seconds > 0)
        : items,
      total: history ? 2 : items.length,
      offset: 0,
      limit: 60,
      letters: [],
    });
  }
  const itemMatch = /^\/api\/v1\/items\/([^/]+)$/.exec(url.pathname);
  if (request.method === 'GET' && itemMatch) {
    const item = items.find((entry) => entry.id === itemMatch[1]);
    return item
      ? send(response, 200, { item, listed: false })
      : send(response, 404, { error: 'not found' });
  }
  const playback = /^\/api\/v1\/items\/([^/]+)\/playback$/.exec(url.pathname);
  if (request.method === 'GET' && playback) {
    return send(response, 200, {
      plan: { allowed: true, mode: 'direct', reason: 'compatible' },
      direct: `/media/${playback[1]}`,
      directType: 'video/mp4',
      duration: 8,
      start: 0,
    });
  }
  const progress = /^\/api\/v1\/items\/([^/]+)\/progress$/.exec(url.pathname);
  if (request.method === 'PUT' && progress) {
    revision += 1;
    return send(response, 200, { ...baseProgress, revision });
  }
  send(response, 404, { error: 'not found' });
};

createServer((request, response) => {
  const url = new URL(request.url || '/', `http://${request.headers.host}`);
  if (url.pathname.startsWith('/api/')) return api(request, response, url);
  if (url.pathname.startsWith('/media/')) {
    if (!authorize(request, response)) return;
    return streamFile(request, response, media, 'video/mp4');
  }
  if (url.pathname === '/qa/backdrop.jpg') {
    return streamFile(request, response, backdrop, 'image/jpeg');
  }
  const posterMatch = /^\/qa\/poster\/([^/]+)\.svg$/.exec(url.pathname);
  if (posterMatch) {
    return send(response, 200, poster(posterMatch[1]), 'image/svg+xml');
  }
  const relative = normalize(decodeURIComponent(url.pathname)).replace(
    /^\/+/,
    '',
  );
  const candidate = resolve(dist, relative || 'index.html');
  try {
    const safe = candidate.startsWith(`${dist}/`);
    const file =
      safe && statSync(candidate).isFile()
        ? candidate
        : join(dist, 'index.html');
    return send(
      response,
      200,
      readFileSync(file),
      types[extname(file)] || 'application/octet-stream',
    );
  } catch {
    return send(
      response,
      200,
      readFileSync(join(dist, 'index.html')),
      types['.html'],
    );
  }
}).listen(port, '127.0.0.1', () => {
  process.stdout.write(`Kinosail native QA: http://127.0.0.1:${port}\n`);
});
