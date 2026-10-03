# Browser session cookie issuance failure analysis

The affected HTTP surface is browser sign-in, including local passkeys, public
passkeys, and approved public Quick Connect grants.

Failure modes recorded before the production change:

- Session creation persists `ExpiresAt` using the initial clock reading.
- Cookie issuance reads the clock again after persistence. A second boundary can
  make the cookie expire later than the stored session.
- Legacy migration correctly uses the stored expiry. The retained real-handler
  migration test requires exact equality with the original cookie and can fail.
- Public issuance used the system clock instead of the configured session clock.
- Idle expiry, absolute expiry, the public eight-hour cap, cookie security flags,
  app isolation, logout and revocation must remain unchanged.

The fix emits the committed session expiry and calculates only the remaining
`Max-Age`. It does not change a session timeout, renew authentication, or relax
legacy migration assertions.

A deterministic HTTP issuance regression was written before the fix. Its
synthetic clock advances by one second during persistence. All three modes
(local, public and public grant) failed on base revision `fe85fbbb` and passed
with the fix. Repeat with:

```sh
go -C packages test ./identitycore -run '^TestCookieIssuancePreservesPersistedExpiryAcrossSecondBoundary$' -count=1
```

The retained Player real-handler migration, app-cookie isolation and passkey
cookie isolation tests passed 20 repetitions each. This is source/HTTP-handler
proof, not physical Safari or production deployment proof. Immediate tab
reopening is a separate investigation; this clock mismatch is not established
as the user's recurring sign-in cause.
