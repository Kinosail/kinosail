# Dependency decisions implemented

Owner: the maintainers of each consuming app; shared tooling belongs to monorepo maintainers. Review date: 2026-09-06. Preserve existing APIs, native TV support, authentication protocols, media features, and quality thresholds.

| Component | Decision and resulting contract |
| --- | --- |
| YAML | Use the maintained `go.yaml.in/yaml/v3` continuation. Configuration and Dashboard imports reject additional documents as well as malformed, oversized, duplicate, and unknown inputs before writes. |
| DNS-SD | Use `github.com/libp2p/zeroconf/v2` with an updated DNS parser. Validate the complete advertisement before opening sockets. The initially considered `brutella/dnssd` lacks a public close operation for its initial-probe failure path; its implementation was rejected after lifecycle review. |
| govad | Keep the immutable commit. Independent reconstruction matches its embedded Silero weights byte for byte; official ONNX inference matches 596 reference/synthetic frames within 0.000002. See [source/model review](govad.md). |
| Go metrics | Own the narrow AST/counting and coverage calculations in `scripts/quality/metrics`; standard library only. Keep Halstead difficulty below 80 and CRAP below 25. Reject missing, malformed, and stale coverage. |
| Native toolchain | Remove unconfigured SonarJS and pin the resolved TV runtime exactly. Retain ESLint 9 while the Expo React lint plugin excludes ESLint 10 from its supported peer range. Keep Stryker and resolve its `qs` dependency through the scoped workspace override. |
| Executable artifacts | Enforce reviewed govad source/model and locally served HLS/HTMX hashes. Replace download-wrapper actions with checksum-verified ShellCheck/actionlint invocation; pin the QEMU helper image digest. |
| Runtime images | Require patched Debian `libaom3` in Player/Subtitles. Before loading an image on Nox, produce a complete SBOM and vulnerability report and reject fixable high/critical findings, matching the release policy. Local deployment requires `syft` and `trivy` on PATH. |
| Other libraries | Keep the existing database, authentication, cryptography, MCP, UI, media, and platform stacks. Update compatible versions and monitor their owning manifests; do not replace standards implementations to reduce dependency count. |

The migrated YAML and DNS-SD upstream license/notice texts are retained under `packages/third_party` and copied into the consuming runtime images. Refresh those texts when updating their selected modules.

Installed Nox watchers use copied scripts. Refresh each one with `make -C apps/<app> install-nox-autodeploy` for Player, Subtitles, and Dashboard before publishing this upgrade; the installer includes the deployment scanner.

## Verification contracts

- `make tooling-check`, `make max-loc`, `make packages-check`, and affected apps' `make verify-changed` validate the repository integration. The metrics module has focused negative and race tests; deployment tests prove a rejected scan never loads or starts the remote image.
- The Halstead differential compared 1,168 production Go files and 7,489 function reports with exact equality. The subsequent discovery helper split was compared again. The owned tool's functions also remain under the same threshold.
- The CRAP differential across all five Go modules matched 6,268 functions. Ten differences are method/function name collisions: four in shared navigation, two in Player collections, and four in Subtitles collections/trusted HTTPS. The former checker combined coverage by name; the new position-based calculation correctly attributes each function's covered blocks. None changes the threshold result. The score formula, block weighting, and complexity convention remain unchanged.
- Independent macOS Bonjour discovery observed registration, resolution, port/TXT values, and goodbye removal on two interfaces. This is not physical Home Assistant or every Linux network proof.
- The image scanner also reports module-wide `GO-2026-5932` for unused `x/crypto/openpgp`. None of the five module import graphs includes that package; `govulncheck` found no affected application call path.
- Scans retain all findings, including advisories without a distribution fix. Passing the release gate does not mean a vulnerability-free image. Source tests, image scans, remote publication, deployed revisions, health, and physical-device results remain separate evidence.

The full dated inventory, original decisions, differential reports, model reproduction scripts, and image evidence are retained in the local dependency audit bundle. This note records the chosen changes and verification methods; use the delivery report for the final run status and revision.
