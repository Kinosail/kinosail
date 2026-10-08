import {test as check} from 'node:test';
import assert from 'node:assert/strict';
import {registerHooks} from 'node:module';

// Import the actual composed registration graph without constructing a Page,
// running a case body, navigating, or using a provider.
const registrations = [], context = [];
const test = (title, optionsOrCallback, callback) => registrations.push({title: [...context, title].join(' > '), callback: callback ?? optionsOrCallback});
test.skip = test.use = test.beforeEach = () => {};
test.describe = (title, callback) => {context.push(title); try {callback();} finally {context.pop();}};
test.describe.configure = () => {};
globalThis.kinosailSubtitleRegistrations = {test, expect: () => {throw new Error('case body must not execute during collection');}};
const qaRoot = new URL('../../apps/subtitles/e2e/', import.meta.url).href;
const registrationShim = 'data:text/javascript,' + encodeURIComponent('export const {test,expect}=globalThis.kinosailSubtitleRegistrations;');
const axeShim = 'data:text/javascript,' + encodeURIComponent('export default class {constructor() {throw new Error("accessibility body must not execute during collection");}}');
const hooks = registerHooks({resolve(specifier, context, next) {
  if (context.parentURL?.startsWith(qaRoot) && new URL('.', context.parentURL).href === qaRoot) {
    if (specifier === '@playwright/test') return {url: registrationShim, shortCircuit: true};
    if (specifier === '@axe-core/playwright') return {url: axeShim, shortCircuit: true};
    if (specifier.startsWith('./') && !/\.[a-z]+$/i.test(specifier)) return next(specifier + '.ts', context);
  }
  if (context.parentURL === qaRoot + 'subtitle-dashboard-helpers.ts' && specifier === '../../../scripts/testing/auth-form-navigation')
    return next(specifier + '.ts', context);
  return next(specifier, context);
}});
try {await import('../../apps/subtitles/e2e/subtitle-dashboard.spec.ts');}
finally {hooks.deregister(); delete globalThis.kinosailSubtitleRegistrations;}

check('actual dashboard registers one case per unique full title', () => {
  const titles = registrations.map(row => row.title);
  const duplicates = titles.filter((title, index) => titles.indexOf(title) !== index);
  assert.deepEqual(duplicates, []);
  assert.ok(registrations.every(row => typeof row.callback === 'function'));
});
check('compact loaded, pending/failed, and enlarged-choice journeys all remain registered', () => {
  for (const title of ['Compact navigation does not cover the current subtitle task',
    'Landscape keeps the current task reachable during pending and failed navigation',
    'Enlarged settings keep native timeout choices inside the phone and landscape page']) {
    assert.equal(registrations.filter(row => row.title === title).length, 1, title);
  }
});
console.log(JSON.stringify({registrationTitles: registrations.map(row => row.title)}));
