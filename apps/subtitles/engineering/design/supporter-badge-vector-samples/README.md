# Supporter badge vector samples

This study contains ten independent badge systems. Each system shows all ten supporter levels in both Kinosail badge families.

The source files use SVG geometry and paint only. They contain no raster images, embedded data, external assets, masks, or filters.

Open `index.html` to compare all sets. Open an individual SVG to inspect or edit one direction at full resolution.

The concepts preserve three parts of the current system:

- the caption frame as the Subtitles app emblem;
- signal green for the active Living Standard family;
- brass and gold for the permanent Patron Order family.

Levels 7–10 add visible regalia. Lower ranks remain quieter and easier to read at small sizes.

Run these commands after geometry changes:

```sh
node engineering/design/supporter-badge-vector-samples/generate.mjs
node engineering/design/supporter-badge-vector-samples/verify.mjs
```
