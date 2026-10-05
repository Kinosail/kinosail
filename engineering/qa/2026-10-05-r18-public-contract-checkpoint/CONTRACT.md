# R18 public contract source checkpoint

Date: 2026-10-05. This is source preparation, not new runtime acceptance.

Current main is `bd1ea2e148787ae8d4a0a46640bc2a965e10fe6a`,
tree `6070e86cc8a3522810218d311cc2d5d6246f02d0`.
The preserved staged implementation is `ce4f2ec3d4512fb57f42656430426cdc17216ecd`.
Its worktree currently ends at `0433888e7f3afb0b35aec3beac3031d6097e0ea2`,
a later diagnosis-only commit. The two existing dependency symlinks remain
untracked and unchanged. No local file, index, ref, commit or source was mutated.

## Audited behavior and authority

Canonical backlog R18 / CB01 is "Multiple web playback tabs share one Home
Assistant target". The original audit explicitly recorded runtime_verified=false.
Source showed one localStorage ID per origin and Viewer Profile, with every tab
publishing one Server record and listening to one command resource. That audit
alone did not prove a runtime failure or an unauthorized cross-Profile action.

The approved behavior gives simultaneous browser players distinct target IDs,
keeps stable entity references across normal reloads, and defines tab lifetime
rather than physical-device lifetime. Tokens belong only to the active document.
Native ID and strict Player/Command JSON contracts stay unchanged. No browser
fingerprints, household targets, deployments or real account changes are authorized.

The reviewed staged contract remains authoritative: Profile-scoped sessionStorage
candidate, document-memory server claim, one existing 64-record map, 30-second TTL,
no takeover, bounded 35-second reconnect wait, and truthful Retry/storage limits.
Reload and history returns keep their candidate; fresh-navigation live conflict
may fork because copied storage and lost-release navigation are indistinguishable.
This explicit lifetime limit must remain disclosed. Navigation type grants no authority.

## Historical proof, verified again by read-only evidence inspection

These receipts are retained byte for byte. Their hashes are SHA-256.

| Receipt | Source revision | Receipt hash | Reached boundary |
| --- | --- | --- | --- |
| command-baseline-receipt.json | 49915982ad9dc7dc6a85780f181cb2974de81666 | 67715a25093e2a78d4eae14177ffa1e0971d33114f3854b28278458742dd5af2 | Actual disposable Go Server/Chromium: two shared-ID failures, two isolation prerequisites blocked, two harmless seek controls PASS |
| route-red-reviewed.json | 507c513e3eadd389e6e45c5eed1d4c48bebc4a8e | 351167e23fe792737172a124f2f6448c2086df76b991d5143f645af3c18aed76 | Five registered-route tests reached HTTP 405 at the absent proposed claim route; later ownership assertions blocked |
| deadline-red.json | ab206462c386c1b4b8f1d36d4984c939c1eea11b | 429cc6b60af9754b4b23e03e919dd39f6f90b8263ebe53704df50467024c5c39 | Two intended isolated routed-409/virtual-clock deadline failures |
| deadline-green.json | ed36a5c6960f95df49650a27f6a3c6da22e7a1b9 | 415d01635d686b2954962768f85ebdaa7d6639f0464d5f103261e10221696842 | Same two isolated deadline cases PASS, zero skips/flaky/global errors |
| protocol-green.json | 4528adba9d016a4a882ea6b64ae8b26f7685d819 | 56c3dd67616c55dd619d1362edd36471b5d96a7224ae4e031bab357f5a8b7522 | Controlled registered Go protocol: 30 parents plus six subtests PASS, zero fails/skips |
| csrf-green.json | 22fb2c4654c05f304143f07ab6a820ce0a62bf91 | aca7bd885094b003c70fee85e34a774c24f1ffd6c3c36cd201c45397b51a0659 | One actual authenticated app-handler/session-cookie/CSRF test PASS, zero fails/skips |

The raw baseline report hash is
`6971288a74067d79b4f371298895fda51939357eaaeb202c90777d08ff342632`.
The private raw baseline log hash is
`bf3d00f8031f58a1ad9e49c5f8eacb050324b1708301ef75ba089e6f92b0e697`.
Both match the retained receipt. Raw logs, bodies, URLs, tokens and cookies are private.

The actual baseline "two actual browser tabs publish distinct Home Assistant
targets" failed twice at home-assistant-tabs.spec.ts:49: expected two target IDs,
observed one. This is an incumbent runtime reproduction, beyond the audited suspicion.
The "public seek command affects only its addressed fictional tab" cases failed
twice at line66's same uniqueness prerequisite. They did not reach the isolated
command-effect assertion and must not be reported as proven wrong-tab delivery.
The "harmless public seek reaches one registered fictional player" cases passed
twice. No baseline result skipped, retried, flaked or reported a global error.

