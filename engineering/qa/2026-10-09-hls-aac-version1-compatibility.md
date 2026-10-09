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
- Missing/wrong binding, missing/mixed clock, corrupt timeline, master, init
  or first media must reject without encoders or cache/source mutation.
- Immediate, idle and joined snapshots must preserve every cache file's inode,
  size, modification time and SHA256. Baseline response restoration is separate.
- Each owned Server and child process must settle under the existing bounded
  process/resource checks. Receipts include exact revision, tree and script hashes.

Before full compatibility acceptance, add actual missing-zero, interior and final
Version1 continuation; speculative adoption; policy/source/generation replacement;
concurrent Version2 migration; and the actual supported client request journey.
Do not claim these are covered by the complete-cache test.

Retain all eighteen Version2 missing-binding controls. Retain the five Version2
media journeys with every one of 942 AAC packets, packet duration/adjacency,
480 decoded source frames and the full 960008-sample PCM/EOF oracle.
No assertion, security, scanner or required gate may be weakened.
The original all50 baseline failures and fourteen held fingerprints remain open.
