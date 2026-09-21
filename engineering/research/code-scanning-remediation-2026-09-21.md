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
