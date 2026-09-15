# Keep native installation and replacement outside the Server process

Each native package owns its machine service, private media runtime, filesystem layout, and one-shot Update Manager. The Kinosail Server exposes one shared update plan and report contract, but it never replaces its running executable or invokes privileged package tools. This keeps Windows, macOS, Linux, and Docker native while preserving one Server implementation and one safety policy.

## Consequences

Every Update Manager must verify the signed release manifest and selected artifact, defer active playback, create and verify a Recovery Backup, restart the Server, and prove health. A failed update must restore both the prior release and its private state before restart. Each native package keeps one stable package identity and a crash-safe transaction journal. Automatic updates remain off by default. Native packages must preserve Server data during normal uninstall. No background sidecar, beta channel, or release-channel selector is allowed.
