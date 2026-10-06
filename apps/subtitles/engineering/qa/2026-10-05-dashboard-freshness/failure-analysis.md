# R16: dashboard freshness before implementation

R16 / SUB-05 remains runtime-unverified. This package prepares public controls and
failure-first browser contracts against main
e85544ebce14aab0454ea11d1fb01415dfbedd4f, tree
f312d738144f356a088ccd10f14d51657ade92a9. Source identities are in
source-inventory.json. No production file, existing proof, or runtime was changed.

The approved backlog is 704949 bytes, SHA256
02bc9d14f7debc0cd5df8f1be32fad894fc65e4c0e0789c2be00b0394f85d503.
The audit describes automatic ready-to-ready upgrades. The first proposed
reproduction instead uses an actual manual ready-to-ready Save as a smaller
public ledger/History/source/recovery witness. It does not prove automatic
provider upgrades or exercise their 24-hour eligibility and 15-minute automation
throttle. Those remain separate acceptance boundaries.

## Three distinct source boundaries

1. subtitle-status.js polls only five coverage totals. Its History branch returns
   without scheduling a poll. It has no live-event subscription. If all five
   totals remain equal, this code does not reveal its existing Refresh notice.
2. subtitle_dashboard.html places #subtitle-update only outside the History
   branch. Subscribing to ledger events alone cannot offer a History Refresh
   element that the current template does not render.
3. Provider-health observe/reset persists factual changes but has no event
   publisher or revision token. A ledger-only subscriber does not establish
   provider-only freshness.

The Server already publishes authenticated subtitles.updated after durable
History changes, with resource /api/v1/subtitle-library. The shared event envelope
has only id, type, resource, and at. Its bounded history is 128 events, queue 16,
subscriber cap 256, heartbeat 15 seconds, and access recheck 1 second.
Last-Event-ID supports retained replay; it has no gap/resync signal. Numeric ID
gaps are not proof of lost subtitle events because other event types/profiles
share the sequence. Current-session revocation must be tested through public
auth, not inferred from a mocked access predicate.

## Disposable public fixture eligibility

Use the exported server.New(Config), an independently owned TLS loopback target,
and a retained fresh owned filesystem root. RequireAuth is true. Enroll a real
Owner through setup, account-page CSRF, TOTP MFA, then current-session CSRF.
The initial cookieless setup response is not required to contain CSRF. Reject
redirects, foreign authorities/paths, conflicting catalogue registration, and
unregistered item destinations. Discover the actual item ID from the real bounded
catalogue response. Never use private ledger/event/auth setters or fake Save
responses. All request and response-body deadlines derive from owned contexts.

Config.FFprobe stays empty. application_init preserves it; mediaprobe.inspect
returns empty facts before process launch for an empty executable. Cached
subtitle facts treat that configuration as checked. The ledger/replay fixture supplies an
owned fictional .mp4 catalogue file and valid English sidecar A; it claims no
decoded-media proof. Preview/apply explicitly uses automaticSync:false and no
draft, audio, encoder, or generation route. FFmpeg/FPCalc point to an owned
unavailable sentinel and must never execute.

With provider configuration absent, witness summary total=1, ready=1, wanted=0,
pending=0, unavailable=0 before and after actual inspect/preview/apply B. Use a
second authenticated control request while the first dashboard is open: its own
runAction would reload the page and conceal external-change freshness.
Require real B bytes, recoverable A bytes, one new matching manual updated
History entry, changed source/installed/recovery projection, and all five equal
totals. subtitleDocument emits two trailing newlines for each cue; expected B/C
bytes follow that actual canonical serialization. Initial recovery retains exact
A bytes, including A's original single trailing newline. A 200 response alone is insufficient; apply can return 500 after effects.
Manual Save marks Frozen=true and cannot be relabeled an automatic upgrade.

Provider-only and bad-CSRF fixtures use a separate retained empty media root:
all five summary totals are zero. automate() returns before provider work when
the real catalogue has no items. This avoids forced timing or private state while
keeping the provider initially untested. The public provider test sends actual
JSON {}. Provider-only eligibility uses a separately owned loopback SubDL
stand-in at its actual /api/v2/me route. Existing provider/test performs that real request. Require
actual successful provider health/quota changes, unchanged sidecar/History and
equal five totals. No paid or external provider is contacted. Source absence of a
publisher is recorded separately; no existing no-event behavior is made a
permanent contract. Any new provider event/revision producer needs its own
failure-first public contract before implementation.

## Failure classification and UI invariants

First establish the real Owner, catalogue, summary, Save/effect/History and SSE
controls. Missing auth, invalid preview, changed counts, absent durable effects,
malformed payloads, unjoined work or missing registration are prerequisites,
never product RED. Only an eligible case with completed observation and settled
owned work can identify a missing Refresh affordance as intended RED.

Library, History and provider Summary each need phone and desktop cases.
The Go control's single-item or empty catalogue is not a browser scroll fixture.
Require a real nonzero scroll range and position before crediting scroll
preservation. A later Library fixture can use at least 41 owned ready items;
History can use 20 actual setup Save events. Both require actual public setup
within the unchanged allowance, not seeded private History or forced DOM.
Provider Summary's actual layout must independently establish its eligibility.
These larger browser fixtures are proposed and unimplemented; if setup or layout
is ineligible, the case is prerequisite-blocked and no scroll proof is credited.
Notice arrival must retain row DOM identity/content, expanded rows, scroll and
focused control: no automatic replacement, navigation or focus move. Then
exercise the real Refresh action and await rendered current factual data while
retaining supported query/page and open-row/scroll state. Distinguish notice
focus invariance from focus restoration after an explicit user page replacement;
the latter is not established by the current public contract.

