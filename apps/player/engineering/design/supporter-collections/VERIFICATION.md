# Verification of the design deliverable

- `python3 verify.py`: 200 distinct display SVGs, 200 distinct compact SVGs, complete ten-rank/two-family coverage, local SVG paint references, accessible titles, no embedded raster/remote/script content in the 600 badge and certificate SVGs. Ten vector collection sheets parse.
- `node --check studio.js` and `node --check supporter-preview.js`: passed.
- Chromium in-app browser: all ten gallery selections show twenty badges without broken images or horizontal overflow at 320 CSS pixels. Artwork dialog opens, Escape closes it, and focus returns to its trigger.
- Supporter prototype inspected at 320, 390, 768, and 1440 CSS pixels without horizontal page overflow. Narrow-screen annual pricing and the 390px full page were visually inspected.
- Checked both-family, one-time-only, recurring-ended, and new-supporter states, navigation hiding (including removal of its rank tooltip), annual pricing, and invalid collection-query fallback. Download targets point to the corresponding family SVG.
- Twenty PDF pages rendered with Poppler and inspected as a contact sheet. PDF inspection confirms twenty pages and zero embedded raster images; SVG-derived badge artwork remains vector in the PDF.
- Root `make max-loc` and Player `KINOSAIL_VERIFY_WORKTREE=1 make verify-changed`: passed for the design-only files.

This is an isolated design proposal, not a live app implementation. Production APIs, billing, activation, authentication, and existing artwork were not changed. Physical iPhone/Android devices, Safari/Firefox, native share-sheet completion, and OS-level reduced-motion/forced-colors emulation were not tested. The prototype includes reduced-motion and forced-colors CSS. Screenshots document the inspected browser, not physical-device evidence.
