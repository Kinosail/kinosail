# Kinosail web Player documentation

Read **[Kinosail Player Docs](https://kinosail.com/docs/)**. Start with **[Install with Docker](https://kinosail.com/quickstart/)**. Use a source build to contribute or test code that is not released yet. These docs do not cover other Kinosail apps.

## Read the guides

- [Docker installation](quickstart.md) and [first setup](getting-started/first-setup.md).
- [User guide](user-guide/index.md) for browsing, playback, profiles, and collections.
- [Owner guide](owner-guide/index.md) for libraries, access, playback, and recovery.
- [Developer guide](developer-guide/index.md) for the versioned API and MCP.
- [Reference](reference/index.md) for configuration, API, compatibility, and privacy.
- [Troubleshooting](troubleshooting/index.md) and [FAQ](faq.md).
- [Build from source](source-install.md).

## Preview and publication

The site uses Jekyll, local layouts and assets, Liquid links, and `_data/navigation.yml`. GitHub shows the source Markdown. Build the site to see its full interface.

Follow the [documentation build and preview instructions](../../../engineering/documentation/README.md). The builder includes the local font, sets the site path, and leaves out research files. Write generated files outside the checkout.

CI builds and checks each pull request. GitHub Pages publishes changes merged to `main`. Before you change the site interface, check links, assets, search, keyboard use, and small screens. Follow the [documentation contribution guide](contributing/documentation.md).
