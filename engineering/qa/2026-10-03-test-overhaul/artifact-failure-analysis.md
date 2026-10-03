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
  describe a dirty run as proof of an unchanged SHA. Compare source status,
  revision, and patch hash after the command; reject changes during the run.
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

## Upload completeness failure found before the uploader repair

Run37150700625 preserved each visible browser evidence file and its checksum,
but actions/upload-artifact omitted Playwright `.last-run.json` by default.
The checksum list consequently names a member absent from the downloaded ZIP.
A successful command and a ZIP digest alone cannot prove upload completeness.
Enable hidden-file upload only for the disposable E2E evidence directory.
The directory must contain synthetic test evidence, never real session state.
Verify every checksum member after downloading the final uploaded archive.
The earlier receipt records missing members explicitly and does not claim a
complete archive. Browser result JSON and screenshots remained hash-verifiable.

## Populated settings journey preparation, before implementation

A smoke tag can select a test that still skips without KINOSAIL_TEST_INSTANCE.
Fresh-install onboarding and prepared-Owner settings journeys require distinct
Server states; do not set the flag across parallel fresh-install tests.
After the existing suite succeeds, start fresh state and create an MFA Owner
through the actual API. Abort on setup, confirmation or onboarding failure.
Require supported loopback HTTP; do not bypass HTTPS verification.
Run the existing two search journeys serially with the populated flag and
synthetic Owner credentials. Never print the Owner token or TOTP secret.
Use a separate artifact subdirectory so reporters cannot overwrite the first
run. Record exact command, setup outcome and source/input hashes. Preserve
failures and retain the isolated search guard until actual passing results.

Independent source review found that Playwright environment overrides take
precedence over configured reporter paths. A child run inheriting the outer
HTML output path can erase the earlier report. Set both HTML and JSON child
output overrides inside the prepared run directory. Require setup HTTP201,
confirmation HTTP200 with enabled:true, and finish HTTP303 with Location:/;
do not follow redirects into a login page and claim completed setup.
