# Security audit hardening ledger

This ledger closes every candidate listed under “Findings not raised” in the Kinosail Security Auditor review for `kinosail-audit-target.Uz9b4t`. A candidate is marked closed either by a code hardening change with regression coverage or by an existing, directly verified repository control that already prevents the proposed path.

| Candidate | Resolution and evidence |
|---|---|
| Subtitle sidecar TOCTOU | `readUpgradeSidecar` and backup state reads now validate the opened inode with `os.SameFile` and use bounded reads. Replacement-after-check regression coverage is in `apps/subtitles/internal/server/subtitle_v1_coverage_internal_test.go`. |
| First-owner setup race | Bootstrap remains loopback-by-default, setup-before-LAN is documented, and `auth.Manager.Setup` serializes the first successful owner creation. The concurrent setup regression remains in `apps/dashboard/internal/auth/coverage_setup_race_test.go`. |
| Plaintext non-loopback browser session theft | Non-loopback Dashboard public URLs must use HTTPS; HTTPS origins force Secure session cookies even when an embedding caller builds `server.Config` directly. Coverage is in `apps/dashboard/cmd/kinosail-dashboard/main_test.go` and `apps/dashboard/internal/server/auth_http_test.go`. |
| Dashboard probe SSRF | Existing Owner/MCP authorization, normalized HTTP(S) targets, DNS/IP validation and pinning, disabled redirects/proxies, special-use blocking, and public-host allowlisting remain covered by `apps/dashboard/internal/dashboard` tests. |
| Public gateway reaching private Player routes | Existing fixed-socket routing, provenance-header stripping, public-host validation, and Player public-route deny-by-default policy remain covered by `packages/publicgateway`, `packages/remoteaccess`, and Player authorization tests. |
| Jellyfin `{user}` cross-profile access | User-scoped browse, latest, item, and played-item handlers now require the path user to match the authenticated viewer. Cross-profile 404 and same-profile success coverage is in `apps/player/internal/server/jellyfin_extended_test.go`. |
| OIDC/SAML issuer or account-link confusion | Existing issuer/audience/signature/nonce/state/PKCE checks, signed SAML correlation, authenticated local-profile linking, and exact SCIM identity matching remain covered by `packages/federation` tests. |
| MCP prompt injection | MCP server instructions and `read_api` explicitly classify returned titles, descriptions, text, and other values as untrusted data; server-side scope and route checks remain authoritative. Coverage is in `packages/mcpgateway/gateway_flow_helpers_test.go`. |
| Backup traversal or unauthenticated restore | Backup state reads now revalidate the opened inode and bound bytes; the existing fixed archive-entry allowlist, manifest validation, key requirement, and atomic private persistence remain covered by `packages/backup` tests. |
| `x/crypto/openpgp` advisory | `golang.org/x/crypto` is updated to `v0.57.0` in all four Go modules, and repository source remains free of `openpgp` references. |
| `KINOSAIL_LINT_BASE` Actions injection | Base revision resolution now uses `git rev-parse --verify --end-of-options` and passes the canonical revision as a quoted value. |
| Raw `template.HTML` XSS | Existing embedded-artwork and fixed-icon allowlists remain the only raw-HTML sources; request/provider strings continue through `html/template` escaping and the existing template/security tests. |
| Installer TLS bypass | The only retained `curl --insecure` use is a literal loopback setup-status probe. Release artifacts remain digest-selected and Cosign-verified; installer tests cover the setup probe path. |
| Generic API-key scanner matches | Matches are deterministic MFA test fixtures, with no corroborated production credential; `.gitleaks.toml` now scopes an explicit allowlist to those two test files. |
| `AGENTS.md`/`.codex` instruction poisoning | `SECURITY.md` now records that repository instructions are not authorization, deployment configuration, or secrets, and privileged automation must not execute untrusted pull-request instructions. |

## Verification boundary

Focused hardening tests and the affected shared packages were run after these changes. The repository’s `.gates-disabled` marker remains in place; disabled wrapper suites were not represented as passing. The full Player server package still contains three unrelated pre-existing contract failures (management-page fixture status, MCP management-route classification before this change, and API-parity copy); the MCP classification was repaired and its dedicated test passes. The full Subtitles server package still contains three unrelated provider-ranking boundary tests whose expected ordering/scores disagree with current behavior. Those baseline failures are separate from the audit candidates and were not represented as passing.
