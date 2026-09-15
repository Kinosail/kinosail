# Warm and trustworthy dark color for Kinosail

Research snapshot: 2026-08-30.

This brief translates primary color research into dark-mode palette choices for Kinosail. It is a design input, not an implementation specification.

## Recommendation

Use **Olive Hearth** as the first screenshot direction:

- warm olive-charcoal surfaces;
- a lighter, softer pear-green signal;
- warm cream text; and
- brass only for atmosphere or rare emphasis.

This keeps Kinosail recognizable. It also avoids the common mistake of making the whole interface bright or saturated to make it attractive.

No study proves that one green hex value makes an app trustworthy. The stronger evidence supports a complete composition: controlled saturation, familiar structure, clear contrast, and color that fits the product context.

## What the evidence supports

### Limit saturation before changing the brand hue

Skulmowski et al. tested saturated and desaturated versions of 50 websites from ten content domains. High saturation reduced trustworthiness and appeal in a context-dependent way. Appeal formed first, while trust judgments formed later. This supports low-chroma surfaces and a limited bright accent, not a saturated green wash. [Skulmowski et al., *The negative impact of saturation on website trustworthiness and appeal*](https://doi.org/10.1016/j.chb.2016.03.054)

Valdez and Mehrabian found stronger and more consistent emotional effects from brightness and saturation than from hue alone. Green and blue-green were among the pleasant hues, but the tests used isolated color chips. For Kinosail, tune lightness and saturation before replacing green with a supposed “trust color.” [Valdez and Mehrabian, *Effects of color on emotions*](https://pubmed.ncbi.nlm.nih.gov/7996122/)

Wilms and Oberfeld independently varied hue, saturation, and brightness. All three dimensions and their interactions affected valence and arousal. Saturated and bright colors increased arousal and physiological response. A bright signal can therefore add energy, but broad use can make a calm media interface too active. [Wilms and Oberfeld, *Color and emotion*](https://doi.org/10.1007/s00426-017-0880-8)

### Add warmth through lightness and supporting colors

Hammond et al. found that greater luminance increased perceived warmth for every tested primary hue. The effect was strong for green. This result came from colored light, not application screens, so it supports a direction rather than a precise token. A lighter pear green can feel warmer than a dark or blue-green signal. [Hammond et al., *Increasing intensity directly increases the perceived warmth of primary colors*](https://doi.org/10.1038/s41598-024-77942-1)

Observers in Manalansan et al. placed warm-to-cool color along an orangish-red to greenish-blue direction. This supports moving the current signal slightly toward yellow or olive instead of cyan. It does not show that warm colors increase application trust. [Manalansan, Whitehead, and Webster, *Warm versus cool colors and their relation to color perception*](https://doi.org/10.1167/jov.25.4.13)

### Trust comes from context and composition

An online experiment with 240 participants found that hue-context congruence increased trust in a tourism website. The study used only red and blue. Its useful lesson is contextual fit, not a universal winning hue. [Khrouf and Frikha, *Websites' hue-context congruence as a vector of trust and behavioral intentions*](https://doi.org/10.1108/IJOEM-05-2020-0474)

Kim and Moon changed color, imagery, menus, and layout across cyber-banking interfaces. Those combined design elements changed perceived trustworthiness and elegance. Treat trust as a whole-screen result, not a property of green alone. [Kim and Moon, *Designing towards emotional usability in customer interfaces*](https://doi.org/10.1016/S0953-5438(97)00037-4)

A 2025 experiment found that green and red background cues changed perceived facial trustworthiness. Subtle cues did not change trusting behavior, while salient cues did. Green can support a positive convention, but it is not proof that the product is trustworthy. [Schotz et al., *Social versus nonsocial visual cues of trustworthiness uniquely influence trust related behavior and memory*](https://doi.org/10.1038/s41598-025-17094-y)

Miniukovich and Figl studied 1,530 participants and more than 3,000 webpages. Page prototypicality had a strong association with perceived trustworthiness. The study was correlational and used screenshots, but it supports preserving familiar media-app structure while Kinosail changes its mood. [Miniukovich and Figl, *The effect of prototypicality on webpage aesthetics, usability, and trustworthiness*](https://doi.org/10.1016/j.ijhcs.2023.103103)

### “Pretty” requires restraint and a clear first frame

Reinecke et al. collected colorfulness, complexity, and appeal ratings for 450 websites from 548 people. Colorfulness mattered, but visual complexity mattered more. The effects also varied with age, gender, and education. Use artwork for rich color and keep application chrome coherent. [Reinecke et al., *Predicting users' first impressions of website aesthetics*](https://doi.org/10.1145/2470654.2481281)

Schloss and Palmer found that preference and harmony for color pairs increased with hue similarity. Preference also depended on component preference and lightness contrast. This supports neighboring olive surface tones, with stronger lightness contrast for the pear signal. [Schloss and Palmer, *Aesthetic response to color combinations*](https://doi.org/10.3758/s13414-010-0027-0)

Palmer and Schloss found that color preference tracked associations with liked and disliked objects. This means that personal, cultural, and product associations remain important. A palette must be tested with Kinosail artwork and users. [Palmer and Schloss, *An ecological valence theory of human color preference*](https://doi.org/10.1073/pnas.0906172107)

### Dark mode needs extra reading care

Piepenbrock et al. found better visual acuity and proofreading performance with dark text on a light background for younger and older adults. Dark mode can still fit a cinema product, but it is not inherently more readable. [Piepenbrock et al., *Positive display polarity is advantageous for both younger and older adults*](https://doi.org/10.1080/00140139.2013.790485)

Dobres et al. found that white-on-black text needed longer exposure under dark illumination, especially at small sizes. A later two-study experiment with 459 participants also found a small, reliable light-mode reading advantage that did not match stated preference. Use large, high-contrast text and short metadata in dark mode. [Dobres et al., *Effects of ambient illumination, contrast polarity, and letter size*](https://doi.org/10.1016/j.apergo.2016.11.001); [Palmén, Gilbert, and Crossland, *How bold can we be?*](https://doi.org/10.1145/3544548.3581552)

## Recommended dark tokens

These values are solid sRGB starting points. They do not replace tests on the rendered pixels.

| Role | Token | Value | Intent |
| --- | --- | --- | --- |
| Canvas | `--bg` | `#0C110E` | warm near-black with a green undertone |
| Quiet surface | `--surface` | `#162019` | main grouped region |
| Raised surface | `--surface-2` | `#222C23` | selected or interactive region |
| Strong surface | `--surface-3` | `#303A30` | hover or nested control only |
| Primary text | `--text` | `#F4F1E8` | warm projection-light cream |
| Secondary text | `--muted` | `#B7B1A5` | warm gray metadata |
| Strong boundary | `--line-strong` | `#65705C` | essential control boundary |
| Primary signal | `--signal` | `#A9C46A` | pear-olive action and progress |
| Text on signal | `--signal-ink` | `#172009` | dark label on filled signal |
| Focus | `--focus` | `#D7EAA1` | keyboard and remote focus only |
| Warm accent | `--warm-accent` | `#D4A76B` | sparse brass atmosphere or special metadata |
| Danger | `--danger` | `#FF9188` | error or destructive action with text or icon |

The recommended solid pairs have these WCAG 2 contrast ratios:

| Pair | Ratio |
| --- | ---: |
| `--text` on `--bg` | 16.87:1 |
| `--muted` on `--surface` | 7.85:1 |
| `--signal` on `--surface` | 8.60:1 |
| `--signal-ink` on `--signal` | 8.65:1 |
| `--warm-accent` on `--surface` | 7.60:1 |
| `--line-strong` on `--surface` | 3.21:1 |

[WCAG 2.2](https://www.w3.org/TR/WCAG22/) requires at least 4.5:1 for normal text, 3:1 for large text, and 3:1 for essential non-text controls. It also prohibits color as the only visual way to convey information. [WCAG 2.2 use of color](https://www.w3.org/WAI/WCAG22/Understanding/use-of-color); [WCAG 2.2 non-text contrast](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast)

Kinosail currently uses translucent surfaces over artwork. The final contrast depends on compositing. Use an opaque scrim behind text and controls when artwork can enter the sample area.

## Screenshot variants

Use the same populated route, artwork, viewport, and content in every comparison. Change only the color roles.

| Variant | Canvas and surface | Signal | Warm accent | Expected character |
| --- | --- | --- | --- | --- |
| **Olive Hearth** | `#0C110E`, `#162019` | `#A9C46A` | `#D4A76B` | best balance of warmth, identity, and calm |
| **Moss Cinema** | `#0D0F0B`, `#171A14` | `#BEDB6D` | `#D1A85B` | more luminous and cinematic |
| **Sage Velvet** | `#110E0C`, `#1D1814` | `#8FC99A` | `#CB955F` | softer, domestic, and inviting |

Do not combine every accent in one screen. Use green for the primary action, active navigation, progress, and focus. Use brass for atmosphere or one exceptional detail. Let posters provide most saturated color.

An 80% neutral, 15% green, and 5% brass or artwork split is a screenshot hypothesis. It is not a published threshold.

## Dark-mode acceptance checks

- Keep the first frame recognizably a media library.
- Keep one obvious green primary action.
- Keep normal text at 4.5:1 or higher on its final composited background.
- Keep essential control boundaries and state cues at 3:1 or higher.
- Add text, icons, shape, or position for selected, played, ready, warning, and error states.
- Compare populated desktop, tablet, compact mobile, and 320 px screenshots.
- Inspect missing artwork, long titles, hover, focus, active, disabled, loading, empty, and error states.
- Check forced colors and user color-scheme preference. The CSS specification defines `prefers-color-scheme` as the user's expressed light or dark preference. [Media Queries Level 5](https://www.w3.org/TR/mediaqueries-5/#prefers-color-scheme)
- Test readability in both dim and bright rooms. Dark mode preference does not prove reading performance.

## Evidence limits

- Most studies used websites, isolated color chips, colored light, or short laboratory tasks. They did not test Kinosail.
- Trust ratings are not the same as secure behavior or long-term product trust.
- Hue meanings change with context, culture, content, prior experience, saturation, and lightness.
- Several trust studies used shopping, banking, tourism, or face judgments. Transfer to a private media app is an inference.
- WCAG ratios are minimum accessibility constraints. They do not prove beauty, comfort, or trust.
- The three palettes need rendered comparison with real Kinosail artwork before selection.
