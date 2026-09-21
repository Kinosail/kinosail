# Code scanning remediation — 2026-09-21

The initial GitHub inventory contained 25 open medium-severity CodeQL findings and no open secret-scanning alerts. Review each alert against its actual transport and output encoding; do not remove queries or add artificial sanitizers to silence a report.

| Alerts | Finding | Resolution and evidence |
| --- | --- | --- |
| #7 | Service-worker profile/logout message lacks an explicit sender check | Require an exact same origin and a window client with a same-origin URL; reject unknown fields, malformed profile/revision values, and conflicting logout payloads before identity side effects. |
| #5–6 | Missing origin checks in dedicated digest/storage workers | False positives: the handlers run only in dedicated Blob Workers. References stay private to their creator and there is no Window-message forwarding. Cross-origin Window messages cannot reach the implicit worker channel. |
| #16–20 | Request/panic log injection | False positives: both production processes install `slog.JSONHandler` in `appcli.Execute` before serving requests. Fixed keys and JSON-encoded values preserve record boundaries, including panic text, stack traces, quotes, CR/LF, and control characters. |
| #21–37 | Playback trace log injection | False positives: `ReadTrace` validates six string fields, ten numeric fields and one boolean before session/log side effects, and the same JSON handler encodes the structured values. |

The explicit classifications were independently reviewed against the current CodeQL models and production call paths. Each false-positive dismissal must retain this rationale; it is not a blanket rule suppression. Future findings continue to run through the same security-extended queries. The findings gate now rejects every open security-severity finding, including medium and low.

## Configuration repair

The maintained workflow successfully analyzed Go, JavaScript/TypeScript, and Python on main `20a92b20`. Retired default-setup Go and Swift analyses from `ae618325` still reported failed execution (IDs `1807599529` and `1807599489`). Default setup was already disabled; those failures do not describe the current Go scan.

Restore Actions and Swift in the maintained change-aware language selection. Swift requires a macOS runner and explicit builds of both iOS and tvOS targets. A native-only change must require Swift analysis and findings validation without selecting server containers. Only after successful replacement scans should the retired default configurations be removed, preserving their metadata and SARIF evidence. Do not delete active analyses or dismiss an execution failure as a code false positive.

The first maintained Swift attempt ([job 106213221287](https://github.com/Kinosail/kinosail/actions/runs/35560725398/job/106213221287)) exhausted 15 minutes while building the iOS simulator target, before tvOS or analysis. Its log shows both ARM64 and x86_64 SDK module generation. Follow GitHub's recommendation to scan one CPU architecture: a temporary Xcode configuration limits only the instrumented scan to ARM64 while retaining both iOS and tvOS builds. The ordinary client compilation gate retains its normal architecture settings. Allow 30 minutes for cold Swift extraction; other language ceilings remain 15. Record the successful replacement timing before claiming a speedup.

The ARM64-only attempt still spent over 20 minutes in iOS SDK/module extraction. Investigation found a supported-configuration gap: CodeQL 2.27's own autobuilder disables `COMPILATION_CACHE_ENABLE_CACHING`, `SWIFT_ENABLE_COMPILE_CACHE`, and `SWIFT_USE_INTEGRATED_DRIVER`. Upstream explains that caches can omit compiler invocations and the integrated driver introduces Xcode modules incompatible with the extractor. Apply those same settings to the manual scan, plus the autobuilder's unsigned-build settings. These overrides are confined to CodeQL; the ordinary native build remains unchanged. This is a correctness requirement for extraction, not a query suppression.

The supported settings completed [hosted Swift job 106220574103](https://github.com/Kinosail/kinosail/actions/runs/35563388011/job/106220574103) in 23m58s: both instrumented builds took 21m38s and analysis/upload took 1m24s. All 28 reported rules completed with zero findings, scan warnings, or unresolved AST nodes. This confirms extraction with the supported settings; it is not a controlled speedup measurement because earlier attempts never completed.

Container delivery previously stopped waiting for sibling CI after 20 minutes, shorter than the restored Swift scan's 30-minute ceiling. Allow 35 minutes for successful sibling CI and 40 for the encompassing job; publication still fails closed on failures or timeout. A fake-clock regression reproduces the premature rejection at 25 minutes and proves both eventual-success deployment-target emission and bounded timeout without deployment targets. This changes the maximum wait, not the time to publish once checks pass.

## Regression evidence

- `node --test packages/webassets/offline-identity.test.mjs`: 31 passing cases; malformed, foreign, missing, oversized, and conflicting messages cause no identity storage access, writes, or broadcasts.
- Player Playwright `worker-message-security.spec.ts` and the four existing service-worker contract/upgrade specs: 42 passes across Chromium, Firefox, and WebKit with retries disabled. Real service workers accept valid profile and logout messages after sender checks.
- Dedicated-worker security tests use real Blob Workers and cross-origin frames, with an in-memory filesystem to isolate channel security from browser OPFS availability. Actual OPFS checks also passed locally in Chromium/Firefox; local WebKit OPFS returned a transient filesystem error and is not claimed as verified by this test.
- `go test -race ./packages/playback ./packages/servertest` and focused request-log/trace tests in both app server packages pass. Injection payloads derive from a valid `sequence: 1` event; rejected traces produce neither session changes nor trace logs. Panic/request logs decode as exactly two JSON records with their original field values.
- Hosted replacement scans, final alert states, merge proof, and publication evidence belong in the delivery PR record. Local success alone does not certify GitHub's final alert state or a live production deployment.

## Sources

- [Go JSON handler implementation](https://go.dev/src/log/slog/json_handler.go) emits line-delimited JSON and encodes structured values.
- [CodeQL slog model](https://github.com/github/codeql/blob/main/go/ql/lib/ext/log.slog.model.yml) and [log-injection customizations](https://github.com/github/codeql/blob/main/go/ql/lib/semmle/go/security/LogInjectionCustomizations.qll) model generic logging sinks without conditioning on the installed JSON handler.
- [HTML dedicated-worker communication](https://html.spec.whatwg.org/multipage/workers.html#communicating-with-a-dedicated-worker) describes the creator/worker implicit channel.
- [GitHub stale-configuration guidance](https://docs.github.com/en/code-security/how-tos/manage-security-alerts/manage-code-scanning-alerts/resolve-alerts#removing-stale-configurations-and-alerts-from-a-branch) distinguishes retiring old configurations from fixing or dismissing code findings.

- [GitHub Swift build guidance](https://docs.github.com/en/code-security/reference/code-scanning/codeql/build-options-for-compiled-languages#customizing-swift-compilation-in-a-codeql-analysis-workflow) recommends one architecture for analysis. The local `xcodebuild(1)` manual documents `XCODE_XCCONFIG_FILE` as an override applied to every built target.

- [CodeQL 2.27 Swift autobuilder](https://github.com/github/codeql/blob/codeql-cli/v2.27.0/swift/swift-autobuilder/BuildRunner.cpp#L80-L84) and [upstream rationale](https://github.com/github/codeql/commit/662c7b08e8abafda2d3b4d2f75a407ebb481701b) define the compatible compiler settings.
