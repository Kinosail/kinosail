# Shared Kinosail packages

This Go module contains operations and contracts reused by Player and Subtitles. It is part of the root [Go workspace](../go.work), not a separately deployed service.

## Work here

Keep application-specific web adapters and presentation in `apps/<app>/`. Put shared semantic validation and operations here when existing consumers need the same behavior. The app `go.mod` files resolve this module through `../../packages`, so retain the complete monorepo layout.

Examples of shared ownership include [configuration](configurationcore/), [library discovery](library/), and [identity](identitycore/). Consult each package's exported API and tests before extending it. Third-party code under [third_party/](third_party/) retains its own licenses and notices.

## Build and verify consumers

From this directory, `go build ./...` compiles the shared module. Changes can affect more than one app; review all callers and their observable behavior.

When quality gates are enabled, run `make -C packages check` from the repository root and the affected app checks described in [CONTRIBUTING.md](../CONTRIBUTING.md#verification). While `.gates-disabled` exists, the suites remain disabled and skipped checks are not passes.

## License

Kinosail-owned packages use the repository's [license](../LICENSE) and [contribution terms](../CONTRIBUTING.md). Third-party directories retain their own terms. See [LICENSING.md](../LICENSING.md).
