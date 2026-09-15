---
name: "Kinosail Dashboard"
description: "An Electric household launcher with open application rows and truthful service status."
colors:
  bg: "#0b0d0b"
  surface: "#151914"
  surface-2: "#20271e"
  surface-3: "#2c3428"
  text: "#f6f8f2"
  muted: "#a0a79c"
  line: "#30382c"
  line-strong: "#64705c"
  signal: "#c4ff47"
  signal-ink: "#142000"
  focus: "#daff98"
  danger: "#ffabab"
  success: "#8bd7b1"
  warning: "#f0cb8c"
  info: "#a9c7ee"
  light-bg: "#f4f8ef"
  light-surface: "#fff"
  light-surface-2: "#e9efe2"
  light-surface-3: "#dbe4d2"
  light-text: "#162011"
  light-muted: "#5b6852"
  light-line: "#d4ddcc"
  light-line-strong: "#78866e"
  light-signal: "#3c6100"
  light-signal-ink: "#fff"
  light-focus: "#304e00"
  light-danger: "#a62337"
  light-success: "#23674b"
  light-warning: "#805418"
  light-info: "#305d94"
typography:
  display:
    fontFamily: "Manrope, ui-sans-serif, -apple-system, BlinkMacSystemFont, \"Segoe UI\", sans-serif"
    fontSize: "clamp(2.5rem,4.5vw,4.5rem)"
    fontWeight: 750
    lineHeight: 1.08
    letterSpacing: "-.035em"
  headline:
    fontFamily: "Manrope, ui-sans-serif, -apple-system, BlinkMacSystemFont, \"Segoe UI\", sans-serif"
    fontSize: "1.65rem"
    fontWeight: 800
    letterSpacing: "-.025em"
  body:
    fontFamily: "Manrope, ui-sans-serif, -apple-system, BlinkMacSystemFont, \"Segoe UI\", sans-serif"
    fontWeight: 400
  label:
    fontFamily: "Manrope, ui-sans-serif, -apple-system, BlinkMacSystemFont, \"Segoe UI\", sans-serif"
    fontWeight: 600
rounded:
  control: "10px"
  quiet: "12px"
  panel: "16px"
  artwork: "12px"
  primary: "999px"
spacing:
  action-gap: ".75rem"
components:
  button-primary:
    backgroundColor: "{colors.signal}"
    textColor: "{colors.signal-ink}"
    rounded: "{rounded.primary}"
    padding: ".7rem 1rem"
  button-quiet:
    backgroundColor: "transparent"
    textColor: "{colors.text}"
    rounded: "{rounded.quiet}"
    padding: ".7rem 1rem"
  input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.text}"
    rounded: "{rounded.control}"
    padding: ".68rem .85rem"
  navigation:
    textColor: "{colors.signal}"
  application-row:
    backgroundColor: "transparent"
    textColor: "{colors.text}"
    height: "144px"
  pending-row:
    height: "144px"
---

# Design System: Kinosail Dashboard

## Overview

**Creative North Star: "Electric"**

Electric makes the household's applications easy to recognize and open. A spacious heading and search lead into open destination rows; service status remains supporting context. Add application is the distinct green action, while editing and checking keep quieter treatments.

The user selected Electric option 1 across the active web apps, replacing the prior coral system. `packages/webassets/static/last-light.css` owns shared roles; `internal/server/web/static/dashboard.css` owns Dashboard layout. Root `PRODUCT.md` preserves direct application links, bounded service-health information, permissions and independent app delivery.

**Key Characteristics:**

- Recognizable application rows with nearby direct links.
- A single dominant green creation action and quiet administration.
- Real service state and placeholders aligned with loaded rows.

## Colors

### Primary

Electric green (`signal`) marks the main action, selected destination and saved-progress line; `signal-ink` supplies its readable foreground. The light appearance uses deep leaf green and white action ink. Pale green focus remains distinct from the selected state.

### Neutral

Black-green canvas and successively lighter green-gray surfaces separate the page, controls and overlays. Off-white text and sage-gray supporting text establish hierarchy. The light counterpart uses pale green paper, white surfaces and dark green text. Use the matching light roles together rather than carrying the dark action color into a light page.

### State

Success, warning, danger and information keep their existing semantic colors and readable labels. Green action color does not prove a service is healthy or an operation succeeded.

**The Action and State Rule.** Use Electric green for action or selection; attach actual state text to operational outcomes.

