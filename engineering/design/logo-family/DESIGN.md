---
name: Kinosail logo family
description: Distinct app silhouettes in the original Kino colors.
colors:
  kino-lime: "#c8f169"
  kino-paper: "#f6f8ef"
  kino-ink: "#090a08"
rounded:
  web-icon: "112px"
---

# Design System: Kinosail logo family

## Overview

Player, Subtitles, and Dashboard share the original Kino colors and flat, solid geometry. Each app has a distinct silhouette: a split sail for Player, paired caption bands for Subtitles, and three dashboard panels. This document governs logo assets only.

The rendered contact sheet is [logos.png](logos.png). App interface design, typography, and layout remain owned by their existing design documents.

## Colors

Kino lime carries the main shape, Kino paper supplies the contrasting shape, and Kino ink supplies the background. Use the frontmatter values directly; no gradient, tint, or shadow is part of the marks.

## Layout

Web icons use a square 512-unit viewBox and a rounded background. Maskable Player and Subtitles variants use a full-bleed square background and transform the mark with `translate(51.2 51.2) scale(.8)` to create additional crop clearance. Preserve those variants instead of applying the ordinary rounded icon to a maskable slot.

## Shapes

- Player: lime triangle at `(160,112)`, `(384,256)`, `(160,304)`; paper sail fragment at `(160,336)`, `(288,308)`, `(160,400)`.
- Subtitles: two broad caption bands with opposing tails, spanning x=128–384 and y=144–384.
- Dashboard: a tall left panel and two stacked right panels, spanning x=128–384 and y=128–384, with 32-unit gutters.

**The Geometry Rule.** Preserve the complete path geometry and proportions when resizing; change the SVG viewport or render dimensions rather than redrawing individual pieces.

## Components

Authoritative web SVGs are `apps/player/internal/server/static/icon.svg`, `apps/subtitles/internal/server/static/icon.svg`, and `apps/dashboard/internal/server/web/static/icon.svg`. Player and Subtitles keep adjacent `icon-maskable.svg` variants. Inline header marks and documentation copies must track their app's authored geometry.

Player native sources live in `apps/player/apps/native/assets/source/`. The native `icon.svg` uses a square background so Apple owns the final corner treatment. `tv-foreground.svg` contains the transparent mark; `tv-background.svg` supplies the background for the tvOS image stacks. `tv-banner.svg`, `top-shelf.svg`, and `top-shelf-wide.svg` own the corresponding compositions.

Regenerate the Swift asset catalogs from the repository root on macOS:

```sh
bash apps/player/apps/native/scripts/generate-brand-assets.sh
```

The script requires `sips` and `ffmpeg`; it emits the iOS icon as RGB and renders tvOS foreground and background layers separately. It does not generate the web PNGs or the contact sheet. Keep raster exports derived from their SVG sources rather than hand-editing them.

Verification for this documentation consists of source inspection and visual inspection of the rendered contact sheet. `.gates-disabled` remains in force: no tests or detector commands were run for this documentation. This evidence does not establish populated-browser behavior, deployment state, installed-icon appearance, tvOS parallax, or physical-device rendering.

## Do's and Don'ts

- Do retain the original Kino colors and distinct app silhouettes.
- Do update raster exports when their authored SVG changes.
- Don't add decorative gradients, shadows, or outlines to the marks.
- Don't apply these logo-specific rules as a replacement for an app's interface design system.
