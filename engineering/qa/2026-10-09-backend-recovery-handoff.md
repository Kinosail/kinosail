# Maintained backend recovery source: PR530 and PR534

Owner: playback backend recovery thread 01a111fd-c299-75f4-bda5-8abd2bf0d000. Maintained branch: codex/hls-backend-recovery-maintained-20261009. Native client and simulator ownership remains separate.

This is preserved recovery source, not a production delivery or merge admission. Both old drafts can close unmerged under the user's merge-or-close request once this exact branch and ledger are verified.

## Exact relationship and preservation

PR530 a99b6032ca707b96c70c8e123fafea3f6b1ab996 contains useful, unique production-language helpers: strict presentation/AAC metadata validators; native/source-normalized AAC clocks; the bounded two-process source collector; retained/rooted source and private-asset acquisition; restricted AAC-LC configuration/edit/first-payload readers. Its21 production paths and26 test paths are retained by their exact Git blobs in this source tree.

These helpers are not discarded as 'only diagnostics.' The published530 has no preparation/readiness caller for the private/source wrapper and no accepted new producer certificate. Its two existing-runtime changes add optional Presentation metadata and fail-closed validators/projection conditions while preserving legacy nil-origin bytes. Actual source/asset observations and adversarial tests qualify the helper slice only. Joining a producer, generated-media certification, refill/reopen and native presentation still require implementation and proof.

PR534 a7b5d42d0852d611bc2e8ccdf3c20c36af09ec8c is a separate, two-path attempted AAC preroll repair. It does not contain or supersede530's helpers. Its published video-only preroll rule is insufficient: four lost packets are restored but one packet repeats, clocks disagree and full PCM differs. Exact normal CI37926378478 failed the Player media gate. Never merge that scope-only head.

This maintained source retains the complete R16 tree7133c942f3a3cf0670a661f1516881b28c714a11 from fd24092c93f08312b6a7af05b70b67644f22cc8b, including all530 helper bytes and the independently reviewed R15 DTS clip/refill proposal. It reconciles all nine incoming PR494 paths from main664f4e2ad40049b03a5da62dba444adc3caa97ec byte-for-byte. It preserves original530,534,R16 and currentmain ancestry through ordinary merge parents. Neither original branch is rewritten or deleted.

The new written-first source-witness/rescale contracts are separately preserved in ancestor 0e53795ceb399eb095309095bbd30dff06a14670. That contract tree is unexecuted and non-buildable until its proposed fields/helpers exist; it is not a runtime RED receipt. Its three test/document changes are not included in this maintained source tree. The tests-first matrix records source ambiguity, malformed tracks/edits, rescale carry, ownership, source/generation races and cache isolation before production transfer.

## Terminal runtime evidence

R12 run37924562167:933AAC, omission934→939, failure.
R13 run37925896961:943AAC, duplicate934,960512samples versus960008, failure.
R15 run37932586143:942completeAAC restored, but a two-tick edited boundary and964608samples versus960008 remain. Observation success is not acceptance.
R16 run37936887691 at fd24092c:terminal failure/incomplete. Four arms observed; explicit-discontinuous initial mux invalid/unavailable. No unchanged rerun.

R16 normal-initial mux with packet-derived+4600ticks:942exactAAC559..1500 including935–938,480exactvideo frames/payloads,constant edited AAC offset,zero clock/ordinal transitions,whole960008sample PCM reference hash,completeEOF,source/canonical identity unchanged. This is fixed2s CLI evidence, not byte-identical indexed0.1s public argv, browser/native or cache acceptance.
Receipt27143968c4c10463ac0c174f618fac865ad78fb8d37447cf570b86c7c5515d95;artifact11618871666;APIarchive digest sha256:81b36359a66ead4be34c7df3eb6885e08ec7948b93870e18c235dfdca8f43406. No archive download/independentZIP rehash.
Pinned JellyfinFFmpeg8.1.2-5 package b4e72894ad26c809ed0104805f5415a97be75212b9fcf9d60b89ad25bb3d43e3; source commit5b16ac4eeef9559e652faac4a5907c3de6e15eb8.
Historical failed arms, observations, private receipts, workflow refs and API digests remain preserved. Normal checks do not override strict runtime failures.

## Next concrete production blockers

