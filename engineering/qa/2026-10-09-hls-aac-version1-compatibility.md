# HLS Version1 client compatibility recovery

Status: test-first recovery; no PR, main merge or release admission.

Production tvOS requests HLS playlists through GET without a server preparation POST.
The native owner confirmed this path. Main 922e7fa65cb3752d4a204b6e14782f58c6133fd7
adds navigation and QA changes without changing that request path.

## Preserved reproduction

Pinned source: 543a2f535be2ab1e3206ad8120c038c3b7960ae8.
Baseline: 664f4e2ad40049b03a5da62dba444adc3caa97ec.
Hosted run: https://github.com/Kinosail/kinosail/actions/runs/37975088257
Receipt SHA256: 6de07fcdf3b268730353f147730784634b02afa5a21402478f00f1d03bddefac.
Artifact: 11639370023. Archive digest is GitHub API metadata, not a downloaded rehash.

The baseline created a real Version1 indexed cache from the pinned public MP4.
Its source SHA256 is 198a6808092cea2a5481afc24e79481e4966ae577cbb8c46c214737e2173a3ed.
Four independent clones preserved the source and adopted cache. Baseline master
and rendition GET/HEAD returned 200. Candidate requests returned 404.
Candidate requests started zero source encoders and left each cache unchanged.
A baseline restart restored the exact original responses.

This is an existing-client regression. A new POST requirement is not a solution.
The old diagnostic receipt remains preserved even when the new acceptance test passes.

## Feasible repair and limits

Qualification selects the new Version2 policy before serving an existing Version1
generation. The preflight then compares its old full base binding with Version2
and rejects it. Bypassing only this check is insufficient: policy validation,
assets, seek settings and workers also consult the effective Version2 policy.

A version-specific read contract can retain a genuine Version1 generation under
the exact current base settings. It must verify the whole binding, master,
timeline and Version1 certificate; retain source, generation and rendition
handles; and recheck source safety, settings and canonical generation before
responding. It must preserve the query renderer and media Range semantics.
A sticky positive Version2 eligibility decision must remain unchanged.
Mixed versions, unreadable metadata and invalid bindings must reject before
preparation. Error is not absence. GET must not silently migrate a cache.

Complete physical Version1 assets can use read-only delivery without an encoder
or cache writes. Missing lazy fragments need the existing lifecycle-owned
Version1 continuation, with a validated request/job-local contract through
seek, policy validation and publication. That continuation must preserve
source and cache isolation. Its concurrent migration owner must cancel and
join safely. A playlist 200 alone cannot prove these absent assets playable.

Version1 binds source size and modification time, not a historic inode token.
Retaining the present source detects changes during a request. It cannot
reconstruct Version2 identity or detect an equal-stat substitution completed
before opening. This is an inherited Version1 contract, not new Version2 proof.

GET adoption can remove a speculative .startup marker. The first immutable
cache test therefore explicitly uses an already-adopted seed with that marker
absent. Speculative adoption and lazy continuation remain separate blockers.

## Preserved fixture counterevidence

Test-only checkpoint 113af591f8836889428cbbeed03ca18590de2358 ran on unchanged
production source from 543a2f535be2ab1e3206ad8120c038c3b7960ae8.
Hosted run 37978808780 stopped before candidate cases: compat_seed_all_media_physical.
Receipt SHA256: c7c3597b3a0b6f0f882e499c4dcdaf9ee760605fe9f17bba2e9af957283bb1d5.
This is a seed-completeness failure, not a qualified compatibility RED.
The baseline preparation produces a genuine lazy Version1 cache. Its original
playlist 200 controls therefore do not prove all physical fragments exist.
For the separate complete-cache control, fetch init and all ten fragments through
the disposable baseline Server's existing GET path before sealing independent
clones. Record each response, prior physical presence, source calls and joined
cache/source state. This baseline-only hydration is not a candidate behavior change.
Lazy Version1 client compatibility remains mandatory and independently held.

## Qualified complete-cache RED

Test-only head 6d801195ae0738a2f2ae6dbcd8aa2bb58aa960de preserves unchanged production.
Hosted run: https://github.com/Kinosail/kinosail/actions/runs/37980045962
Receipt SHA256: 9eb7bc0441bd0e1dc64ca7ca2b4818f5e156cd37164fddfe9cde1a6fbe5c4ef2.
Artifact: 11641222032. GitHub archive metadata digest:
0bcd51b491b49305d0f4982f640a3b540b03dfa335b1ec7b09425f1d8e69a15d.

