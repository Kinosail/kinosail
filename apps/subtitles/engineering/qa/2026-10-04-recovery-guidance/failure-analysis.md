# R19 and R20: recovery guidance

Baseline: `2e9ede47a` (includes landscape/swipe-close PR 461).

## R19: unavailable-file Settings link

Public behavior: the unavailable-file Library notice must reach the Media Libraries section in Settings.

Failure modes considered before editing:

- The link names a nonexistent fragment, leaving the owner at the top of Settings.
- The notice disappears or uses a different destination in a filtered unavailable view.
- The target resolves to a real but unrelated Settings section.
- An unrelated loaded, empty, or pending view advertises the failed-file recovery notice.

The regression renders a synthetic failed track check through the Library HTTP adapter, follows the emitted Settings path, and resolves its fragment against the actual Settings HTML. It also checks the target heading. Existing coverage verifies unavailable counts and recovery after a source change but does not follow the guidance link. The regular populated-browser data has readable media, so this retained HTTP check covers the unavailable-state gap without changing real user data or adding a production seam.

## R20: deployment-managed provider instructions

Public behavior: documentation distinguishes native configuration, the source/release Compose files, additive overlays, and minimal platform installation files.

Failure modes considered before editing:

- Claiming that `.env` always forwards arbitrary native-process settings.
- Claiming that source/release Compose omits settings it already maps.
- Claiming minimal platform files forward provider settings they do not map.
- Omitting the required personal-use acceptance or secret-file mount requirement.
- Changing exact configuration names, defaults, or deployment files while correcting prose.

The guide and README are reconciled against every checked-in Subtitles Compose base, overlay, platform manifest, the published Player/Subtitles combined manifest, the ZimaOS Subtitles/Both manifests, the TrueNAS shared template, and the inline Docker Compose installation example. This documentation repair changes no configuration values. A behavior-mirroring prose assertion would add little confidence; the recorded manifest inventory is the verification artifact.