1. Bind the first canonical AAC payload uniquely to retained source SHA/integerPTS. Raw/edited generated edit difference alone is insufficient; require equal payload/duration and equal PTS/DTS edit deltas.
2. Derive separate typed per-track physical/logical origins from actual seek/mux microseconds with integer rescaling. Guard fractional carry, invalid track/grid, bounds and nonzero initial video clock. No fixed4600 or PCM-selected shift.
3. Select a source/policy/recipe/track-bound V2 producer expectation outside manager.mu; hlsSettings must remain subprocess-free under validation locks. Completed unsupported-grid decisions retain legacyV1. Transient/malformed qualification must not downgrade.
4. Give only verified indexedV2 a local effective-policy discriminator. Reuse-only rejection is insufficient: startupWindowReady, masterFresh/clock completion, existing init/fragments, prepareSegment race winners and active same-policy workers can bypass it.
5. Require V2 timeline/certificate/generation at readiness and direct asset delivery, including an admitted opened asset. Replace V1 jobs through existing cancel/join semantics; reject indexed read errors instead of ordinary unindexed fallback.
6. Use one inherited2s lease, existing owned probes and retained rooted source/init/first descriptors, with final source/policy/hash/generation checks. The legacy video scalar's plain Cmd.Run is not a new whole-process certificate.
7. Execute actual authenticated public prepared refill4, full ordered AAC/video/PCM/EOF, reopen/zero/missing-first and interrupted-unindexed-cold counterevidence. Then exact independent review, currentmain reconciliation and unchanged normal protected gates.

The narrow MP4/H264/AAC-LC stereo48k mechanism does not grant MKV, HEVC, audio-transcode, arbitrary resume, non-key native presentation or fourteen held-fingerprint eligibility. All14 inherited holds remain unchanged. The preserved50 live simulator baselines are not repaired/retested; that next full sweep requires the separate native owner and authorized deployed runtime. No Nox deployment is changed or qualified by this checkpoint; historical f532 receipts remain preserved.

Mac capacity hold and disconnected read-only execution transport prohibit local filesystem writes/builds/media/downloads/cleanup. This reconciliation uses GitHub objects and metadata only. No original-media mutation, credentials, Library writes, simulator action, Nox update or manual deployment.

## Later verified checkpoint: 2026-10-09

The prior source and R16 evidence above remain intact. Later candidates are preserved on separate owned refs.
Fresh remote main is `922e7fa65cb3752d4a204b6e14782f58c6133fd7`, retaining delivered PR493/e30 ancestry.
No later backend candidate is merged or admitted for production.

### Delivered PR493 and original PR490