## Typography

Manrope is the self-hosted variable web family (weights 200–800), served at `/static/manrope.woff2?v=1` with `font-display: swap`. Its source and license live in `packages/webassets/static/fonts/`. Compact negative tracking and balanced wrapping belong to headings; body copy and controls use normal tracking. Supporting counts and time values use tabular numerals.

The frontmatter records reused roles from the current cascade. Apple clients have a separate rounded system-type contract; do not export fixed web sizes into native controls.

## Layout

The header is at least 80px high. The board is capped at 96rem with `clamp(1.25rem,4vw,4rem)` horizontal padding; flexible household copy sits beside a 16–25rem service-status column. The heading is followed by a divider, search and nearby board actions. Supporting copy is bounded to 45ch.

Application rows form two columns with a 2rem gutter. Rows and loading placeholders share 144px height. A 52px mark sits beside title, description and health text, with the direct-open affordance at the far edge.

Below 760px the board stacks, the heading is 2.75rem, service status becomes a full-width strip, and search takes the available width. Below 480px the application and pending grids become a single column with 48px marks and 1.1rem page padding. Editing expands the affected row to at least 208px rather than hiding its controls.

## Elevation & Depth

Tonal surfaces, fine dividers and real content provide depth. Ordinary panels and resting content have no shadow or backdrop blur. Bounded menus and dialogs use the shared overlay shadow; its dark and light values live in the sidecar extensions.

**The Stable Content Rule.** Web hover changes color or framing without lifting content. Shared color transitions last 180ms with `cubic-bezier(.16,1,.3,1)`; reduced-motion rules suppress animation and transitions.

## Shapes

The strong primary action uses the capsule radius. Ordinary fields and controls use the smaller control radius; quiet buttons use their separate radius. Panels and artwork have modest corners. Open rows use dividers and transparent backgrounds instead of a rounded card around every object. Native system components retain their own geometry.

## Components

### Buttons

Add application uses the Electric primary capsule. Edit board, service checking and header utilities remain quiet controls. Board actions are at least 48px high with a 0.75rem gap. Destructive operations retain their own roles and feedback inside the existing dialogs.

### Inputs / Fields

Fields keep visible labels, native form behavior and a minimum 44px control height. Shared keyboard focus is a 2px outline with a 4px offset; fields use a 2px offset. At 900px and below, text entry uses at least 16px type. Disabled controls retain their existing disabled semantics and reduced visual emphasis.

### Navigation and filters

Application category filters are text controls with a 44px minimum. Selection uses green text and a bottom rule, not a filled badge. The search field is 50px high with a 2px focus outline offset by 3px. Search, favorites, recent access and the command menu retain their existing behavior; application links open the configured destination directly.

### Cards / Containers

Applications are open divider rows, with no resting card shadow or enclosing radius. Hover and focus-within add a tonal surface and thin frame without moving the row. Marks use the artwork radius and retain their application identity. The Dashboard family mark keeps its authored panel geometry in Electric green.

Settings and command dialogs use bounded surface panels, the panel radius and shared overlay shadow. Settings disclosures organize real fields and operations; their 48px summaries preserve keyboard access.

### Progress, pending and recovery

Service labels distinguish confirmed health, disabled checks, unavailable state and active work. A green action or selected filter does not establish service health.

Loading rows keep the two-column/one-column grid and loaded row height; placeholder marks and text align with their eventual positions. They appear only during the request. Loaded, empty, offline and failed states use their real content and recovery controls. The current review includes a capture of an actual pending request.

### Asset provenance

The active app mark is authored SVG geometry, recolored to the approved Electric family. Web PNGs derive from the app's SVG assets; the native raster pipeline uses the authored files under `apps/player/apps/native/assets/source/` and `scripts/generate-brand-assets.sh` in that workspace, with `sips` and `ffmpeg`. No generated raster is a source of truth for logo geometry. Family provenance and active paths are recorded in `engineering/design/cinema-direction.md`.

## Do's and Don'ts

### Do:

- Do use semantic roles in both appearances and retain visible keyboard focus.
- Do keep status, empty, pending and failure states tied to the underlying operation.
- Do preserve real content, accessible labels, native form behavior and reduced-motion support.

### Don't:

- Don't recolor the shell from individual artwork.
- Don't add decorative background imagery to administration or lift web browsing content on hover.
- Don't turn leftover kicker, glyph-icon or legacy style declarations into new shared patterns.
