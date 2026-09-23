# Kinosail documentation site

The web Player documentation site is published to <https://kinosail.com/> by `.github/workflows/ci.yml`. Pull requests build and check the artifact; only `main` can deploy to the `github-pages` environment.

## Edit the source

- User guides: `apps/player/docs/`.
- Layout, search, and assets: `apps/player/docs/_layouts/` and `apps/player/docs/assets/`.
- Scope: web Player only, Docker first; other apps and native clients are not promoted here.

The build copies only documentation inputs into a temporary staging directory. It excludes research folders, preserves guide URLs, bundles the local font, and builds full-text search. The app documentation remains usable in its standalone renderer. Nothing changes application binaries or deployment.

## Build and preview

Install Ruby 3.2 or newer, Bundler, Python 3.9 or newer, and Node.js 26. From the repository root:

```sh
export BUNDLE_GEMFILE="$PWD/engineering/documentation/Gemfile"
bundle install
npm ci --prefix engineering/documentation --ignore-scripts
npm test --prefix engineering/documentation
python3 -m unittest discover -s engineering/documentation -p 'test_*.py'
python3 engineering/documentation/build.py --output /tmp/kinosail-preview-root
python3 engineering/documentation/check.py /tmp/kinosail-preview-root
python3 -m http.server 4180 --bind 127.0.0.1 --directory /tmp/kinosail-preview-root
```

Open <http://127.0.0.1:4180/>. Stop with Ctrl-C. The output directory must be new and outside the checkout; choose a new output name for each build rather than deleting unrelated files. `--baseurl` and `--url` allow another HTTPS origin and path. Pass the same base path as the second argument to `check.py`.

Keep generated output and local Bundler caches outside tracked source. Update `Gemfile` and `Gemfile.lock` together when changing dependencies.

## Verify and publish

The checker verifies every local link, fragment, asset, page heading, and the aggregate search index. Build input tests reject malformed origins, path prefixes, and unsafe output destinations before side effects. Inspect desktop and mobile layouts, light/dark themes, keyboard navigation, copy buttons, search results/empty/error states, and narrow tables before publishing interface changes.

GitHub Pages must use **GitHub Actions** as its build source. No custom domain is required. Deployment uses a static artifact with only Pages and OIDC write permissions; pull-request jobs have read-only repository access and cannot deploy. A failed build or link check blocks publication.

After the protected pull request merges, verify the CI documentation job and Pages deployment, public HTTPS home page, a nested guide, and search JSON. Source merge and live publication are separate facts.
