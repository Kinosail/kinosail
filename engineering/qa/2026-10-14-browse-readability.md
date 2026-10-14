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

The [Playwright screenshot style option](https://playwright.dev/docs/api/class-page#page-screenshot-option-style) temporarily hides only the measured pseudo-elements.
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