Baseline GET hydration served init and all ten fragments through its existing
Version1 lifecycle. Two joined source workers used offsets 12 and 20 seconds.
All served asset hashes match physical files after joining. The complete journey
matches exactly 480 independently decoded source frames before sealing clones.
Eighteen positive controls returned baseline 200/206 and candidate 404.
Each reached the zero-encoder and unchanged-cache/source checks.
Independent exact-source review qualified those eighteen as causal regression evidence.

Eight negative controls stopped before candidate requests at compat_generation_count.
Hydration creates additional seek directories. They are fixture-blocked, not qualified RED.
Select the unique bound .copy-timeline generation without removing other directories.
All Version2 contracts, strict media, cold-cache and missing-binding jobs passed.

## Written-first failure matrix

The new public test runs real baseline and candidate Go Servers on hosted Linux.
It uses the pinned FFmpeg 8.1.2-5 package, the same public source and separate
disposable cache clones. Candidate traffic sends no preparation POST.

- Complete cold and warm master/rendition GET and HEAD must match baseline 200
  and exact bodies. Warm-request controls reject an absent binding before exact restoration.
  A separate deterministic contract proves sticky positive eligibility; these
  public warm requests alone are not a policy-table witness.
- Init, first and final media GET/HEAD and Range must match baseline status,
  exact bytes and Content-Range behavior.
- A playlist-to-all-media journey must serve all ten fragments and exactly 480
  decoded source frames after the 12-second offset.
- Missing/wrong binding, missing/mixed clock, missing/corrupt timeline, master, init
  or first media must reject without encoders or cache/source mutation.
- Immediate, idle and joined snapshots must preserve every cache file's inode,
  size, modification time and SHA256. Baseline response restoration is separate.
- Valid start and session/start queries must match the baseline renderer.
  Empty, zero, negative, duplicate, nonnumeric, oversized, at-EOF and beyond-EOF
  start values must preserve baseline rejection and cause zero encoder/cache writes.
  Their single WARN must contain only bounded correlation and the invalid-start class.
- Deterministic tests cover a sticky positive Version2 decision during Version1
  reads, post-open source/root/generation/rendition/binding/master/manifest changes,
  and exact metadata-lease release with retained later-media network delivery.
  Canceled callers and closed generations under inherited leases must reject safely.
  Ordinary public HTTP cannot reliably schedule those in-flight boundaries.
- Each owned Server and child process must settle under the existing bounded
  process/resource checks. Receipts include exact revision, tree and script hashes.

Independent review found missing .copy-timeline with a retained Version1 clock
could fall through to preparation. Write actual cold/warm rejection controls and
an isolated sticky-positive routing control before changing that selector.
They must reject without encoders, cache writes or eligibility changes.
Genuine unindexed cold caches with neither index marker retain their old path.

Before full compatibility acceptance, add actual missing-zero, interior and final
Version1 continuation; speculative adoption; policy/source/generation replacement;
concurrent Version2 migration; and the actual supported client request journey.
Do not claim these are covered by the complete-cache test.

Retain all eighteen Version2 missing-binding controls. Retain the five Version2
media journeys with every one of 942 AAC packets, packet duration/adjacency,
480 decoded source frames and the full 960008-sample PCM/EOF oracle.
No assertion, security, scanner or required gate may be weakened.
The original all50 baseline failures and fourteen held fingerprints remain open.

## Preserved first reader counterevidence

Reader head 4dea2d5f7c6b86972294daa4ffe6f7627c42ccca did not restore compatibility.
Run: https://github.com/Kinosail/kinosail/actions/runs/37983325864
Receipt SHA256: fafe0e7194569b2a8aec8ce175dae827229f12a82acc77314ca76753a471b378.
Artifact: 11642312462. GitHub archive metadata digest:
3e88adb0a07e7956c83fb55c8dfa7a24a2bcf9593080471c354a62205e2cbbf7.

All eighteen original valid controls and eleven query controls still returned 404.
All ten malformed-cache controls passed without source encoders or cache/source writes.
Strict Version2 core media, missing-binding and unindexed cold proof passed.
The isolated lifetime, source/generation and sticky-eligibility contracts passed.

