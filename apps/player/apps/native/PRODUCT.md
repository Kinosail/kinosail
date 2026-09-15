# Kinosail native Player product

<!-- impeccable:product-schema 1 -->

## Platform

ios

## Users

Members of self-hosting households using Kinosail Player on iPhone, iPad, and Apple TV. This workspace specializes the confirmed family scope in the root PRODUCT.md.

## Product Purpose

Provide native access to the household Kinosail Server while preserving privacy, security, accessibility, and existing server API contracts.

## Operating Context

The active implementation is a SwiftUI media client with iOS and tvOS targets in Kinosail.xcodeproj. Source implementation is complete for the retained feature set; the user has authorized signed-device installation with test suites remaining disabled. The platform value selects Impeccable's Apple native guidance; tvOS work must additionally follow this workspace's Apple TV and remote-input requirements.

## Capabilities and Constraints

- AGENTS.md and IMPLEMENTATION.md own the active migration scope and work items.
- Compile Sources/; the previous src/ and modules/ implementation is reference only.
- Preserve /api/v1 compatibility and Direct First playback; never fabricate successful server state for unimplemented operations.
- Only iOS and tvOS are current targets. Do not add other platform targets through design work.
- The user has granted creative freedom for this rewrite; previous layouts, colors, fonts, and feature parity are not requirements.
- Compilation, implemented behavior, and physical-device validation remain separate evidence.

## Evidence on Hand

README.md, AGENTS.md, IMPLEMENTATION.md, DESIGN.md and VERIFICATION.md describe the current implementation, design and evidence boundaries.

## Accessibility & Inclusion

Preserve platform accessibility, native navigation and focus, touch interaction on iPhone/iPad, and remote interaction on Apple TV.
