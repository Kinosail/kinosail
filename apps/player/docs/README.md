# Kinosail Player documentation

Start with the [app README](../README.md) for requirements and a local source installation. These are the user-guide sources; engineering notes belong in [engineering](../engineering/README.md).

## Read the guides

- [Getting started](getting-started/index.md): installation and first setup.
- [User guide](user-guide/index.md): browsing, playback, profiles, and collections.
- [Owner guide](owner-guide/index.md): libraries, access, playback, integrations, and recovery.
- [Developer guide](developer-guide/index.md): versioned API and MCP boundaries.
- [Reference](reference/index.md): configuration, API, compatibility, and privacy.
- [Troubleshooting](troubleshooting/index.md) and [FAQ](faq.md).

## Preview and publication

The pages use Jekyll, local layouts/assets, Liquid links, and the navigation in `_data/navigation.yml`. They are not a Markdown-only static HTML folder. Opening a source page in GitHub shows the text, but Liquid navigation requires Jekyll rendering.

Install Ruby 3.2 or newer and Bundler, then use the shared pinned [documentation bundle](../../../engineering/documentation/README.md). From the repository root:

```sh
export BUNDLE_GEMFILE="$PWD/engineering/documentation/Gemfile"
bundle install
bundle exec jekyll serve --source apps/player/docs --destination /tmp/kinosail-player-docs --host 127.0.0.1 --port 4100
```

Open `http://127.0.0.1:4100`. Use a separate output directory so generated files never mix with source. The shared `Gemfile.lock` pins the renderer and parser used for both app sites.

`_config.yml` defaults to a local root with no public hostname. Before hosting, explicitly set `url` and `baseurl` for the chosen origin and path. Build and inspect both app sites with distinct output locations. GitHub Actions is disabled, and this documentation change does not enable Pages or publish a website. Review publication permissions before exposing private repository content.

When quality gates are enabled, check rendered links, asset paths, search, keyboard navigation, and narrow layouts at the actual hosting base path. While `.gates-disabled` exists, disabled suites remain off; do not claim their checks passed. Follow the [documentation guide](contributing/documentation.md).
