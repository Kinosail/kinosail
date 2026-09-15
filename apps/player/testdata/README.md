# Public test media

`scripts/generate-test-media.sh` creates a small, deterministic Library for Movies, Shows, Music, Audiobooks, Books, and Photos. It also creates a local synthetic TMDB-compatible catalogue so metadata enrichment and artwork caching can be tested without network access or a TMDB account.

No third-party creative media is downloaded or embedded. Generated fixture output is dedicated to the public domain under CC0 1.0 Universal; the generator code remains covered by the repository's software license.

Future contributions must follow `CONTRIBUTING.md`: no downloaded creative media, and any original copyrightable fixture contribution is dedicated under the same CC0 terms.

The local catalogue is an API-compatibility fixture, not TMDB data. A real TMDB integration still requires the Server owner to accept TMDB's terms and provide their own token.
