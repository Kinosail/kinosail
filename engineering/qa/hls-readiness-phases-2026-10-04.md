# HLS readiness phase diagnostics

## Failure analysis before production edits

Baseline: fetched main `7ad46ad19`, after the unused HLS codec estimate removal.
Raya's historical production revision is `9d1b158e`.
Its reported audio-transcode attempt emitted `HLS transcode started`, then a
30-second playlist readiness timeout without a matching terminal event.
The start message precedes capacity acquisition and actual child launch.
It cannot distinguish an admission delay from input or publication delay.

The observation change must prevent these failures:

- A queued job is described as an already running encoder.
- A failed executable launch emits a process-start event.
- A running child without complete output is described as media ready.
- Input seek is confused with displayed resume time or segment numbering.
- Publication or hardware recovery produces duplicate first-readiness events.
- A request timeout loses the last known encoder phase.
- Logs expose media paths, recipe tokens, URLs, command arguments, credentials,
  playback-session identifiers, or arbitrary remote error text.
- New observation code changes scheduling, process cancellation, shared-cache
  ownership, playback policy, or readiness decisions.

Use fixed phase/outcome/mode/work-class values, bounded numeric timing and
capacity fields, and the existing validated request ID. Do not create a new
session identifier. A published master verifies the existing server publication
contract; it does not establish browser decoding or moving frames.

## Failure-first verification

Public HTTP regressions cover executable-launch failure, a launched encoder
waiting for output, full-capacity contention, effective input seek, successful
master publication, event order, and privacy. Controlled executable/probe
adapters are needed to force these lifecycle boundaries without slow media.
These checks are isolated public-interface tests, not decoded-media E2E proof.
They must fail on the baseline before production edits.

The existing generated-media public browser journey remains the real-media
acceptance boundary. Retain only allowlisted phase/numeric evidence from its
private server log, plus exact revision, command, environment and checksums.
Keep its raw logs, configuration, authentication and browser traces private.
Hosted checks are authoritative while local app builds are resource constrained.

## Independent diagnostic reproduction

The out-of-tree generated-media probe uses actual production playback/governor
package seams with generated H.264 High 1920x804 SDR and DTS core 5.1 audio.
With fresh output directories, zero-offset master publication took 76 ms and
three-second input seek took 51 ms. Holding all three permits prevented child
launch at a modeled 200 ms request deadline; release at 300 ms allowed launch
at 304 ms and publication at 376 ms. Artificial `-readrate 0.25` input launched
a child but produced no nonempty init before the 1.5-second job deadline.
All final governor counters returned to zero.

Receipt SHA256: `d20abd0cf3212747d9031f540d4ec248a976ebe767821d8db0b782a095d7a905`.
This is a scaled package-seam experiment. It is not a real HTTP timeout, Nox
workload, DTS-HD MA 7.1 conversion, or Safari playback reproduction.

Raya's cause remains unproven. Chrome's automation checkbox remains disabled,
and the latest bounded Nox state inspection returned AppleEvent timeout -1712.
No real-title playback test ran. Original-tab restoration remains unverified.
The diagnostic change does not resolve or claim to resolve Raya playback.

## Implementation and focused evidence

The pre-change HTTP checks failed in all three cases: failed executable launch
was called started, running output wait lacked phase evidence, and queued
admission was called a running child. Baseline log SHA256:
`20f9da4dd34fe673457c3bb4e686163b519e3997988f9c10d80a476a1cecb81e`.

The phase observer now emits queued, admission-wait, admitted/rejected, actual
process-start/start-failure, and first successful master-publication facts.
It records the actual input `-ss` offset separately from segment numbering.
Offsets beyond the bounded seven-day diagnostic range use -1, not a clipped
value. Request failures retain the last observed phase through a wrapped error
that preserves `errors.Is` and the existing public error behavior.

Process launch uses the same bounded stderr/redaction helper and explicit
`Start`/`Wait` equivalents of `Run`. Publication keeps its existing readiness
checks and writes; observation is once per job. Diagnostic ordering is serialized
per job and buffers an early publication observation until launch is observed.
No viewer identity, session identifier, device path or stream recipe is added.

All three repaired HTTP journeys and existing failure-privacy checks passed.
The affected playback package also passed. Source caps, diff checks, CI contracts
and workflow validation passed. Full hosted checks and real-media phase evidence
are still pending. The hosted phase extraction step uses the existing synthetic
browser journey without changing PR455's startup runner or browser fixtures.
It retains fixed enums and bounded numbers and strips request identifiers.
