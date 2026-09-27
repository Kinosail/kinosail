# Kinosail Subtitles documentation

Read the published [Kinosail Subtitles documentation](https://kinosail.com/subtitles/), or start with the [app README](../README.md). These are the user-guide sources; engineering notes belong in [engineering](../engineering/README.md).

## Read the guides

- [Getting started](getting-started/index.md): installation and first setup.
- [User guide](user-guide/index.md): coverage, wanted items, matching, and maintenance.
- [Owner guide](owner-guide/index.md): writable libraries, providers, language, access, and recovery.
- [Developer guide](developer-guide/index.md): versioned API and MCP boundaries.
- [Reference](reference/index.md): configuration, API, compatibility, and privacy.
- [Troubleshooting](troubleshooting/index.md) and [FAQ](faq.md).

## Preview and publication

The pages use Jekyll, local layouts/assets, Liquid links, and the navigation in `_data/navigation.yml`. They are not a Markdown-only static HTML folder. Opening a source page in GitHub shows the text, but Liquid navigation requires Jekyll rendering.

Install Ruby 3.2 or newer and Bundler, then use the shared pinned [documentation bundle](../../../engineering/documentation/README.md). From the repository root:

```sh
export BUNDLE_GEMFILE="$PWD/engineering/documentation/Gemfile"
bundle install
bundle exec jekyll serve --source apps/subtitles/docs --destination /tmp/kinosail-subtitles-docs --host 127.0.0.1 --port 4101
```

Open `http://127.0.0.1:4101`. Use a separate output directory so generated files never mix with source. The shared `Gemfile.lock` pins the renderer and parser used for both app sites.

`_config.yml` defaults to a local root with no public hostname. The shared Pages build sets `url` and `baseurl`, publishes Subtitles at `/subtitles/`, and checks both app sites before deployment. Build and inspect a preview at the actual hosting base path.

Check rendered links, asset paths, search, keyboard navigation, and narrow layouts at the actual hosting base path. Follow the [documentation guide](contributing/documentation.md).
