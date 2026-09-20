import { readFileSync } from 'node:fs';
import path from 'node:path';

// Follow the Go asset declarations so lint sees the same scopes as the browser.
export function browserScriptBundles(repo) {
  const declarations = new Map();
  const bundles = [];
  for (const [namespace, file] of [
    ['webassets', 'packages/webassets/webassets.go'],
    ['player', 'apps/player/internal/server/assets.go'],
    ['subtitles', 'apps/subtitles/internal/server/assets.go'],
  ]) {
    const source = readFileSync(path.join(repo, file), 'utf8');
    for (const [, asset, name] of source.matchAll(/\/\/go:embed (\S+\.(?:js|css))\s+(\w+) \[\]byte/g)) {
      declarations.set(`${namespace}.${name}`, [path.join(path.dirname(file), asset)]);
    }
    for (const [, name, expression] of source.matchAll(/^\s*(?:var )?(\w+)\s*=\s*(.+)$/gm)) {
      let dependencies;
      if (/^webassets\.\w+$/.test(expression)) dependencies = [expression];
      else if (expression.startsWith('joinScripts(')) dependencies = expression.slice(12, -1).split(', ');
      else if (expression.startsWith('append(append([]byte(nil), ')) {
        const match = expression.match(/^append\(append\(\[\]byte\(nil\), (\w+)\.\.\.\), (\w+)\.\.\.\)$/);
        if (!match) throw new Error(`Unsupported script composition: ${file}: ${expression}`);
        dependencies = match.slice(1);
      } else continue;
      const key = `${namespace}.${name}`;
      declarations.set(key, dependencies.map(value => value.includes('.') ? value : `${namespace}.${value}`));
      if (dependencies.length > 1) bundles.push(key);
    }
  }
  function resolve(key, visiting = new Set()) {
    if (/\.(?:js|css)$/.test(key)) return [key];
    if (visiting.has(key)) throw new Error(`Circular script composition: ${key}`);
    const values = declarations.get(key);
    if (!values) throw new Error(`Unknown script composition: ${key}`);
    return values.flatMap(value => resolve(value, new Set([...visiting, key])));
  }
  const referenced = new Set([...declarations.values()].flat());
  return bundles.map(name => ({ name, files: resolve(name) }))
    .filter(bundle => !referenced.has(bundle.name))
    .filter(bundle => {
      if (bundle.files.every(file => file.endsWith('.css'))) return false;
      if (bundle.files.some(file => !file.endsWith('.js'))) throw new Error(`Mixed script composition: ${bundle.name}`);
      return true;
    });
}
