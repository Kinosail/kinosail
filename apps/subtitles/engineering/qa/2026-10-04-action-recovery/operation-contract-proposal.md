# R06 operation contract proposal

This is a review proposal, not implemented behavior. The public Save/Restore controls passed. All eight isolated browser cases reproduced the intended deadline failures at source `a046ec85`; the earlier timed-out partial aggregate remains preserved and excluded. Production work waits for this contract's independent review.

## Why preparation must precede a write

A client-generated ID alone cannot prevent replay after a server forgets it. The server must issue and record an operation before the mutation. An unknown or expired ID must always be rejected before side effects; it must never implicitly start another action.

## Additive public interface

1. `POST /api/v1/subtitle-operations` accepts strict JSON `{ "action": "apply|restore|fetch|replacement|maintain|draft|audio", "item": "<16 lowercase hex characters>" }`. `item` is omitted only for `maintain`. The server checks Owner access and item visibility, creates a random 32-byte hex operation ID, persists the prepared receipt, then returns `201` with the ID and status URL. It accepts no query parameters and at most 1 KiB of JSON. Starting an operation here does not change a subtitle.
2. Existing mutation routes retain their current JSON and authorization contracts. A new client supplies `X-Kinosail-Operation: <ID>`. Only an existing prepared operation for the same Owner, action, and item may start. Its first body digest is bound atomically before the public application operation executes. Unknown, expired, foreign, mismatched, or malformed IDs are rejected before execution. Clients without this header keep their existing public contract.
3. `GET /api/v1/subtitle-operations/{operation}` is Owner-only, accepts no query parameters, and returns bounded metadata: ID, action, item, state, safe outcome, and HTTP status when completed. It contains no raw paths, submitted subtitle text, credentials, or provider response. An operation from another Owner has the same unavailable response as an unknown ID. Status responses use `private, no-store`.

Opt-in execution returns `202` with a receipt rather than keeping a browser write request open. The client polls status with a bounded GET. On completion, it reads the installed subtitle and History or reloads the library before reporting success. It does not obtain large subtitle documents from receipt storage.

## State, replay, and persistence

| State | Meaning | Allowed next step |
| --- | --- | --- |
| prepared | Recorded; no mutation has started. | The one bound mutation may start before expiry. |
| running | The operation started and may still change data. | Read-only status, inspection, history, and navigation. No fresh write for the same unresolved scope. |
| completed | The application operation returned. | Reconcile public data before another reviewed action. Failed HTTP outcomes do not imply that no partial write occurred. |
| unknown | Restart or an uncertain completion prevents proof. | Read public subtitle/history data and explain uncertainty. Never replay this receipt or label it canceled. |
| unavailable | ID missing, expired, foreign, or inaccessible. | Keep the outcome unknown. Never synthesize a new write from Retry. |

The server persists receipt metadata atomically before starting a write and after completion. A failure to persist preparation prevents execution. A failure to persist completion leaves an uncertain receipt, even if the write itself completed.

On restart, recorded `running` entries become `unknown`; they are never resumed. Prepared entries remain prepared only within their deadline. Completed receipts retain their factual outcome. Expired entries may be removed from internal receipt storage, but the mutation adapter still rejects their IDs. Creating a new receipt is an explicit new action after public data review, not a replay mechanism.

Proposed bounds for review: 64 metadata receipts, a 96 KiB metadata file, five minutes to activate a prepared receipt, and 30 minutes of completed receipt retention. Running work remains tracked until it returns or reaches its operation deadline; it is not evicted to make capacity. Full capacity returns a safe `503` before execution. Receipt files and any audio results are owner-readable server data. Existing task artifacts are not cleanup targets.

## Work and audio limits

Only one running subtitle mutation executes at a time. Concurrent activation of a different mutation receipt is rejected before application work. The same active/completed ID returns its existing receipt without invoking the mutation again; a different submitted body for that ID is a conflict. Existing fingerprint and recovery-file guards remain in force.

Unknown outcomes block the client's automatic retry and write controls while it reconciles. A fresh action requires explicit review of the current subtitle and History. It must obtain a new prepared receipt and be presented as a new action, never as completion or cancellation of the old one. After an actual restart, prior-process work cannot still run; a prior unknown receipt does not permanently block the new Server. Tests must close/cancel the first Server lifecycle before constructing the restarted Server, rather than running two installations against one state directory.

Audio analysis uses one background job at a time and the existing serial audio-analysis seam. A second activation receives an explicit busy outcome rather than starting another process or waiting indefinitely. Audio work retains its existing 20-minute execution bound. Its waveform and speech-probability result is capped at 128 KiB, with at most two retained results and the same bounded retention lifetime. No audio, path, transcript, or credential is included in receipt metadata.

Normal browser reads have a 15-second deadline. The initiating write and each status poll have a 30-second or shorter client deadline. Server mutation deadlines must account for provider batches and audio synchronization; the client deadline does not cancel or roll back a started mutation. Automatic-sync preview must use the bounded audio job before previewing, preserving the current audio cache and fingerprint checks. The job context must follow the existing `Config.Lifecycle` shutdown context, with its operation deadline; browser cancellation must not erase its status. The exact server limits and shutdown hook must be verified before implementation; no existing deadline is reduced speculatively.

Draft start/cancel must reconcile through the existing public draft status. A stalled draft GET must eventually throw into the existing bounded backoff. Language changes and page lifecycle events invalidate stale read results and abort owned browser requests without treating that abort as write cancellation.

## Required proof before claiming completion

- Real public Save/Restore: lost response, one executed write, protected recovery bytes, current fingerprint, and matching history.
- Same-ID replay and different-body conflict: zero additional writes or history events.
- Prepared, unknown, expired, foreign, malformed, and unavailable IDs: rejection before any side effects; Owner and CSRF controls remain effective.
- Restart with a running receipt: unknown outcome, no resumption, no replay, and preserved current/recovery data.
- Receipt capacity and expiration: bounded storage and no active eviction. An expired operation cannot be reactivated.
- Two concurrent audio activations: only one local stand-in process starts, with a visible busy result for the other; no encoder, provider, or production data in QA.
- Interrupted status reads and language/navigation races: safe editing/navigation restored, stale results ignored, submitted edits preserved, and no automatic POST retries.
- Structured lifecycle logs: safe operation/action/request correlation and outcome class, without tokens, payloads, paths, or remote responses.

This proposal still needs independent review of persistence/restart behavior, concurrency, and the asynchronous contract before implementation. It does not authorize unrelated settings, deployment, or user-data deletion.
