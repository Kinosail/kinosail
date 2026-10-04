# R06: stalled actions and uncertain writes

The approved R06 audit requires bounded subtitle requests and safe recovery after a lost response. This batch starts with existing public Save and Restore routes. Runtime reproduction is pending. Source inspection alone does not confirm the user impact.

## Failures to reproduce before implementation

| Failure | Observable requirement | Data-integrity constraint |
| --- | --- | --- |
| Save response never settles | The editor leaves its global pending state within a bounded wait. Safe editing and navigation become available. | One POST at most. The installed subtitle and recovery copy are inspected before another write becomes available. |
| Restore response is lost after completion | Read-only inspection and history identify the current subtitle. The user can leave or inspect the result. | Never repeat Restore automatically. Restore exchanges the current and recovery files, so repetition can undo the completed operation. |
| A write remains in progress after the browser deadline | The page identifies the outcome as unknown or still pending. | An unchanged fingerprint or history is insufficient proof that the write did not happen. No fresh write or blind Retry is enabled by that observation alone. |
| Dashboard action never settles | Navigation and read-only refresh remain usable; the pending indicator ends or becomes explicit status. | An uncertain action must not be replayed by refresh, polling, or a second click. |
| Read-only inspection, preview, or draft polling stalls | Each read has a deadline and a recoverable error state. | Preserve editor inputs. Read retries cannot repeat mutations. |
| Audio analysis needs minutes | Analysis exposes bounded progress/status, rather than an indefinite foreground request. | Bounded server work, storage, and result lifetime. No extra provider, encoder, or deployment work in QA. |
| Navigation or language changes during recovery | Stale responses cannot overwrite the new language or page state. | Keep existing request-generation, draft, focus, and input guards. |
| Invalid operation or status inputs | Owner authorization, CSRF, strict JSON/query limits, and safe errors remain effective. | Reject before side effects. No raw paths, credentials, or provider responses in status or logs. |

## Test boundaries

The initial Go controls invoke the real Server through its public HTTP handler and use disposable synthetic sidecars. They establish completed Save/Restore, public fingerprints, history, and exact recovery bytes. Their synthetic media is not playable; they are public API controls, not media playback evidence.

The initial browser regressions consume that Server's rendered HTML, served scripts, and captured API outputs. A browser transport boundary withholds a mutation response while exposing either the completed or unchanged public read result. This is isolated renderer/transport evidence. It does not prove that a real network response was lost after a real write.

A later native Server fixture must execute the approved mutation on its disposable data before withholding the response. It must record the single mutation, current/recovery hashes, public reads, request guards, and final fixture integrity. The approved fictional clip may be copied and hashed; no encoder or real library is permitted.

## Design decision still open

Existing fingerprints reject stale Save, but do not correlate a Restore or maintenance request with its result. Read reconciliation can report changed data; unchanged data cannot prove completion. A bounded Go receipt/status contract may be needed to distinguish running, completed, and unavailable outcomes safely. No new API contract or production behavior has been added at this preparation checkpoint.

R16 preparation is excluded from this batch and remains byte-exact. R07 sources and receipts remain frozen at their reviewed checkpoint.
