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

Saving public settings adds persisted fields. Pre-feature Subtitles binaries reject these fields through their strict settings validator. Roll back with a compatible binary or restore a pre-change backup through the supported stopped-server workflow. Do not edit live state to remove fields. Player and Subtitles startup does not write public defaults.
