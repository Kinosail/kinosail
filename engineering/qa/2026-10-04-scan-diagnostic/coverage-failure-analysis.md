# Missing deep coverage detail

Matched main and audio-queue candidate deep runs both fail the existing Player
coverage requirement: 87.3% against 89%. Selected package percentages also
match: server 87.0%, command 97.9%, backup and configuration 100%. This is
actual gate evidence. It does not identify the uncovered public failure paths.

Retain the exact existing coverage profile and its function summary only when
manual diagnostics are explicitly enabled. Preserve the original deep test
command, selected mode, coverage denominator, threshold and failure outcome.
Do not upload the whole verification directory or raw test/process logs.

Failure risks before implementation:

- Changing test selection or threshold could hide the uncovered statements.
- An evidence step must not turn the failed Go test job green.
- A broad artifact path can include private fixture or process evidence.
- Normal PR/push runs must not accidentally enable this optional evidence.
- An absent or partial profile does not prove a completed passing test run.

Static workflow controls protect invocation and artifact boundaries that
product E2E cannot observe. Hosted execution will separately verify the real
profile, exact revision and job outcome; no local Go process is authorized.