The baseline used a real disposable HTTP Go Server with authentication disabled,
two fictional copies of the approved clip, and public state/command routes.
It did not establish authenticated Profile switching, household integration,
ordinary LAN browser compatibility, or the claim implementation's browser GREEN.
The immutable fixture identity was
`9dbd85e7863d921e209978c8349992e5f8505b0d7ffa468c37b8caf97af3f0a4`.
No clip was transferred, substituted, generated, opened or decoded for this checkpoint.

The retained deadline RED/GREEN, protocol and CSRF artifact maps were rehashed:
10, four, five and five artifacts respectively, with zero missing or mismatched
artifacts. Raw Go terminal accounting matches the curated 36 and one PASS records.
Protocol source aggregate is
`ec03b3ee0a4a63c807215e2825e102470be73060f614a169615e9349fb46a6e0`.
Its retained binary hash is
`5e373f80f779e4c5f82c47a11dd5120c336febf51c977d120d82c280763c6a00`.
The actual-CSRF binary hash is
`cdccdbf69e0548a6f4fb7d84fe2c640b37cb879d2d31513271fa02f832fcec38`.
These are historical products; they are not products built from current main.

## Exact source boundaries and reconciliation

| Path | Current main Git blob | Staged ce4f Git blob |
| --- | --- | --- |
| packages/homeassistant/http.go | 245a18cc5ddd2ad0ad921dd0dd6ca03f1407633e | bc7a436e7613a291d5165d70597ae6b57ef9b824 |
| packages/homeassistant/player.go | ceb2130f568d0c5300f4dd2c2c5dc8868efbf076 | a65b755e0745acee71c4ae6a604498b631dbc8d1 |
| packages/homeassistant/integration.go | 11598c3bd1360b71d19a7d662353fdffbe500f20 | same |
| packages/homeassistant/claims.go | absent | 925315375658a39259fa5a78173d5ad05ab492e9 |
| packages/homeassistant/claims_http.go | absent | 681fe1fcc72f65cfe78f8852a4dc79f4180e4247 |
| packages/homeassistant/claims_http_test.go | absent | be6a8bf7ce0ea0651f50dfe0085ec7077164827d |
| packages/homeassistant/claims_diagnostics_test.go | absent | f1287052fc26cb266d68e593a517203b31808708 |
| apps/player/internal/server/home_assistant_claim_csrf_test.go | absent | cd54f35b624d09890f0a7ceab5ff951c5c061670 |
| packages/webassets/static/player-home-assistant-identity.js | absent | ff5927c25666f011250a19d3e7d26da1a820c76e |
| packages/webassets/static/player-core.js | 7a06c5d21d4a75c2577c87570456489fe5e1c1c7 | 7cc11c814b1107d038ffd9c8d0f21c7062bf27ed |
| packages/webassets/static/player-progress.js | 43443602359f97a9088108527096c5c8ca2370c5 | bec9008a8274b2d0b72084008d213bc49c134d77 |
| packages/webassets/webassets.go | f0bdc0a8cc097dd621d2fc9949ff02ea4d58c7fd | 16044207527dec8339352eb4328a0e3ed672cd98 |
| apps/player/internal/server/assets.go | 56d4a231419ab69262185c16fb2e4efb9db9ab0d | same |
| apps/subtitles/internal/server/assets.go | 4059870e0f0155a5ce76071e6687afc354023abc | bacee986a98cb94c7323467f38b7568e1119b92e |

Current main still stores a shared localStorage target at player-core.js:18-21
and uses that ID for state/SSE at player-progress.js:202-244. Go registers no POST
claim/release routes; state overwrites and drains by ID plus Viewer Profile.
These are source facts, not a fresh current-main execution result. Same-Profile
updates to one native ID are existing supported behavior, not a newly claimed
security defect. The browser's shared identity is the R18 incumbent bug.

Do not reconcile ce4f wholesale. Preserve the nine native playback sources,
the current progress/navigation acknowledgment and teardown behavior, and current
PlayerProgress composition including player-progress-navigation.js. Preserve
Subtitles joinScripts(source cues, Save operation, inspector) composition.
Q15's nine independent prepared source objects stay untouched. A later browser
integration requires renewed exact seam coordination and content-hash delivery proof.