Coalesce bursts into one affordance. Observe real navigation/back/pagehide and
same-document replacement to detect duplicate subscribers. Test retained replay,
revocation and failures separately. Do not infer unbounded reconnect correctness
from one retained event or manufacture cursor-gap meaning. A history-overflow
fallback is a later separately bounded authoritative contract.

The proposed first notice allowance is 75 seconds: existing 60-second ready
polling plus its 15-second full-response bound. Effects eligibility has a
10-second absolute window inside it, never afterward. No fake clock accelerates
production. Proposed fresh-case cap is 120 seconds, including 20 setup + 75 notice
+ 15 explicit Refresh + 5 cleanup + 5 headroom; pair 250, outer 255. Root must
approve actual hosted admission/driver budgets before runtime. These are proposed
test bounds, not a measured product SLO or an implemented behavior.

Every assertion records attempted, completed and nullable passed. Preserve fixed
case/assertion identities, exact known-error multiset, zero retries and unknown
error rejection. Export only allowlisted booleans/counts/timings/source hashes.
Never export cookie/TOTP/CSRF/provider key, request bodies, runtime media filenames/URLs, raw
errors, config or unredacted event data. Retain owned roots and evidence; cancel
and join streams/handlers/readers before checked socket/root close. No cleanup
deletion is authorized.

## Reviewed constructor-isolation correction

The first raw helper package left HardwareDevices empty and had no pre-New
settings document. Source review found two constructor defaults outside this
fixture's intended boundary: transcodehardware.Probe traverses default /dev paths
even with ProbeHardware=false, and updates.Schedule checks GitHub releases
immediately when the default UpdateChecks=true. No runtime or host device/network
access was executed or observed from the first package.

The corrected raw factory supplies one explicit nonexistent device path inside
its newly retained root, checked absent before Server construction. It writes
state/settings.json through the owned os.Root before server.New. That fixed
validated document retains libraries:["."], RequireMFA and picker defaults, sets
updateChecks:false, homeAssistant:false and jellyfinCompatibility:false. Nonempty
libraries is required for settings.go to adopt saved settings. Database startup
validates and imports the legacy document; no private setting method is called.
No real account or installation settings are read or changed.

The unchanged real database constructor consumes this newly synthesized private
settings.json after successful durable import into retained SQLite state.
packages/documentdb/documentdb.go, blob
42582fb4486abddb0cefa66a3030d65ac4e0e40e, removes successfully imported
legacy JSON at lines 73–78 only after store.prepare succeeds. The seed file is
not promised to remain after successful Server startup; its authoritative
settings remain in the retained SQLite store. This canonical startup consumption
is separate from fixture cleanup, evidence deletion or actual user-data deletion.
The helper adds no removal call, and its exact seed/source bytes remain pinned.

Config.Subtitles still supplies the actual owned provider URL/runtime key; the
installation settings schema has no provider credential fields and does not
replace those values. AuthURL remains the exact owned TLS origin. Empty DLNAURL
and disabled Home Assistant bind the existing no-SSDP/no-advertisement source
branches. These guards are source-grounded; no no-multicast/no-device/no-update
runtime proof is claimed. Historical raw helper and document pins are retained.

## Handoff

The sixteen source files are preparation only: three documents, two frozen Go
control/assertion files, and eleven new raw Go helpers. The newR16PublicFixture
factory is now authored from the exported Server and actual Owner enrollment
flows. The frozen controls retain their earlier failure-first comment about
missing helpers as historical source provenance; the eleven helpers below now
supply that seam, without claiming compiler or runtime acceptance. Request returns only consumed/closed bounded response values; OpenEvents
has a separate five-second real-header allowance and keeps the stream owned
until its actual reader joins. File reads use only the retained owned roots.
Stream and fixture settlement are idempotent. Checked socket/root/body failures
fail admission. Retained media/state roots are never removed.

The independent TLS target is allocated before environment/root construction.
Its fixed routes and dynamic item destinations are registered from the actual
bounded catalogue (16-character lowercase hexadecimal item IDs from library
SHA256 prefixes). Owner networking helpers receive only the network target,
contexts and runtime form values. They preserve the real cookiejar/TOTP/current
CSRF; the fixture neither sets auth state privately nor fabricates public
responses. The provider peer accepts only GET /api/v2/me with its exact runtime
key, emits factual quota17 and counts real requests. All output bodies and
credentials remain private. Current event capture is bounded to sixteen queued
frames, 1024 data bytes per frame and 4096 scanner-line bytes.

Cleanup cancels the owned lifecycle, closes/joins SSE and HTTP handlers/readers
and the provider, then closes os.Root descriptors only after that work has
settled. It records transport settlement separately from admission health.
There are no process launches or directory/file removals in the helper sources.
The actual application may perform its own temporary readiness write probe only
inside the disposable configured media directory.

The fixture's StopAndJoin acknowledgement covers its owned TLS listener, HTTP
requests and handlers, SSE/body readers, provider peer and root descriptors only.
It does not acknowledge every internal application worker or database shutdown.
The managed Server closes its database and cancels scheduled workers internally
without a public join API. Cancellation is a request, not a completion witness.
Future outer Go-process settlement remains required and unrun. Retained roots
remain present regardless; closing the fixture descriptors never deletes them.

No compiler readiness or runtime PASS is inferred from RAM source checks.
Owner/anonymous/CSRF/revocation controls do not claim a separately authenticated
Viewer case, automatic upgrade, overflow fallback or browser layout acceptance.
Canonical formatter, lint/security, compiler, actual public controls, browser
RED, product changes, GREEN, normal required gates and merge are pending.
Existing R06 protocol, Restore/Save proofs, Q47 and native files remain untouched.
