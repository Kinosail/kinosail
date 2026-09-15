# Kinosail Subtitles security

The [monorepo security policy](../../SECURITY.md) covers reporting, supported revisions, and deployment boundaries. Release-bundle readers can use the [online security policy](https://github.com/MikeO7/kinosail/blob/main/SECURITY.md).

Report vulnerabilities privately through the repository's **Security → Advisories → Report a vulnerability** form when available. Otherwise, open a non-sensitive issue asking for a private contact channel, without exploit details. Include the app, version or commit, platform, impact, reproduction, and required access. Never post credentials, tokens, backup archives, private addresses, or personal media information.

Create and secure the first Owner over a private connection before exposing network access. Protect external configuration, credentials, backup keys, and media separately from application-state backups. Consult the [README](README.md) and [release checklist](engineering/release-checklist.md) before deployment.

GitHub Actions is disabled. While `.gates-disabled` exists, quality checks are disabled too. Workflow definitions, development images, and skipped checks are not release or security certification.
