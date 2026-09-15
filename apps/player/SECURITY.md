# Security policy

Security fixes are provided for the latest published Kinosail Server release. Upgrade to the newest patch release before reporting a problem that may already be fixed.

Do not open a public issue containing exploit details, credentials, tokens, private media information, or other sensitive data. Use the repository's **Security > Advisories > Report a vulnerability** form to report privately. If private vulnerability reporting is not available, open a non-sensitive issue asking the maintainer to establish a private contact channel and include no vulnerability details.

Include the Kinosail version, host platform and architecture, container engine, affected endpoint or feature, impact, minimal reproduction, and whether the issue requires an authenticated Owner or Viewer. Remove credentials, tokens, Library paths, media titles, and viewing activity from logs or screenshots.

The maintainer will acknowledge a complete report, coordinate validation and remediation privately, and publish a security advisory when users need to act. Please allow a fix and release to be prepared before public disclosure.

## Secure defaults

- Jellyfin compatibility remains enabled by default. One Owner is sufficient, and every real Owner must enroll one passkey or TOTP authenticator. TOTP-protected Jellyfin profiles use Quick Connect.
- Passwords use self-describing Argon2id hashes; login work is timing-equalized and throttled by source and account. Browser sessions have 15-minute idle and 8-hour absolute limits, API/device sessions expire after 30 days, and API keys are scoped, hashed, usage-tracked, and expire after 30 days.
- Sensitive Owner writes require recent human authentication. Browser cookies are non-persistent, `HttpOnly`, and `SameSite=Strict`; browser writes require same-origin evidence. Exact public-host validation, restrictive browser headers, bounded request bodies, and narrow Jellyfin-only query tokens are enforced centrally.
- Outbound integrations reject link-local, multicast, unspecified, and metadata-service destinations after DNS resolution and do not follow redirects by default. Saved secrets are accepted from bounded regular files, automatic backups fail closed without encryption, and the activity journal is HMAC-chained to detect tampering.
- The release container is non-root and read-only with no capabilities, `no-new-privileges`, resource limits, bounded `noexec` temporary filesystems, and rotated logs. Base images and CI actions are digest-pinned; release CI performs CodeQL, dependency/vulnerability, secret, and image scans and publishes SBOM, provenance, and keyless signatures. The installer verifies the image signature and launches the resolved digest.

Host firewall/VLAN policy, disk encryption, Secure Boot, physical access, trusted certificate installation, DNS account controls, and off-host backup custody remain operator responsibilities. See the [release checklist](engineering/release-checklist.md).
