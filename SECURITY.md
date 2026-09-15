# Security policy

This policy covers Player, Subtitles, Dashboard, the native Apple clients, shared packages, and repository tooling.

## Report privately

Use this repository's **Security → Advisories → Report a vulnerability** form when private vulnerability reporting is available. If it is unavailable, open a non-sensitive issue asking the maintainer to establish a private contact channel. Include no exploit details in that request.

Do not post credentials, tokens, backup archives, private URLs, library paths, media titles, or viewing activity in an issue. Redact logs and screenshots before sharing them.

A useful private report includes:

- affected app and version or full source commit;
- host OS, architecture, container engine, and client version;
- affected feature or endpoint and the minimum reproduction;
- expected and observed behavior, impact, and required access; and
- whether the issue requires an Owner, Viewer, local network, or public listener.

The maintainer coordinates validation, remediation, and disclosure privately. There is no published response-time SLA.

## Supported revisions

Security maintenance targets current `main` and the latest published app release when one exists. This monorepo has no published GitHub releases as of September 15, 2026. Report source-build findings with the exact commit; older app versions are not a long-term-support promise.

## Deployment boundaries

- Create the first Owner over a private local connection before opening network access.
- Player and Subtitles require Owner strong authentication. Their default generated local HTTPS certificate needs explicit trust; public access uses a separately restricted gateway.
- Player's Jellyfin compatibility is off by default. Enable only the integrations you need.
- Player mounts media read-only. Subtitles intentionally writes sidecars beside videos; protect originals and recovery copies separately.
- Dashboard uses localhost HTTP by default. Configure trusted HTTPS, its exact public URL, trusted hosts, and secure cookies before crossing an untrusted network. Dashboard MCP tokens grant Owner access.
- Back up application state, external configuration, secret material, and media separately. Protect backup keys independently from encrypted archives.

Read the app's setup and security guides for details. Host firewall rules, certificate trust, DNS account security, disk encryption, physical access, and off-host backup custody remain operator responsibilities.

## Verification and release evidence

Signed-release installer checks must remain enabled. A checksum establishes file consistency; it does not establish a publisher's identity.

GitHub Actions is disabled. While `.gates-disabled` exists, local quality suites are also disabled. Neither retained workflow definitions nor skipped checks establish security certification. The [Player release checklist](apps/player/engineering/release-checklist.md) records the required release evidence and current blockers.