Other current inputs: api_test_helpers_test.go is
`1dff92a02e53cd6de99f842844b7c10381d73915`; servertest/api_helpers.go is
`4b2a73be66934e1c5ee49c1baf4f9447346a30ea`; productapi/me.go is
`2eb04187ca3ad246720c680a7911955511227699`; existing app state-CSRF test is
`2d7d9bf4d3bd5db11a5ab75c326a8e5f18fddef2`.
httpguard/csrf.go is `21ce4421beb125598ea0abe39fc60e8975b909a3`;
httpguard/json.go is `4707d57cbecde54713a49a1b9b91213ce06d064d`.
Root AGENTS is `1972e460c934a674dd361643453cdcd7350fe754`;
Player AGENTS is `b9e55b9584981206c0229f738b8cfe49590eeea8`;
Player DESIGN is `0afc08a66654009c67e326751a0e57002f7555af`.
The public-interface/test-first guidance applies. No visual or skill-context
launcher was run; there is no UI change in this preparation slice.

## Failure analysis before new test source

1. Two generated document claims may alias one target or share authority.
2. An unaddressed target may consume another target's queued seek.
3. Same-Profile sibling claim, missing/wrong/duplicate/oversized claim header
   may overwrite position, release ownership or drain a command with valid CSRF.
4. A successful release may churn the stable candidate or reuse retired authority.
5. A stale valid-CSRF document may delete or overwrite the replacement owner.
6. A strict native state/command schema may become permissive or require new credentials.
7. A setup/Owner/CSRF/missing-route failure may be misclassified as intended defect RED.
8. A hung public request, oversized response, incomplete capture or source/product
   drift may be credited as completion.
9. Source-only tests or handler proof may be presented as browser storage,
   reload, expiry, BFCache, secure-cookie transit or authenticated Profile proof.

Existing isolated registered-route tests cover clock/Profile/reservation/65th
limit/restart boundaries. Existing app CSRF proof covers invalid CSRF. The new
gap is real app behavior with correct CSRF plus wrong sibling authority, and
independent claimed targets through app middleware. No private integration
method, mocked success response, direct record write or fake Viewer switch is used.

## New test-first source, append-only and unreferenced

Only the three new paths approved by root are prepared. All were absent on main.
Two Go artifacts and this QA contract form the final source package.

| Path | Git blob | SHA-256 | UTF-8 bytes | LF lines |
| --- | --- | --- | --- | --- |
| apps/player/internal/server/home_assistant_document_targets_test.go | 69f15a59a3291dbedcef0df575f851e4c95c4d50 | 18ab485d6f82196554b598152a15984f44db06cc2620e5cd04f17b2b9bccb0c5 | 4465 | 91 |
| apps/player/internal/server/home_assistant_document_targets_helpers_test.go | 1750f363773b4d86b200434a0bf387bc60eecb4b | 8de16a1fbb631a76bf22378c3fca25e6cff7dfda55239437e58e8ed377316cb8 | 7998 | 232 |

The 315-line first test draft exceeded the repository 300-line gate. A mechanical
split moves its exact imports, declarations, helpers and comments into the helper
file, with only the tests' required import block added to the 91-line test file.
Concatenating the helper source and the test file from its first Test function
reconstructs the original 12396 bytes exactly. All 16 functions, every assertion,
fixture, request/body bound, route and comment remain exact. The four test bodies'
combined SHA-256 is
`2f9756984118691f7594ee9e2f3b77da8b3905bfef957e27e4ed9928cb49b4e3`.
Both files have formatting headroom below300 lines. No formatter or lint ran.

Excluded append-only drafts remain preserved:
- Original full test blob `e0b1864f09144b49e2c14e0ede0b83ff4b15ad3a`,
  SHA-256 `9ebe2276ae5d3758017ec842f11a013927c91ff41fb0fba9f91ff17b3f1f98b0`,
  12396 bytes/315 lines. No assertion was dropped to meet the cap.
- QA blob `62a64060e2248af9a69c1e5da16f33a38713114f` contained an accidental RAM
  replacement prefix duplication and was superseded before review or reference.
- Prior unsplit QA checkpoint `38c7284d25b6e0c85112efe2915210d809123650` is
  superseded by this split manifest. No prior evidence/source was deleted.

Four exact top-level test names:

- TestHomeAssistantDocumentTargetsRemainIndependentThroughPublicApp
- TestHomeAssistantDocumentSiblingCannotOverwriteReleaseOrDrain
- TestHomeAssistantDocumentReleaseKeepsCandidateAndRejectsRetiredClaim
- TestHomeAssistantDocumentNativeStrictContractRemainsUnchanged

