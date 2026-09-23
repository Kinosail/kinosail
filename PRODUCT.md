# Kinosail product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Self-hosting households: Owners administer their servers and Viewers use their media and household applications. The user confirmed this shared scope during Impeccable initialization.

## Product Purpose

Kinosail Player provides private media browsing and playback. Kinosail Subtitles finds, validates, and adds subtitle sidecars for media the household controls. Kinosail Dashboard provides direct access to configured household applications and bounded service-health information.

## Operating Context

This monorepo owns Player, Subtitles, Dashboard, shared packages, and native Player clients. Web servers are independent Go applications with embedded browser assets. Each application retains its own binary, container, version, release, deployment, and health evidence.

The native Player workspace has its own PRODUCT.md and platform guidance. Supporter and Home Assistant remain in separate repositories.

## Capabilities and Constraints

- Kinosail Server is free to run on household hardware and includes the web Player. Compatible Jellyfin mobile apps for iOS and Android are an optional integration, disabled by default and requiring trusted HTTPS.
- Preserve privacy and Direct First playback; buffering alone does not authorize transcoding.
- Web and versioned HTTP API adapters call shared validated application operations.
- Validate untrusted inputs before side effects and preserve application-specific authorization.
- Dashboard opens application addresses directly and does not proxy application content.
- Kinosail is source-available, not open source.
- Existing AGENTS.md instructions remain authoritative. Quality gates remain disabled while .gates-disabled exists.

## Evidence on Hand

Application README.md files describe current capabilities and operating boundaries. Native implementation status is recorded in apps/player/apps/native/README.md and IMPLEMENTATION.md. Build, source publication, deployment, browser behavior, and physical-device behavior are separate evidence; do not infer one from another.

## Product Principles

- Keep household media and administration private.
- Preserve direct access and truthful state and failure feedback.
- Reuse shared application behavior while keeping app delivery independent.
- Make common household tasks understandable and accessible.

## Accessibility & Inclusion

Preserve keyboard access, visible focus, responsive operation, reduced-motion support, and accessible feedback. Native clients must preserve platform accessibility and appropriate touch or remote interaction.
