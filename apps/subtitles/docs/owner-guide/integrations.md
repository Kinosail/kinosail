---
title: Connect subtitle providers
description: Create provider credentials and configure the Server.
section: Own the Server
last_reviewed: 2026-09-15
---

# Connect subtitle providers

Configure at least one provider for downloads. Keep credentials in Owner Settings or protected deployment secrets, never in an issue or committed `.env`. Provider quotas, account availability, and terms can change; consult the provider's current pages.

| Provider | Account and instructions | Values used by Kinosail |
| --- | --- | --- |
| SubDL | [Account panel](https://subdl.com/panel), [API documentation](https://subdl.com/api-doc) | API key |
| OpenSubtitles.com | [Sign in](https://dl.opensubtitles.com/en/users/sign_in), [getting started](https://opensubtitles.tawk.help/article/getting-started) | API application key, account username, password |
| SubSource | [Account](https://subsource.net/), [API documentation](https://subsource.net/api-docs) | API key and explicit personal-household-use acceptance |

## Save credentials

1. Create your own account at the provider.
2. Obtain its API key. For OpenSubtitles.com, create an API consumer in your account and use the account username/password along with that application key.
3. Follow **Enter key in Kinosail** from the setup page, or open the provider section in Settings.
4. Save the credentials and restart if requested.
5. Fetch one wanted subtitle. A configured key does not guarantee that a match exists or that the account has quota remaining.

Kinosail checks the configured providers and ranks candidates against the video, release, language, and subtitle role. Existing local and embedded text options are considered first.

## Deployment-managed credentials

The native process accepts `KINOSAIL_SUBDL_API_KEY`, the `KINOSAIL_OPENSUBTITLES_*` credential variables, and `KINOSAIL_SUBSOURCE_API_KEY` with `KINOSAIL_SUBSOURCE_PERSONAL_USE=true`. Supported secrets also accept `_FILE`; mount the referenced regular file into the container.

A Compose `.env` file only supplies interpolation values. The supplied Compose files forward SubDL and OpenSubtitles settings, but do not currently forward SubSource settings. Use Owner Settings for SubSource or add explicit environment/secret mappings in a local Compose override. Review the checked-in `.env.example` and `compose.yaml` together.

SubSource downloads stay unchanged: Kinosail validates them but does not rewrite, synchronize, shift, or convert the stored sidecar. An API key alone does not replace the required use acceptance.

See [privacy]({{ '/reference/architecture-and-privacy/' | relative_url }}) for data sent to providers and [troubleshooting]({{ '/troubleshooting/' | relative_url }}) for failures.
