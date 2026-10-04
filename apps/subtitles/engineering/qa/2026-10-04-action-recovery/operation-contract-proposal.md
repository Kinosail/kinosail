# R06 operation contract proposal

This is a review proposal, not implemented behavior. The public Save/Restore controls passed. All eight isolated browser cases reproduced the intended deadline failures at source `a046ec85`; the earlier timed-out partial aggregate remains preserved and excluded. Production work waits for this contract's independent review.

## Why preparation must precede a write

A client-generated ID alone cannot prevent replay after a server forgets it. The server must issue and record an operation before the mutation. An unknown or expired ID must always be rejected before side effects; it must never implicitly start another action.

## Additive public interface

1. `POST /api/v1/subtitle-operations` accepts strict JSON `{ "action": "apply|restore|fetch|replacement|maintain|fetch-wanted|audio", "item": "<16 lowercase hex characters>" }`. `item` is omitted only for `maintain` and `fetch-wanted`. The server checks Owner access and item visibility, creates a random 32-byte hex operation ID, persists the prepared receipt, then returns `201` with the ID and status URL. It accepts no query parameters and at most 1 KiB of JSON. Duplicate and unknown fields are invalid. Starting an operation here does not change a subtitle.
2. Existing mutation routes retain their current JSON and authorization contracts. A new client supplies `X-Kinosail-Operation: <ID>`. Only an existing prepared operation for the same Owner, action, and item may start. Its first body digest is bound atomically before the public application operation executes. Unknown, expired, foreign, mismatched, or malformed IDs are rejected before execution. Clients without this header keep their existing public contract.
3. `GET /api/v1/subtitle-operations/{operation}` is Owner-only, accepts no query parameters, and returns bounded metadata: ID, action, item, state, safe outcome, and HTTP status when completed. It contains no raw paths, submitted subtitle text, credentials, or provider response. An operation from another Owner has the same unavailable response as an unknown ID. Status responses use `private, no-store`.

Opt-in execution returns `202` with a receipt rather than keeping a browser write request open. The client polls status with a bounded GET. On completion, it reads the installed subtitle and History or reloads the library before reporting success. It does not obtain large subtitle documents from receipt storage.

### Route and compatibility mapping

| Receipt action | Existing activation route | Receipt completion status |
| --- | --- | --- |
| apply | `POST /api/v1/subtitle-library/{id}/apply` | Existing Save outcome, normally `200`. |
| restore | `POST /api/v1/subtitle-library/{id}/restore` | Existing Restore outcome, normally `204`. |
| fetch | `POST /api/v1/subtitle-library/{id}/fetch` | Existing Fetch outcome, normally `201`. |
| replacement | `POST /api/v1/subtitle-library/{id}/replacement` | Existing replacement outcome, normally `204`. |
| maintain | `POST /api/v1/subtitle-library/maintain` | Existing batch outcome, normally `200`. |
| fetch-wanted | `POST /api/v1/subtitle-library/fetch-wanted` | Existing batch outcome, normally `200`. |
| audio | `POST /api/v1/subtitle-library/{id}/audio` | Existing bounded audio result, normally `200`. |

An absent operation header retains the route's exact JSON, query, authorization, content type, status, and response body contract. An explicitly supplied empty, duplicate, uppercase, or oversized header is invalid and returns `400` before application work. Unknown, expired, or foreign operation IDs return `404`; mismatched action/item/body returns `409`. These responses do not activate or consume a prepared receipt. The opt-in adapter uses the existing application operations and their validation; it does not accept new body fields or broaden item visibility. The asynchronous receipt reports the factual application status, including errors after partial writes. It never converts a failed outcome into a success.

Draft start/cancel do not use this operation header. They retain the existing bounded draft job, its ID, its start/cancel input, and its public draft status. Browser deadlines and recovery use that existing correlation. A coarse `draft` receipt cannot authorize start and cancel interchangeably.

## State, replay, and persistence

| State | Meaning | Allowed next step |
| --- | --- | --- |
| prepared | Recorded; no mutation has started. | The one bound mutation may start before expiry. |
| running | The operation started and may still change data. | Read-only status, inspection, history, and navigation. No fresh write for the same unresolved scope. |
| completed | The application operation returned. | Reconcile public data before another reviewed action. Failed HTTP outcomes do not imply that no partial write occurred. |
| unknown | Restart or an uncertain completion prevents proof. | Read public subtitle/history data and explain uncertainty. Never replay this receipt or label it canceled. |
| unavailable | ID missing, expired, foreign, or inaccessible. | Keep the outcome unknown. Never synthesize a new write from Retry. |

The server persists receipt metadata atomically before preparation succeeds, before activation launches work, and after completion. The exact submitted body digest and `running` transition must be durable before worker launch. A failed activation persistence executes no application work and does not silently return to a reusable prepared state. The in-memory receipt becomes `unknown`, and the previous process's receipt cannot reactivate. A failure to persist completion also leaves an uncertain receipt, even if the write itself completed. Admission remains held while that work is active.

