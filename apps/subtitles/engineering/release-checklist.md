# Release verification

Use this checklist from a clean commit on `main`. `make check` is the authoritative code gate; container and populated-browser checks cover separate release surfaces.

## Candidate gates

- [ ] `make check` passes: module size and tidy checks, lint, dead-code report, coverage ratchet, installer lifecycle, shell/workflow validation, secret scan, race detector, and vulnerability scan.
- [ ] `make container-test` passes against the production `Containerfile`, including non-root startup, FFmpeg capabilities, backup creation, restart persistence, Library monitoring, direct media, adaptive HLS, and trick-play output.
- [ ] `make test-instance-check` passes against the generated populated fixture and Chromium journey.
- [ ] `KINOSAIL_BROWSER_MATRIX=full make browser-test` passes when Chromium, Firefox, and WebKit are installed.
- [ ] `git diff --check` passes and `git status --short` contains only the intended release changes.

## Self-hosting acceptance

- [ ] A fresh installer directory starts one localhost-only `kinosail` service and reaches `/healthz`.
- [ ] The default listener rejects plain HTTP and TLS 1.1, negotiates TLS 1.2/1.3 with only ECDHE AEAD suites, uses a generated leaf signed by the persistent local CA, and reuses the authority after restart.
- [ ] The first Owner Profile can be created, must enroll one passkey or TOTP authenticator, survives a container restart, and a second setup attempt is rejected; one Owner remains sufficient.
- [ ] `install.sh ... --lan` refuses before Owner setup and succeeds after setup.
- [ ] A second installer run creates and verifies an encrypted Recovery Backup before updating.
- [ ] The stable `kinosail` Compose project reconnects to the same configuration, cache, and backup volumes when the installer is extracted elsewhere.
- [ ] Only the configured media mount is writable for sidecar creation; the root filesystem is read-only; the container runs as UID/GID 10001 with all Linux capabilities dropped, `no-new-privileges`, PID/CPU/memory/file limits, bounded `noexec` temporary storage, and rotated logs.
- [ ] The installer rejects an unsigned or incorrectly identified image, launches the verified digest, and can advance from a previously pinned digest to the tag selected by `KINOSAIL_VERSION`.
- [ ] Encrypted automatic backup creation, verification, restore, pinned-version rollback, and uninstall-with-data-preserved are exercised; missing backup-key material fails closed.

## Native payload readiness

- [ ] `make native-build` creates identical Linux, macOS, and Windows core archives across two builds for AMD64 and ARM64.
- [ ] `kinosail-release.json` contains exactly six core artifacts, their sizes and SHA-256 digests, compatibility schemas, and update schema version 2.
- [ ] The release publishes `kinosail-native-installation.json`; each core archive contains the same contract.
- [ ] The release publishes a Sigstore bundle for `kinosail-release.json`; the signed manifest covers each archive and the installation contract.
- [ ] Each future native package follows the signed contract for identity, preflight checks, local defaults, repair, silent mode, read-only media access, stable paths, service ownership, crash recovery, rollback, and uninstall.
- [ ] Each native package installs its platform scheduler and update agent; disabled mode performs no network access without a manual request.
- [ ] Windows Authenticode, Apple Developer ID and notarization, and Linux package signing are verified when native installer packages are introduced.

## Publish

- [ ] Confirm the repository and GHCR package will be anonymously readable before announcing the release. Changing repository or package visibility requires explicit Owner approval.
- [ ] Enable GitHub private vulnerability reporting and verify that `SECURITY.md` reaches the private report form.
- [ ] Create a version tag only from the verified commit and wait for the Release workflow to finish.
- [ ] Confirm the release contains `kinosail-install.tar.gz`, its SHA-256 file, Sigstore bundle, SBOM, and GitHub provenance attestation.
- [ ] Confirm anonymous pulls for both `linux/amd64` and `linux/arm64`, then install the published image from a clean release bundle without repository credentials.
- [ ] Confirm the published version is reported by `kinosail version`; verify the image signature and provenance by digest.
- [ ] Follow the self-hosting guide on a second LAN device and verify browse, direct playback, compatible playback, progress, logout/login, and restart persistence.

## Honest boundaries

- Physical GPU passthrough, HDR display output, actual TV/mobile clients, off-LAN networking, firewall/router policy, constrained networks, 10 GiB downloads, and host-loss recovery require their stated hardware or environment.
- Native core archives are installer inputs, not end-user installers. Service registration, private media-runtime bundles, operating-system signatures, and notarization remain package-specific work.
- LAN HTTPS supports the web player, password login, passkeys, and the installable PWA after the generated or custom certificate is trusted for the exact origin. Some third-party clients require a publicly trusted certificate.
