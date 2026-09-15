# Documentation renderer

Player and Subtitles use the same pinned Jekyll bundle. Install Ruby 3.2 or newer and Bundler. From the monorepo root:

```sh
export BUNDLE_GEMFILE="$PWD/engineering/documentation/Gemfile"
bundle install
bundle exec jekyll serve --source apps/player/docs --destination /tmp/kinosail-player-docs --host 127.0.0.1 --port 4100
```

Open `http://127.0.0.1:4100`. For Subtitles, change the source to `apps/subtitles/docs`, output to `/tmp/kinosail-subtitles-docs`, and port to `4101`. Stop the preview with Ctrl-C. The bundle does not change application runtimes.

Use `bundle exec jekyll build` with the same source/destination options to prepare a static artifact. To render below a hosting prefix, add `--baseurl /chosen-prefix` and supply the real `url` in deployment configuration. The app `_config.yml` files intentionally do not claim a live hosting origin.

Keep generated output and local Bundler caches outside tracked source. Update `Gemfile` and `Gemfile.lock` together when changing the renderer. GitHub Pages is not enabled by these files; uploading a public site remains a separate publication step.

## Review a documentation change

Inspect representative rendered pages and GitHub READMEs. Check links, heading order, code examples, table reflow, search, keyboard navigation, and light/dark themes. Use the actual hosting prefix for publication review. While the root `.gates-disabled` marker exists, disabled quality suites remain off; record manual evidence separately.

See [Player docs](../../apps/player/docs/README.md), [Subtitles docs](../../apps/subtitles/docs/README.md), and the [documentation preparation record](../research/documentation-production-readiness-2026-09-15.md).