On restart, recorded `running` entries become `unknown`; they are never resumed. Prepared entries remain prepared only within their deadline. Completed receipts retain their factual outcome. Expired entries may be removed from internal receipt storage, but the mutation adapter still rejects their IDs. Creating a new receipt is an explicit new action after public data review, not a replay mechanism.

Proposed bounds for review: 64 metadata receipts, a 96 KiB metadata file, five minutes to activate a prepared receipt, and 30 minutes of completed receipt retention. A running receipt is never evicted to make capacity. Full capacity returns a safe `503` before execution. Receipt files and any audio results are owner-readable server data. Existing task artifacts are not cleanup targets.

A deadline or lifecycle cancellation requests that work stop; it does not prove settlement. Keep the running admission until the application operation returns and its child process has completed `Wait`. While cancellation is pending, status remains `running` with a safe cancellation-requested indication, and other activation remains busy. Only actual settlement permits admission to reopen. A completion-persistence failure may produce `unknown`, but it cannot release admission while the worker or child process remains active.

## Work and audio limits

Only one opt-in subtitle mutation or audio receipt executes at a time. Concurrent activation of a different receipt returns `409` before application work and leaves the second receipt prepared. The same active/completed ID returns its existing receipt without invoking the mutation again; a different submitted body for that ID is a conflict. Existing fingerprint and recovery-file guards remain in force.

Legacy operations share admission without losing their existing concurrency behavior: ordinary legacy requests may run together under their existing sidecar and audio guards, and are counted until they settle. Exclusive receipt activation is busy while any legacy mutation or audio work is admitted. While an exclusive receipt job is active, a new legacy mutation/audio request returns a safe `409` before application work. Read-only inspection, export, history, and navigation remain available. Automatic maintenance defers its batch while exclusive work is active. Automatic-sync preview and maintenance use the existing serial audio-analysis seam and participate in admission. The existing concurrent legacy Fetch contract (two searches, one created sidecar, one conflict) remains protected.

Unknown outcomes block the client's automatic retry and write controls while it reconciles. A fresh action requires explicit review of the current subtitle and History. It must obtain a new prepared receipt and be presented as a new action, never as completion or cancellation of the old one. After an actual restart, prior-process work cannot still run; a prior unknown receipt does not permanently block the new Server. Tests must close/cancel the first Server lifecycle before constructing the restarted Server, rather than running two installations against one state directory.

Audio analysis uses one background job at a time and the existing serial audio-analysis seam. A second activation receives an explicit busy outcome rather than starting another process or waiting indefinitely. A legacy analysis already holding or awaiting that seam also prevents exclusive receipt activation. Audio work retains its existing 20-minute execution bound. Its waveform and speech-probability result is capped at 128 KiB, with at most two retained results and the same bounded retention lifetime. No audio, path, transcript, or credential is included in receipt metadata.

Normal browser reads have a 15-second deadline. The initiating write and each status poll have a 30-second or shorter client deadline. Server mutation deadlines must account for provider batches and audio synchronization; the client deadline does not cancel or roll back a started mutation. Automatic-sync preview must use the bounded audio job before previewing, preserving the current audio cache and fingerprint checks. The job context must follow the existing `Config.Lifecycle` shutdown context, with its operation deadline; browser cancellation must not erase its status. The exact server limits and shutdown hook must be verified before implementation; no existing deadline is reduced speculatively.

Draft start/cancel must reconcile through the existing public draft status. A stalled draft GET must eventually throw into the existing bounded backoff. Language changes and page lifecycle events invalidate stale read results and abort owned browser requests without treating that abort as write cancellation.

## Required proof before claiming completion

- Real public Save/Restore: lost response, one executed write, protected recovery bytes, current fingerprint, and matching history.
- Same-ID replay and different-body conflict: zero additional writes or history events.
- Prepared, unknown, expired, foreign, malformed, and unavailable IDs: rejection before any side effects; Owner and CSRF controls remain effective.
- Restart with a running receipt: close/cancel the first Server and prove its child process stopped before constructing the next Server; then prove unknown outcome, no resumption, no replay, and preserved current/recovery data.
- Receipt capacity and expiration: bounded storage and no active eviction. An expired operation cannot be reactivated.
- Failed activation persistence: zero application/provider/process work and no reactivation of the uncertain receipt.
- Two concurrent audio activations and a legacy analysis: only one local stand-in process starts, with a visible busy result for the other; no encoder, provider, or production data in QA.
- A local provider/process that delays acknowledging cancellation: second activation remains busy until real application/process settlement, including when completion persistence fails. Request cancellation alone is not a passing stop assertion.
- Interrupted status reads and language/navigation races: safe editing/navigation restored, stale results ignored, submitted edits preserved, and no automatic POST retries.
- Structured lifecycle logs: safe operation/action/request correlation and outcome class, without tokens, payloads, paths, or remote responses.

This proposal still needs independent review of persistence/restart behavior, concurrency, and the asynchronous contract before implementation. It does not authorize unrelated settings, deployment, or user-data deletion.
