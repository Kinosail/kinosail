# Public session timeout failure analysis (before implementation)

Base: 58556202390375fee0c28154acc6f1ed26552405. Scope: Player and Subtitles; no Nox settings or deployment.

## Existing behavior and compatibility

Private browser defaults are 15 minutes inactive and 8 hours absolute. Public issuance always persists an eight-hour ExpiresAt. Browser validation applies the private inactivity and absolute values as well. Therefore legacy public effective absolute is min(private absolute, 8 hours), while its inactivity remains the private value even when larger than that cap. Legacy public non-browser sessions have only the persisted eight-hour expiry. Preserve all these cases when public fields are absent. Do not normalize or persist new defaults on startup. Existing private endpoints and fields retain their meaning.

## Failure paths

- A guest, private Viewer, stale Owner, forged forwarding header, or request missing CSRF changes settings. Keep existing Owner/step-up/CSRF middleware and deny before persistence.
- Invalid, duplicate, unknown, non-finite, zero, negative, unlimited, out-of-range or inactivity > absolute input changes settings or sessions. Validate before locks/writes. Public UI matches private choices; absolute minimum is four hours.
- Public settings accidentally affect private sessions, or private settings overwrite explicitly configured public limits. Resolve limits by persisted channel.
- Public issuance, cookie, migration, active/device counts, or request validation disagree. Use committed ExpiresAt for cookies and one channel-aware expiry policy.
- Raising limits extends an existing idle deadline or revives an expired/unobserved session. Pin old issuance ceilings on policy changes; remove already-expired sessions before raising; never increase stored ceilings. New sessions also persist inactivitySeconds.
- Tightening logs out still-valid sessions unnecessarily. Clamp stored idle/absolute ceilings from original timestamps and remove only sessions already beyond the new limits.
- A partial write, concurrent issuance/request, or restart loses policy or permits resurrection. Lock profiles before settings, commit settings and sessions in one SQLite transaction, then publish both in-memory values. File-only fallback must persist session ceilings first and remain fail closed if settings persistence fails.
- A one-year public session is silently rejected by the fixed 31-day recent-authentication check. Use configured public absolute maximum for already strongly authenticated public sessions; preserve Owner/private step-up requirements and public route/profile restrictions.
- Revocation/profile revision, public concurrency limit, token hashing, or Secure/HttpOnly/Strict cookies regress. Retain these controls; never expose secrets in evidence.

## Verification planned before code

Real native Go Server processes plus real Chrome, disposable synthetic Owners/Viewers and virtual WebAuthn. Owner selects public/private forms; verify public/private persisted fields, invalid input without effects, Viewer/public/stale Owner/CSRF denials, restart persistence, cookie issuance, exact and before-boundary idle/absolute/ExpiresAt rejection independently per channel, tightening/raising/no resurrection, legacy defaults and customized-private cap cases. Capture responsive populated Settings renders and accessibility. Receipt records revision, diff hash, commands, binaries, fixture rules, environment and results; no TLS bypass. Browser HTTP is loopback only. No physical Safari or deployment claim.

## Rollback boundary

Public fields are added only on explicit Owner Save. An older Subtitles binary rejects these fields. A separate new SQLite document would also be rejected by old document-name allowlists, so storage remains one explicit settings schema extension.

The current binary provides Owner-only `DELETE /api/v1/settings/public-session-timeouts` and the **Use original public limits** form. The operation atomically omits only the two public settings, pins current public session expiry to the legacy eight-hour issuance cap (browser validation additionally uses private absolute), and preserves revocations and unrelated data. It never increases existing ceilings. Confirm `publicSessionTimeoutsConfigured:false`, revoke other sessions with `DELETE /api/v1/sessions`, then sign out the current Owner with `DELETE /api/v1/session`. Stop the current binary, start the older binary against unchanged data, and sign in again. Older binaries ignore per-session inactivity ceilings, so this deliberate version rollback requires fresh sign-ins. No historical backup restore or live DB edits are needed. A failed reset leaves the prior policy and sessions; do not downgrade after failure.

The browser fixture tests a real pre-feature binary on the same disposable database after reset, then upgrades again. Current data and removed sessions must remain intact. Before rollback, keep a current recovery backup according to the established Server workflow; restoring historical authentication state is not part of this procedure.