The first reader also required raw ENDLIST. A certified Version1 timeline can
render a full VOD from a checked physical EVENT prefix. Baseline lazy hydration
can fill all physical assets without rewriting that prefix.
Add explicit real-prefix evidence and a deterministic complete-assets/prefix
regression before changing this condition. Keep every full physical asset,
whole binding, certificate, prefix/timeline, source and generation check.
A missing physical asset still rejects in this first slice.

Quality counterevidence is also preserved. The first head failed the 300-line
test-file cap. Its test-only split 675c63c1760a5a17a27a6b85fea7492943a631df passed
that cap and contracts, then exposed five formatting, complexity and inventory
filesystem findings. Repair them without suppressions or weakened checks.


## Preserved prefix-reader result and master admission gap

Exact c845ee45d9874b51cfa806f7e4656ae2adafc46e completed all 39 public cases.
Run: https://github.com/Kinosail/kinosail/actions/runs/37985912538
Receipt SHA256: d5b859bc1f50bdff5108a0b7de214e323e5b4423c5153c980e9c5e75653144a9.
Artifact: 11643713118. GitHub archive metadata digest:
60d9748ba4d51919c6a0dc89f16b4d182b1b8ddbab2b1ee7506777db866c90a0.

The genuine physical EVENT prefix contains four cuts, no ENDLIST and all eleven
physical assets. All eighteen complete-cache playback controls and eleven
query controls match baseline bodies and statuses. The journey matches
480 decoded source frames. All cases preserved cache and source and used
zero candidate source encoders. Nine malformed-cache controls reject.

The wrong-master control alone returned 200. Complete Version1 acceptance
remains false. The shared master selector skips every non-policy comment/tag.
The old receipt lacks the mutation coordinate and enclosing tag or attribute.
A diagnostic-only child records fixed-enum tag/attribute names, integer byte
coordinates, grammar booleans and before/after hashes before that assertion.
It never prints manifest content, source policy or paths.

Before a production correction, classify this actual mutation and write
cross-version malformed-master controls. Preserve historical bandwidth
estimates; existing Version1 certificates do not authenticate an arbitrary
syntactically valid bandwidth edit. No assertion or gate may be weakened.
Lazy continuation tests remain independently held.


## Written-first cross-version master failure matrix

A separate bounded public job builds real old and candidate Servers from exact
source. It seeds genuine complete Version1 and Version2 caches and joins all
source workers. Both positive candidate journeys must serve ten cuts and
480 exact decoded source frames without encoders or cache changes.

Independent clones then damage the bandwidth-policy marker, remove STREAM-INF,
make bandwidth malformed, duplicate bandwidth or STREAM-INF, add an unknown
tag or attribute, remove CODECS or duplicate the rendition.
Other clones replace codec, geometry or video range with syntactically valid
values that disagree with the certified init. These are semantic controls.

For all twelve faults in both versions, master and rendition GET/HEAD must
return 404. Startup, planning, immediate, idle and joined observations must
preserve every cache file and source. Candidate requests send no preparation
POST, start zero source encoders and emit one safe correlated WARN each.
Failed setup and process joins remain in the exact-source receipt.
The separate strict Version2 packet/PCM and missing-binding gates remain intact.

A missing STREAM-INF can make an existing master look nonreusable before the
playlist reader runs. The rejection boundary must precede any preparation
that could delete or restart that malformed indexed generation.
Do not replace historical bandwidth estimates or invent a Version1 master hash.
The inherited certificate binds timeline, init and first media, not every
possible syntactically valid edit to a historical master.

The first cross-version master run at `2ddc704c` stopped during baseline
fixture setup, before any candidate case. Receipt SHA256:
`947f54877bfc01d9ee8d0238337973a2e92770446a4a48895005f9d89be36868`.
Artifact `11645480738` retains that failed setup; it is not a qualified master RED.
Baseline planning writes its real source-specific probe cache. The corrected
setup populates and validates that probe through an owned baseline Server,
joins it, and then seals playback clones. Every existing clone immutability
assertion remains required. No production source has changed.
The Version 2 metadata fixture now uses the generated master writer with its
real initialization. It does not supply an actual source packet proof.

