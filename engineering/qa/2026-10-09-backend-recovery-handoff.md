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