PR493 reviewed head `6907a9b733ca8fcea7a545636edfa49796518e84` is merged at `e30de246899d583802d08038cf8623f34bc2eaa4`.
Exact e30 CI/publication [37506502844](https://github.com/Kinosail/kinosail/actions/runs/37506502844) is terminal SUCCESS: 47 successful jobs and four intentional skips.
Both architecture assemblies, image signing, provenance attestation/verification, commit tags, production-tag promotion and documentation publication passed.
Player publish112430377793/promote112430788660 agree on `sha256:87bd9436f150ca7a454e2aac965ee8a20da094fa165a82d4d16ccb35ec2d63da`.
Subtitles publish112430377898/promote112430788638 agree on `sha256:2a37c2f43ae0560d18c47fecd4444c60e6333415236cbdae4c4e5c0e928c5625`.
These are exact e30 publication facts; current922 image publication and deployed Nox revision are separate boundaries.

GitHub auto-marked diagnostic PR490 merged when its original head entered main through493 ancestry.
Triage thread01a11221-d2d7-7711-a9f1-d380830b0020 is the sole writer for its create-if-absent branch restoration.
Recovery made no original-ref or issue writes and did not race restoration.
The restored remote `codex/hls-prepared-timeline-20261006` freshly verifies at exact `ab81e2dec1274f8ed073d126f2fba852ea032072`.
Original branch/history/evidence and first failed CI attempts remain preserved.

### Current production candidate and qualified remaining regression

Owned branch `codex/hls-aac-v2-r17-20261009` remains at `4c9153393b7811f6d253acf85d3134e8fb5916a3` / tree `e42505917599bea932422602f5dc0c11962e9a59`.
Pending-init consistency passed all82 contracts and quality37997673457.
Proof37997673510 passed core5, cold2, malformed-master24, binding18 and complete39; its lazy job failed.
Four qualified baselines returned HEAD200/200, thirteen GET200 and480 exact source frames.
Adopted lazy, speculative lazy, missing4 and missing9 candidates returned HEAD404/404 and thirteen GET404 without source encoders.
Missing0 baseline GET404 remains unqualified for a new regression; its candidate did not run.
Lazy receipt `a0f4faee0d2dfb905da07f93ee466355a64150ccfe42e2b16cc9bdc87adba2f0` and artifact11648570346 remain preserved.
API archive digest: `sha256:8d5770f3fe1778b1eef0e60bb423409e6dd1fa51be3037d05abd2a7805dda080`.
Physical source/assets/startup markers and owned joins passed; the composite preservation flag also requires hydration.
Future proof must separately project retained-file equality and exact permitted additions.
GET-only compatibility remains required. No client POST, native-call requirement or generic canonical-cache fallback is admitted.

### Accepted private CLI and narrowly qualified observer evidence

CLI diagnostic `codex/hls-aac-v1-stage-proof-20261009` remains at `71ef876ac97666889ca663f1e687bbde0ccf3dd7` / tree `511a4c6de7aa980efe60293a038b7ea3c3eed486`.
Run38001378568 passed private cuts0/4/9 with the independently bound old clock and captured seek/offset.
All cuts matched exact init, complete packet clocks/payloads and48 source frames; cut9 whole-fragment bytes differed and remain recorded.
Receipt `0184deb70e8ce7ba908aa9689ec666910d168f5feca93a77d492a1eba40a0646`, artifact11649341663, API archive `sha256:e3e4194e778a732100bde7b8e8c9e72de2ac4d49bea1b64dbb5b48ffd15033e6` remain preserved.

Written-first live head `15e09ca1661d21200a814d8079647161a8b98c2f` qualified four causal failures in run38003516837 while the old20 controls passed.
Artifact11648944910/API archive `sha256:9c2fece7e5d8d0473b67ab15d7d5da2460835ee1d1b047de2ace1014b8ed0b38` retains that RED.
Observer branch `codex/hls-aac-fd-observer-proof-20261009` now holds `d6194353f26afc7c7e504e73e3c860f2dd6d4193` / tree `62c103782cb9dfd8ed7c9b31f97af973f4f9c1ad`.
Independent exact-source review admitted one bounded run, [38004726356](https://github.com/Kinosail/kinosail/actions/runs/38004726356); it is terminal SUCCESS with all24 unchanged controls passing.
Artifact11650736546 is3233bytes; API archive `sha256:87217f022ddbb0babb6652728925999fb42865cdd2f55c3fd7810961344b32a2`.
Actual retained-FD and permitted startup children count1. A wrong other-inode FD raises the exact qualification class; the wrapper records its actual target identity.
All four actual children were alive with pinned argv/input; every group had two zero observations without qualification errors.
Source stayed unchanged, unresolved owners were zero, and all retained FDs closed afterward.
The executed live-test SHA `0ced6815ea6a457eca74cec8f4568ebee36ac1b580e017403dfc1c4af4a15095` is unchanged from the causal RED.
No artifact ZIP was downloaded or independently rehashed; stated archive digests come from GitHub API metadata.

### Explicit next admission boundaries

Observer admission covers the four witnessed argv forms containing `-hls_time`; default-HLS signatures remain unqualified.
Initial-source swaps, transient proc observations, real Server lifecycle/join and owner/migration controls still require written-first proof.
The private CLI does not authorize GET hydration. A typed Version1 worker still needs owned job/governor/source/root fences and absent-cut-only publication.
No replacement PR or merge is admitted until full compatibility, current-main reconciliation, independent exact-source review and protected checks pass.
All14 scanner holds, historical AAC failures and preserved50 simulator baselines remain unchanged.
Native50 retesting and pairing remain with the native owner; no simulator action or complete50-retest claim follows.
No local files/builds/media, credentials, original-media changes, Library writes, cleanup, Nox update or manual deployment occurred.

### Qualified partial-read and retry checkpoint: 10 October

Main remains `922e7fa65cb3752d4a204b6e14782f58c6133fd7`.
The lazy-read branch preserves maintained13c3, observerd619 and main922 through `b1132e4434a945feab00c8252a1b12010db721ec`.
Incoming native files and the normal layout workflow are preserved.
Original490 remote preservation remains triage-owned; this recovery did not touch its branch or PR.

Partial-read production source is `c295ed28b0232054616d74e6d9fa970e59d60d51`.
GET/HEAD retain certified existing bytes while only later cuts may be absent.
Mutation paths retain complete-only admission.
The read path starts no worker and changes no cache bytes.
Its written-first GET/HEAD/range, lease-release and fence controls passed in38007959190; quality38007959295 passed.
The Lstat-to-open identity gap remains unresolved.
The missing-cut worker is not implemented.

Exact `80c3eba5fa052fdd609e3eb7b84084a2ce8e91ac`, tree `feddb3c41cff0510742313c89520ff3a4478d134`, passed77 Python and45 Go controls.
Quality38013491052 passed.
Proof38013491061 passed interrupted cold H264/HEVC reopen but failed five other public jobs.
Its missing4 candidate was qualified: retained reads passed, absent cut404, no encoder calls, unchanged cache/source and clean joins.
Other lazy arms had observer failures.
Their failed receipts remain preserved.

Written-first `82f4c3c42f09ded6223df9965921ad568aba0b09`, tree `50336ba8f6177b335afbead5f5f50f8d72386216`, ran86 tests.
The75 unchanged controls passed; eleven methods failed only new diagnostic requirements.
All17 actual codec groups had clean source, joins and retained-FD closure.
The new actual reaped-child absence proof completed before its missing-diagnostic assertion.
Proof38014818808/job114102644963 preserves the diagnostic RED.
Quality38014818829 passed.
Artifact11654614113 has API archive digest `sha256:dd3d048db469dc63718941658630a62e08f19c73a735ec3ef6999cb871b42c1f`.

Independent review admitted diagnostic source `0e98cf9e5bc2cc7722ba2f6b11a34594a9ecab20`.
Its tree is `c32b3c1cb8d2d8520bfafc6c0e25b44a5a9db350`.
Retry extraction preserves the20ms deadline,33-read cap, source/parent/start checks and rejection decisions.
Fixed retryOutcome and exact bool-or-None terminalProofCompleted survive both normalization layers.
Receipts explicitly hash the observer and retry helper.
Proof38015123698/job114103574842 passed86 Python and45 top-level Go controls; no failures or skips.
All17 actual codec rows passed, with two zero joins each and no qualification errors.
Source stayed unchanged; final owners were0 and all retained descriptors closed.
Quality38015123742 passed.
All six public jobs failed, so full compatibility remains unqualified.

Master receipt `f21936897655591eb7f0052a88299d8ae876644c5282a3befbd9f1305e3eb1a9` identifies one concrete observer failure.
It records arguments_before, R-to-absent, empty0, attempts6, deadline-after-terminal and terminalProofCompleted=true.
Final joins had two zeros, no remaining owners, qualification failures or cleanup failures, and unchanged source.
This proves a completed late proof was rejected.
It does not measure either global ps call individually or explain other jobs.
A PID-specific query experiment was proposed; no query optimization was implemented.

Lazy receipt `2693bc19c265cd5df8cafc46c0cac85a6c0c7f1094348cc4e2744fbc80ba1aed` supplies four independently reviewed worker REDs.
Adopted lazy, speculative lazy, missing4 and missing9 baselines each pass13 GETs and480 exact source frames.
Each baseline has one legacy refill.
Candidates pass both HEADs and every retained GET with response hashes equal to baseline.
Absent cuts return404:4–9,4–9,4 and9 respectively.
Candidate encoder calls are0; source, cache and retained files stay unchanged.
Every baseline/candidate sampler has0 errors.
Every group has two zeros, no remaining owners, qualification errors or cleanup errors.
Seed and global owner sessions are also clean.
The full-completion predicate remains false because missing additions and adoption have not occurred.
Missing0 remains baseline-unqualified and its candidate was not executed.
These four arms qualify read/HEAD behavior and missing-cut failure only.

| Proof38015123698 artifact | ID | API archive SHA256 |
| --- | --- | --- |
| Contracts |11656385073|b7678e36de47243eab944129959fc116922bf38474b7ad9e6bd1db5ea72240e4|
| Master |11656200357|61695b5864d980e96a6ce7f81c903925f08532fa547fc61d6bac902b32192187|
| Compatibility |11655845633|68bfdc63f2f65cf18d8ad60f3c414e5e22c162d446d6a92145408eaed085c76f|
| Binding |11655805624|84aee737f472a15703ab0f7a20a4fd6cf51862ab8046a6e6cb8c049df49900ea|
| Lazy |11654764752|a6f998fa68b34980a62472302831810050333960a67231daebdffebd511cc762|
| Public |11654699805|0b0342f58e4c694ece192d6bba52f0df118627e427062766156f2d1e0dacc996|
| Cold |11654524953|77042232cfec0ea2bca42ee058a3c0381d78fea67e700304311c30d4a84ff813|

Archive digests above come from GitHub API metadata; ZIP archives were not downloaded or independently rehashed.
Detailed prior REDs and observer corrections remain in engineering/qa/2026-10-09-hls-version1-partial-read.md and branch ancestry.

Next production admission requires written-first Server job/governor/group-join and cancellation/migration/race controls.
The worker must use an owned private stage and publish only absent certified cuts.
Retained init, manifests, source, certificate, startup and foreign files need explicit preservation fences.
Do not widen observer budgets, weaken assertions, scanner holds or gates.
Full compatibility, exact-main reconciliation, independent source review and protected checks remain prerequisites for a PR merge.
No PR, merge, release, native or complete50-retest admission is claimed.
No local files/builds/media, credentials, original-media changes, Library writes, cleanup, Nox update or manual deployment occurred.
