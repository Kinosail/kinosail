# Kinosail web Player documentation

Read **[Kinosail Player Docs](https://kinosail.github.io/kinosail/docs/)**, starting with **[Install with Docker](https://kinosail.github.io/kinosail/quickstart/)**. Source builds are a secondary path for contributors and early evaluation. Other apps and native clients are outside this site's current scope.

## Read the guides

- [Docker installation](quickstart.md) and [first setup](getting-started/first-setup.md).
- [User guide](user-guide/index.md): browsing, playback, profiles, and collections.
- [Owner guide](owner-guide/index.md): libraries, access, playback, and recovery.
- [Developer guide](developer-guide/index.md): versioned API and MCP boundaries.
- [Reference](reference/index.md): configuration, API, compatibility, and privacy.
- [Troubleshooting](troubleshooting/index.md) and [FAQ](faq.md).
- [Build from source](source-install.md).

## Preview and publication

The pages use Jekyll, local layouts/assets, Liquid links, and `_data/navigation.yml`. Opening Markdown in GitHub shows the text; the complete interface requires rendering.

Use the [documentation build and preview instructions](../../../engineering/documentation/README.md). The builder bundles the local font, applies the production base path, and excludes research material. Keep generated files outside the checkout.

The Documentation workflow builds and checks pull requests. Changes merged to `main` deploy through GitHub Pages. Check rendered links, asset paths, search, keyboard navigation, and narrow layouts before publishing interface changes. Follow the [documentation contribution guide](contributing/documentation.md).