At `11733ff6`, both actual V1/V2 positive controls passed HEAD and the
13-request GET journey with 480 exact source frames and unchanged cache.
All 24 malformed-master cases executed; 23 failed. V2 missing stream tags
and duplicate renditions caused cache writes and 12 source encoders each.
Receipt SHA256: `f7e860598c35fa67c95f7df8b30c62036538bd2058c89c27c43391b01b229422`.
The pending-clock controls freeze metadata between master publication and
clock commit. They test GET/HEAD admission and committed HTTP delivery,
and reject a damaged present tag before preparation. They provide metadata
scheduling evidence, not actual source-clock or media acceptance.
A public old-format control removes only the bandwidth marker from genuine
Version 1 media, then compares baseline/candidate bytes and 480 source frames.
Pending owner-root and preflight-to-deletion races remain release blockers.

The qualified cross-version RED is preserved in artifact `11645044736`.
GitHub archive digest:
`6b6545583bf36470d0d8940d59e4010f40e5a25923ca71bfc9a11b4bbd41915f`.

## Pending admission and narrow master repair

Written-first head `5dd3ef8dd7a1bc4b4518ca22c7d9961396cd5b27` ran contracts before production changes.
Run 37994081759 failed only `TestCopiedAACPendingMalformedMasterRejectsWithoutPreparation`.
All four valid pending GET/HEAD controls passed, including certificate-first publication.
Artifact 11646706125 preserves that qualified failure.
Its API archive SHA256 is `22b4c930601c685fd93b66b85f59284c19c10c00ec233b0f9c03d376a301f307`.
Dependent public-media jobs skipped; the older absent-bandwidth-marker positive control remains unqualified.
Quality 37994081732 found only a gofumpt issue in the new fixture's job literal.
The fixture literal is expanded without changing assertions.

The candidate shares a bounded generated-master grammar across Version1 and Version2 readers.
It accepts the known older header without a bandwidth marker, pending its actual baseline control.
Present markers must equal the generated marker; unknown or duplicated tags and attributes reject.
One stream entry pairs immediately with its one allowed rendition URI.
Bound generations compare CODECS, dimensions and range against the same init bytes whose SHA matches the certificate.
Bandwidth remains a grammar-checked estimate; no later hydration recomputation changes historical masters.
Present Version2 masters receive read-only preflight before preparation.
Pending clocks receive structural checks without requiring certificate/timeline agreement during the certificate-first interval.
Existing source-policy and canonical-root checks remain; all leases end before preparation or network transfer.
Final bound certification remains mandatory for delivery.

This slice does not change missing-master handling, owner cancellation, cache deletion or lazy workers.
The preflight-to-preparation deletion race and pending-owner root binding remain release blockers.
Lazy/speculative Version1 compatibility, 14 inherited holds, and native50 retesting remain open.
No production, older-client, native or all50 acceptance follows from this candidate alone.

## Qualified complete-cache and master result

Head `2174aac2a10af0d0747893369056b0c63c9ae36c` completed proof 37994947510 and quality 37994947515 successfully.
All 39 complete adopted Version1 compatibility cases passed; no candidate preparation POST or cache mutation occurred.
Receipt SHA256: `c6ec2a6c8269fd3760da4461dc4591015c8e481f7ba06bbd28f713df48f64d24`.
Artifact 11647282448 API archive SHA256: `87fdadf42a8ac677db357d1584b1f9870f713045e9aa6529c15bd4e11702b6a5`.
All 24 malformed master cases rejected GET/HEAD with zero source encoders, unchanged caches and one correlated WARN each.
Four positive master controls passed 480 exact source frames each, including actual older absent-marker baseline and candidate.
Master receipt SHA256: `e365544fe57311b39f53bbf00c983931009c8be178471e6795aa49fe6e3bd197`.
Artifact 11646533261 API archive SHA256: `7a6d086716c4b3a29c569d6acc2c0bc77ac2888812a242075314c1620e2598bd`.
Five indexed AAC journeys, two interrupted unindexed cold cases and 18 missing-binding cases also passed.
The valid pending admission and malformed pending grammar controls passed.
Grammar-valid pending init mismatches, lazy continuation and ownership races remain unproved.
This proves the complete/master slice; full older-client, production and native acceptance remain false.
