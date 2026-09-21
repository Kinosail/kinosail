import { readFileSync } from 'node:fs';
import path from 'node:path';

// Follow the Go asset declarations so lint sees the same scopes as the browser.
export function browserScriptBundles(repo) {
  const declarations = new Map();
  const bundles = [];
  const resources = new Set();
  for (const [namespace, file] of [
    ['webassets', 'packages/webassets/webassets.go'],
    ['player', 'apps/player/internal/server/assets.go'],
    ['subtitles', 'apps/subtitles/internal/server/assets.go'],
  ]) {
    const source = readFileSync(path.join(repo, file), 'utf8');
    for (const [, asset, name] of source.matchAll(/\/\/go:embed (\S+)\s+(\w+) \[\]byte/g)) {
      const resource = path.join(path.dirname(file), asset);
      resources.add(resource);
      declarations.set(`${namespace}.${name}`, [resource]);
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
    if (resources.has(key)) return [key];
    if (visiting.has(key)) throw new Error(`Circular script composition: ${key}`);
    const values = declarations.get(key);
    if (!values) throw new Error(`Unknown script composition: ${key}`);
    return values.flatMap(value => resolve(value, new Set([...visiting, key])));
  }
  const referenced = new Set([...declarations.values()].flat());
  return bundles.flatMap(name => {
    const files = resolve(name);
    if (files.every(file => !file.endsWith('.js'))) return [];
    if (files.some(file => !file.endsWith('.js'))) throw new Error(`Mixed script/resource composition: ${name}`);
    return referenced.has(name) ? [] : [{ name, files }];
  });
}
