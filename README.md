# Kinosail

This is the monorepo for Kinosail's self-hosted applications and native clients.

## Applications

- `apps/player` — Kinosail Player and its shared native client source
- `apps/subtitles` — Kinosail Subtitles
- `apps/dashboard` — Kinosail Dashboard
- `packages` — Player-baseline modules reused by multiple applications

Each product remains independently buildable, deployable, versioned, and
health-checked. The monorepo does not combine them into one binary, container,
or release.

Kinosail Supporter and the Kinosail Home Assistant integration remain in their
existing repositories because they have different trust, packaging, and release
boundaries.

Run a gate across every application from the repository root, or work directly
inside one application:

```sh
make hooks
make check
make -C apps/player check
```

Kinosail is source-available, not open source.

Repository-wide terms, including the shared `packages/` module, are described
in [LICENSING.md](LICENSING.md).
