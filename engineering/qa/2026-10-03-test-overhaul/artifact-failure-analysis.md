# E2E artifact failure analysis

Written before implementing the run recorder. This recorder wraps real commands;
it does not add isolated tests or replace any required gate.

- A command can fail, be missing, or terminate by signal. Preserve its result,
  console log, and available browser evidence; return failure to the caller.
- An artifact write or checksum can fail. Fail the job instead of claiming a
  verifiable run. Upload available evidence with `always()`.
- A rerun can mix stale evidence with new results. Require a new empty output
  directory and direct browser output into that directory.
- Source can differ from the named revision. Record the checked-out SHA and
  tracked/untracked source status, plus the patch checksum when dirty. Do not
  describe a dirty run as proof of an unchanged SHA.
- A container can differ from the checkout. Record image ID and its revision
  label separately; CI builds the tested image from the checked-out source.
- A run can select only smoke journeys, skip tests, or use response fixtures.
  Record selection variables and the full console result. The audit names the
  actual remaining assertions and distinguishes fixtures from live server paths.
- A toolchain or environment can change. Record OS/architecture, tool versions,
  lockfile hashes, and only a fixed set of safe test-selection variables.
- Logs and browser traces can include credentials. Use disposable synthetic
  data only. Never dump the process environment, production data, or auth state.
  Keep artifacts private under the existing repository permissions.
- Evidence can change after capture. Hash all regular evidence files, then
  write a manifest and hash it in `SHA256SUMS`. Check with `sha256sum -c
  SHA256SUMS` on Linux or `shasum -a 256 -c SHA256SUMS` on macOS.
- A hard runner kill or machine failure can prevent finalization. The uploader
  retains partial files; missing manifest/checksums means incomplete proof.

Validation uses the real repository checks, actual baseline/final Go suites,
and the populated browser workflow. Command failures must stay failures.
