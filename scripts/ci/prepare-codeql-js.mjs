import { mkdirSync, readFileSync, realpathSync, statSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { Script } from 'node:vm';
import { browserScriptBundles } from '../quality/browser-script-bundles.mjs';

const manifests = ['packages/webassets/webassets.go', 'apps/player/internal/server/assets.go',
  'apps/subtitles/internal/server/assets.go'];
const fragments = ['start', 'end'].map(part => `apps/subtitles/internal/server/static/player_streaming_${part}.js`);

export function prepareCodeQLJavaScript(repo, destination) {
  const root = realpathSync(repo);
  function read(relative) {
    const file = realpathSync(path.resolve(root, relative));
    if (!file.startsWith(root + path.sep) || !statSync(file).isFile() || statSync(file).size > 1024 * 1024) {
      throw new Error('Invalid or oversized browser source');
    }
    const bytes = readFileSync(file);
    if (bytes.length > 1024 * 1024) throw new Error('Oversized browser source');
    return new TextDecoder('utf-8', { fatal: true, ignoreBOM: true }).decode(bytes);
  }
  manifests.forEach(read);
  const bundle = browserScriptBundles(root).find(item => item.name === 'subtitles.playerJS');
  if (!bundle || bundle.files.length > 32 || !fragments.every(file => bundle.files.includes(file))) {
    throw new Error('Missing or invalid Subtitles player bundle');
  }
  const source = bundle.files.map(read).join('');
  new Script(source, { filename: 'subtitles-player.js' }); // Parse only; never execute app code.
  mkdirSync(destination, { recursive: true });
  writeFileSync(path.join(destination, 'subtitles-player.js'), source);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  if (process.argv.length !== 2) throw new Error('This command accepts no arguments');
  const root = fileURLToPath(new URL('../../', import.meta.url));
  prepareCodeQLJavaScript(root, path.join(root, 'codeql-generated'));
}
