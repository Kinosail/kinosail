# Browser session recovery, 2026-10-03

## Failure paths identified before implementation

The reported symptom was immediate passkey sign-in after closing and reopening a tab.
The diagnosis separated these paths before changing behavior:

- A missing cookie: lifetime, origin, browser profile, private browsing, or HTTPS rejection.
- A rejected cookie: idle or absolute expiry, stored expiry, revocation, missing state, or Profile changes.
- An accepted session with a sign-in page: a saved `/login` URL or Strict-cookie cross-site navigation.
- Player and Subtitles cookie collisions, including safe migration and logout behavior from PR445.

The initial failure-path record is `.verification/signin-persistence/failure-modes.md`.
PR450 separately fixes issuance-clock drift without changing session policy.

## Reproduced defect and change

Both real native apps reproduced the saved-login failure twice before the change.
After actual virtual-WebAuthn sign-in, the persistent app cookie survived tab closure.
`/api/v1/me` returned 200, but reopening `/login` started another passkey ceremony.
The page always offered a returning browser's passkey without checking its existing session.

A link from another site also reproduced the failure.
The Strict cookie was absent on the initial navigation and its redirect to `/login`.
A same-origin request could still authenticate the existing session.

The shared login script now checks `/api/v1/me` before offering a passkey.
A successful response resumes the sanitized intended destination.
Explicit `stepup=1` and `switch=1` retain sign-in.
Rejected, unavailable, or timed-out checks retain the existing sign-in flow.
The existing return-path validation rejects external destinations; `/login` returns resolve to `/`.
Script asset versions advance to 16 in both apps and the public login page.

Cookie flags, session timeouts, revocation, public expiry limits, and stored credentials are unchanged.
No production authentication or configuration writes were performed.

## Repeatable verification

Install the pinned app E2E dependencies, Go, Python, and Playwright browsers.
Run from the repository root:

```sh
python3 scripts/testing/test-session-persistence-local.py
node apps/player/e2e/node_modules/@playwright/test/cli.js test session-resume.spec.ts passkeys.spec.ts --config=apps/player/e2e/session-persistence.config.ts
node apps/subtitles/e2e/node_modules/@playwright/test/cli.js test session-resume.spec.ts passkeys.spec.ts --config=apps/subtitles/e2e/session-persistence.config.ts
```

The native runner creates both actual app binaries and disposable synthetic Owners.
It signs in through real virtual WebAuthn, keeps both apps on one hostname, and checks:

- Tab reopening at `/`, saved `/login`, and an intended `/account` destination.
- Cross-site navigation with the initial Strict cookie absent.
- Persistent browser restart and app restart using the same synthetic state.
- Acceptance before idle, absolute, and stored-expiry boundaries, followed by rejection at each boundary.

Boundary fixtures modify only stopped, disposable app state inside the runner's temporary directory.
The server's HTTP authorization is the assertion boundary.
The runner removes credentials, cookie values, browser profiles, and synthetic state afterward.
Each artifact contains a revision, commands, environment, safe results, binary hashes, and screenshots.
It contains no cookie values, passkey material, passwords, or authentication bodies.

The first portable run at `20261003T183138Z` passed all 44 recorded observations.
Its receipt records commit `ed0728dd` plus the pending production diff hash.
Both apps returned 200 without any new passkey begin on every recovery journey.
Six expired-session journeys returned 401 and retained `/login`.
Cookie metadata confirmed separate persistent Secure, HttpOnly, Strict app cookies.

The browser-only checks cover accepted, rejected, unavailable, explicit step-up/switch, and unsafe-return cases.
All 34 checks per app passed with installed Chrome and Playwright WebKit.
The retained passkey fixtures return 401 for unauthenticated session checks and declare UTF-8 explicitly.
The retained held-navigation status check observes DOM mutations before navigation can freeze the document.
Its assertion still requires the exact success text while the destination remains unavailable.
The populated Player journey classifies only the same-origin `/api/v1/me` 401 on `/login` as an expected probe.
Every other resource failure or script error remains fatal to the journey.
The populated-server `@smoke` journey also verifies a closed tab and saved login return in required CI.
Authentication trace, video, and screenshot capture are disabled for that journey.

## Evidence boundaries

The native run uses loopback HTTP and trusted browser localhost behavior, with no TLS bypass.
It proves real protocol and persistence behavior, not physical Safari or a physical passkey.
WebKit checks prove the shared script behavior using mocked authentication responses.

Nox image metadata showed revision `21ba0b8`, which includes PR445, and persistent configuration mounts.
Both app containers had zero recorded restarts at the read-only inspection.
The live database projection was denied by filesystem permissions, and no alternate read was attempted.
Actual Nox session settings remain unverified; repository defaults are not evidence of live settings.
The user later confirmed reopening `https://nox7.duckdns.org:38127` at `/`.
A credential-free system-curl request with normal certificate verification observed 303 to same-host `/login`, then 200.
This proves the deployed redirect and trusted TLS, but does not identify why a live session was rejected.
The actual hostname's accepted-session and cookie-sending evidence remain separate verification boundaries.
Deployment, exact deployed revision, trusted HTTPS, and the user's Safari journey require separate proof.

Both committed app `make verify-changed` runs passed their focused Go checks.
Each then stopped at the same 94 preexisting shared-package lint findings.
No local check was skipped or bypassed; required GitHub CI remains the publication gate.
