# Getting help

Start with the guide for the app you are using:

- [Player: installation, sign-in, scanning, and playback](apps/player/docs/troubleshooting/index.md)
- [Subtitles: providers, matching, permissions, and sidecars](apps/subtitles/docs/troubleshooting/index.md)
- [Apple clients: build and verification boundaries](apps/player/apps/native/README.md)

## Report a bug

Search [existing issues](https://github.com/Kinosail/kinosail/issues) before opening a report. Include the app, version or commit, host platform, installation method, browser/client version, expected behavior, observed behavior, and minimal reproduction steps.

From the app directory, `docker compose ps` and `docker compose logs --tail 100` can help identify a startup problem. Use `podman compose` if that runs your installation. Inspect and redact the output before sharing it. Never attach `.env`, secret files, account databases, backup archives, private device pairing files, or media URLs containing credentials.

For playback, include container/codec information and whether the issue occurs with direct or converted playback. For subtitles, include the provider, language, error category, and whether the media directory is writable; keep provider credentials private.

## Request a feature or correct the docs

Describe the task you want to complete, why the current behavior prevents it, and any workable alternative. For documentation errors, link the page and quote the relevant command or sentence. A documentation correction can be submitted directly as a focused pull request using [CONTRIBUTING.md](CONTRIBUTING.md).

## Security reports

Follow [SECURITY.md](SECURITY.md). Do not include vulnerability details in a normal issue. Support is maintainer-led; no paid support plan or response-time guarantee is implied by this repository.
