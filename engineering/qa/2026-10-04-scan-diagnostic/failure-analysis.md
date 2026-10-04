# Full-history scan visibility

Manual deep CI on the reviewed audio-queue head scans all fetched history with
gitleaks v8.30.1. It reports four findings without individual metadata. The
earlier PR check scans its changed-commit range. These are separate scopes.

The diagnostic must keep the same version, rules, ignore file and all-history
coverage. It must return the scanner's failure when findings exist. Only
rule, repository-relative file, commit and line can enter the shared finding
projection. The raw redacted report stays in a private temporary directory
and is removed after projection. Scanner output is discarded. Neither can
enter uploaded artifacts.

Failure paths to verify before implementation:

- A source report can contain private match, secret, author or fingerprint
  fields. The shared projection must exclude them.
- Malformed metadata can contain a control character, invalid revision,
  traversal path or invalid line. Reject it without echoing its content.
- Changing diagnostic flags could narrow coverage or omit redaction. Verify
  the actual invoked argument vector.
- Tool failure or timeout cannot create a successful projection. Preserve a
  nonzero result and upload no raw report.
- A finding must retain the scanner failure. The diagnostic cannot turn a
  failed security gate green.
- A failed pinned scanner install must not use an unrelated runner binary.
  Diagnostic execution and upload require that install to succeed.

These isolated launcher controls cover a privacy and invocation gap that the
existing repository E2E journeys cannot exercise. They use only fictional
findings and a fake executable. No local heavyweight scan is authorized.

The diagnostic is opt-in on the existing manual CI workflow. It runs after
the unchanged secret gate, using the same installed version and repository
configuration. It adds no workflow file, ignore entry or detection change.
The ordinary required job remains failed when the original gate fails.

Test-first observations: the original four controls failed with the helper
absent. After extending privacy/exit/timeout/workflow coverage, eight controls
reported two failures and sixteen subtest errors with no implementation.
The no-report tool-error case initially passed because the script was absent;
that initial result did not prove the implemented error path.
