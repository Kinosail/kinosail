# Browse counts and navigation icons

The populated browse run at `fe6418f3b27155ed18d6fbf3cb34451874ee3b8f` passes all 21 selected cases.
Run `38640412140` retains artifact `11929802766`.
Its archive SHA-256 is `1557b4949d34f635122e491e2e5d0dca9002f10f3cb67342a58fc9f722450eed`.
Exact source pins, case identities, child hashes, image revision, and fixture cleanup verify.
All 59 saved screenshots were inspected.

The Shows toolbar displays `1 items` at phone and desktop sizes.
The playlist index also uses an unconditional plural in its template.
Its single-item rendering needs a dedicated browser baseline.
Forced-colors screenshots show an empty Actions circle and missing bottom navigation icons.
Accessible names, layout assertions, and the existing axe check still pass.

## Regression before repair

Add one populated journey before changing application behavior.
Check browse counts at 390, 1440, and 1920 pixels.
Create an owned playlist and check its index at zero, one, and two items.
Delete that playlist and close its auxiliary page independently, preserving any original failure.

Measure six navigation controls at 720 by 450 pixels in both themes, with forced colors off and on.
Wait for fonts, then capture hidden glyphs, normal glyphs, and hidden glyphs again.
The two hidden captures must have no changes reaching 3:1 contrast inside each control.
The normal capture must add at least eight pixels reaching that contrast.
Retain each PNG and a receipt with revision, browser, control geometry, and pixel counts.
Ordinary-color controls must pass before a forced-colors failure is qualified.

A temporary CSSOM stylesheet hides only the measured pseudo-elements.
The test checks each glyph is hidden before capture and visible for the painted control.
It removes that stylesheet in a `finally` block.
This negative control does not replace the Server response or change its content security policy.
The baseline and acceptance runs must establish that the check operates under the production policy.
These controls test painted icons; they do not replace device accessibility testing.

The [CSS color-adjustment specification](https://www.w3.org/TR/css-color-adjust-1/#forced-colors-properties) explains how forced colors can replace background paint.
The navigation glyphs use a masked background.
A repair must retain the user's system colors and keep the rest of the interface adjustable.
The initial checkpoint contains tests only; no rendering repair is claimed.

## Admission and proof boundaries

Include the new journey in the closed browse selection and require exactly 22 executed cases.
Keep one worker, zero retries, existing failure limits, and verified fixture and trust cleanup.
The changed runner contract first fails against the 21-case runner.
All ten isolated runner contracts then pass with the updated candidate.
They cover omitted, extra, skipped, and failed results; they do not prove browser behavior.

Local application and browser runtime remain held.
Hosted artifacts must establish the failing baseline and the repaired result separately.
No physical-device, deployment, TLS outside the disposable fixture, or full-product completion is claimed.

## First hosted baseline

Run `38643751399` tests commit `0c11cdcc729cb428fca69d95a7914461e5e2f185`.
Artifact `11931183972` has archive SHA-256 `e0c5f99139c3806333d5616453d6196811c1e03352f8325419d0358469d3b953`.
The singular Shows count fails at all three widths; the single-item playlist index also fails.
Plural counts and owned-playlist cleanup pass.
The first case fails, and the configured failure limit leaves the other 21 cases unrun.
Fixture containers, volumes, private media, and browser trust are removed successfully.

The initial icon measurement is invalid: its ordinary-color controls also report zero pixels.
The saved normal and hidden images both show the icons.
Eight trace console events identify rejected inline styles under `style-src`.
The screenshot style option did not hide the glyphs under the production content security policy.
Replace that negative control with a temporary CSSOM stylesheet and assert its computed visibility.
Repeat the unchanged application baseline before qualifying a forced-colors failure.

The standard container smoke fixture does not run this spec with `KINOSAIL_TEST_INSTANCE=1`.
Its separately prepared Owner journeys use an explicit file list that excludes this spec.
The closed browse campaign supplies the required one Show and four Movies.

## Count repair

The Library toolbar and playlist index now use the existing singular/plural template pattern.
A count of one renders `1 item`; zero and larger counts retain `items`.
This changes displayed copy only; request handling, stored data, and failure paths are unchanged.
The hosted regression above precedes this repair.
Repaired browser acceptance remains pending.

## Qualified icon baseline and repair

Run `38644650754` tests the corrected control at `ccd53bb151b42e4b5447275059c2843e12f34b57`.
Artifact `11932825085` has archive SHA-256 `d570bc0ffeb5a29189154b180fa3d5ded9b869dc6142245fc6eb2976ec9f3b83`.
All six ordinary-color icons contribute 23 to 150 pixels reaching 3:1 contrast across both themes.
All twelve forced-colors measurements contribute zero qualifying pixels.
Every pair of hidden controls has zero qualifying differences.
The four singular-count failures repeat, and fixture cleanup passes.
This baseline qualifies the missing forced-colors glyphs before the CSS repair.

The repair scopes `forced-color-adjust: none` to navigation and Actions pseudo-elements.
Their background paint uses the system `CanvasText` color.
The rest of the interface remains subject to forced-color adjustment.
The shared stylesheet keeps its line count so the strict Subtitles stylesheet patch remains applicable.
Hosted acceptance must cover the repaired browse journey and both affected applications.