The canonical apiServer/APIFixture enrolls a disposable Owner through actual setup
and MFA API routes. It writes existing synthetic byte placeholders to its temporary
library; no playable fixture is required, served or decoded, and no playback or
encoder route is requested. No alternative credential-setup path is introduced.
The returned actual session is presented as the existing secure-cookie test seam.
Public /settings supplies real CSRF; public /api/v1/me confirms Owner identity.
Every tested claim/state/release/command uses that correct session CSRF.

The proof is the real application handler over synthetic HTTPS requests, not a
socket/TLS/browser journey. Document claims model independent public clients of
one Viewer Profile, not actual browser documents or persistence. Requests settle
synchronously with a three-second context deadline and their body closes afterward.
Authored bodies are bounded to 4096 bytes; the capture writer rejects output beyond
262144 bytes before retaining it; JSON decoding is bounded to16384 bytes.
No spawned request goroutine or SSE stream exists in this test file.
Raw server/fixture logs remain private; custom failures include no tokens/bodies/URLs.

The native control preserves unknown-field rejection in both state and command
JSON, tokenless registration and exactly-once queued seek dispatch after rejection.
The sibling and retired-authority cases preserve every known published Player
field and require that the legitimate queued seek still drains exactly once.

## Proposed next bounded Go-only phase, not an execution grant

Root owns admission, integration, drivers, scheduling and publication.
The first test-only candidate should contain current main plus this new test.
An existing Go claim implementation may be selected only in a separate later
candidate, after the new route prerequisite is recorded and exact review passes.
No browser/native/Save/shared composition source is needed for this Go-only slice.

Compile separately with one Go build worker, offline dependencies and GOMAXPROCS=2:
`go test -c -p=1 -o <fresh-private-output>/r18-document-targets.test ./internal/server`
from apps/player. Proposed compile command cap90 seconds plus settlement within
100 seconds overall. Compile/prerequisite failure is not product RED or acceptance.

Then under a separate maximum45-second execution grant, use the same admitted
binary, Go test timeout35 seconds, count2 and parallel1. The exact anchored selector is:

`^(TestHomeAssistantDocumentTargetsRemainIndependentThroughPublicApp|TestHomeAssistantDocumentSiblingCannotOverwriteReleaseOrDrain|TestHomeAssistantDocumentReleaseKeepsCandidateAndRejectsRetiredClaim|TestHomeAssistantDocumentNativeStrictContractRemainsUnchanged)$`

Use go tool test2json -t -p github.com/MikeO7/kinosail-player/internal/server with
-test.v=test2json, that selector, -test.count=2, -test.parallel=1, -test.timeout=35s.
Root's external bound must leave time for owned process-group/capture settlement.
No Node, browser, Go downloading, media transfer, device or encoder is involved.

Require eight complete terminal records, exactly two per name, no skips/retries,
global failures, timeout, unfinished request/capture or source/tool/binary drift.
On current incumbent main, the three claim-dependent names can only establish
the missing proposed endpoint prerequisite (expected201, current source predicts405).
They do not reproduce a new incumbent contract violation. The two native records
are existing-contract controls. An unexpected HTTP status or early setup failure
must remain a separately classified prerequisite result, not forced into405.

On the selected implementation candidate, all eight must PASS. Public command,
snapshot, correct-CSRF rejection and stable-candidate assertions must remain.
Record full before/after Go/embed/dependency input manifests, exact candidate/tree,
toolchain, admitted commands, binary hash, timestamps, raw test2json hash and each
named result. Raw capture is private; a bounded allowlisted projection exposes only
test names, source locations, counts, known expected/observed state and hashes.
Source hash or publication alone never implies runtime GREEN.

## Remaining blockers and proof limits

No new build, test, browser, UI, app, deployment or account action occurred.
No source was edited in the preserved checkout; no old test or policy changed.
The three final objects are append-only source artifacts, without tree/commit/ref/PR/dispatch.
Independent exact source review and root integration/admission remain pending.

The narrow Go-only proof does not need the immutable clip and can be scheduled
after active Q47/Restore work. Full R18 still requires actual two-tab candidate
GREEN, normal reload and lost release, non-persisted history return, admitted
persisted BFCache lifecycle, ordinary HTTP LAN without Web Locks/randomUUID,
storage denial, stale/delayed replies and real authenticated Profile switching.
Those browser proofs require their authorized fixture/runtime slots; none is
inferred from historical protocol, CSRF or isolated deadline GREEN.
Current media availability or delivery is not assumed. No substitute is authorized.
Final independent source/evidence review and ordinary protected gates remain mandatory.

MAIN: NO - R18 implementation is staged historically; current-base integration,
new app-contract proof and complete browser lifetime acceptance remain pending.
