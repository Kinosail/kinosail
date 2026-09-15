import { parseMediaItem, type InputValue } from './contract';

const invalid = () =>
  new Error('Kinosail Server returned an invalid collection.');
export function collectionName(value: unknown): string {
  if (
    typeof value !== 'string' ||
    !value.trim() ||
    value.length > 64 ||
    value === '.' ||
    value === '..' ||
    /[/\\\u0000-\u001f\u007f\ud800-\udfff]/u.test(value) ||
    encodeURIComponent(value).replace(/%[0-9A-F]{2}/g, 'x').length > 64
  )
    throw invalid();
  return value;
}
function object(value: InputValue) {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    throw invalid();
  return value;
}
export function parseCollections(value: InputValue): string[] {
  const names = object(value).collections;
  if (!Array.isArray(names) || names.length > 10000) throw invalid();
  const result = names.map(collectionName);
  if (new Set(result).size !== result.length) throw invalid();
  return result;
}
export function parseCollection(value: InputValue, name: string) {
  const body = object(value);
  if (
    body.name !== name ||
    !Array.isArray(body.items) ||
    body.items.length > 10000
  )
    throw invalid();
  const items = body.items.map(parseMediaItem);
  if (new Set(items.map((item) => item.id)).size !== items.length)
    throw invalid();
  return items;
}
