# Q09: Pause a device download without removing it

Q09 is an approved feature opportunity. Baseline absence of Pause is expected
behavior, not evidence of a household incident. Frozen R10 handoff: 9b59524d.

## Public contract

The browser owns a device transfer. The Server prepares a private file through
`POST /api/v1/items/{id}/downloads`; manifest and byte ranges already use
`GET /api/v1/downloads/{id}` and `/file`. Pausing the browser cancels its active
GET without removing the Server job or changing another tab's transfer owner.
Resume revalidates the manifest and each saved block before using it. No new
server-side pause state is required for a device-local operation.

The device action must expose keyboard-accessible Pause during pending work,
then Resume after the request and storage work settle. Status must explain that
the page must remain open while transferring. Remove stays a separate action;
no Remove form or DELETE will be exercised in this QA.

## Failure modes before tests or product edits

- Pause is absent or disabled during manifest, lock wait, or byte transfer.
- Pause falsely announces completion before pending fetch, OPFS writes or job
  locks settle. A second Resume races the same owner or creates duplicate work.
- A partial range becomes committed metadata; a partially written OPFS block
  is trusted without its digest. Verified prior blocks are deleted or refetched.
- Resume trusts changed profile/item/manifest identity or corrupt stored bytes.
- Resume restarts at offset zero despite an intact first verified block.
- A paused record is marked ready without full-file digest validation.
- Pause cancels another tab's owner, broadcasts a destructive cancel, or removes
  the prepared Server file. A queued tab must retain its own ownership.
- Server refresh replaces an active or pausing button and loses its handler or
  keyboard focus. Paused/resumed controls are stale after a refresh or reload.
- Offline Resume loses verified blocks or stays disabled after failure.
- Navigation leaves a lock held, loses progress, or resumes automatically against
  the viewer's intent. Page lifecycle must retain current interruption behavior.
- Status/control labels disagree; page-open guidance is absent; controls overflow
  a phone viewport or show Play offline before the completed digest check.
- Shared bundle composition changes but immutable delivery tokens stay stale.

## Primary proof and justified isolated boundary

Prepare a real Server runner using a deterministic fictional 16 MiB + 31 byte
file, original-quality public preparation, native service worker, native Web
Locks, actual OPFS/IndexedDB, Go-rendered Downloads and the actual composed
downloads asset. A test-only outer HTTP peer holds the second native-size range
after headers and a prefix; Resume delegates normal ranges to the Go handler.
This establishes transport cancellation, verified-block retention, continuation
ranges, final integrity and unchanged Server preparation without an encoder.

Before the scheduled Go build, use the same browser acceptance journey against a
real localhost Node HTTP peer. It serves the unchanged production browser/SW
sources, deterministic manifest and range bytes with independent SHA-256 digests.
The rendered shell and ancillary cached assets are fixture material. This is an
isolated native-browser/real-HTTP control, not populated Server proof. Its distinct
purpose is repeatable held transport ordering and failure controls without heavy
builds. No production test-only export or hook is introduced.

Use real 8 MiB chunks; do not rewrite the production chunk size. Force only the
documented IndexedDB fallback by disabling OPFS in its dedicated case. Native
OPFS cases use the real worker and storage APIs. Inspect saved metadata and bytes
independently through IndexedDB/OPFS; assertions must not invoke private transfer
helpers. The data is fictional and each browser context is disposable.

Existing isolated resilience tests cover orphaned writes and integrity rejection;
do not duplicate or weaken them. Q09 adds the public Pause/Resume interaction and
independent retention/range/lock observations that those tests cannot establish.

Root owns serial Go/browser scheduling, required container/hosted gates and final
integration. Native devices, real user data, deployment and actual Remove QA are
excluded. Root will maintain the campaign tracker and Library deliverable.

## Separate profile-generation control before production repair

Source inspection shows every new Server page selects a new offline identity
revision even for the same Viewer Profile. A storage event currently aborts all
local transfer owners. This may prevent the two-tab Pause/Resume contract before
Pause is used. It is a hypothesis, not yet a runtime-confirmed defect.

The separate ownership acceptance case opens another native page for the same
profile, requires the first transfer to remain active with the same owner, then
uses explicit Pause and resumes from the second tab without refetching the first
block. An isolated companion changes the supplied profile and requires old-owner
cancellation and retained profile-bound data. No profile/account settings are
changed. The native navigation case separately requires interruption and explicit
Resume after a genuine navigation; it does not claim BFCache admission.

If confirmed, scope the repair to transfer-owner profile comparison at the
existing offline-profile event. Do not change identity revision/security rules,
offline playback, Server profile permissions or the prepared download contract.

## Additional phone hit-target control before any layout repair

The successful Go-rendered captures show a fixed mobile navigation band crossing
later flow content at the captured scroll position. The existing ready Play link
can intersect that band. The original tests prove mobile keyboard Pause/Resume
and desktop pointer Pause/Resume, but only visibility of the ready Play link.
Visibility alone can miss an overlay intercepting pointer input.

Prepare a separate real Go/browser control at 390 pixels. Independently record
native element rectangles, viewport/scroll, focus and `elementFromPoint` before
and after explicit centered scrolling. Pointer-click only the fictional Pause,
Resume and Play controls; verify verified bytes, ranges and zero Remove requests.
Require pointer and keyboard Play navigation to the selected fictional offline
job. The deterministic bytes are not playable; this is navigation and hit-testing
proof, not media decoding. Do not exercise Save file or Remove.

Preserve the original eight-case Go run and its fixtures. The additional control
uses a separate opt-in Go runner mode and distinct outputs; no production or CSS
change precedes its execution. Read `ready-layout-boundary.json` for byte-identical
Downloads template constants and shared CSS against the frozen baseline. New
labels/focus can affect scroll position; source equality does not establish every
historical rendered hit target.

The separate actual Go phone control at `7acb6d9a` completed, but its first helper
sampled geometry immediately after `scrollIntoView` and focus. The shipped html
has `scroll-behavior:smooth`. The fresh-page Play sample hit the navigation at
scroll zero; the later failure PNG shows the focused link clear of navigation.
Pointer Pause/Resume and first Play navigation passed, but keyboard activation
was not reached. Classify this as an unsettled-scroll harness boundary, not
proof of a persistent product defect. Preserve the failed raw report and trace.
Before a repeat, add a bounded geometry-stability observation after scrolling
and focus. Retain every pointer, focus, navigation and integrity assertion;
change no CSS, markup or production behavior.
