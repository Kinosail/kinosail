# R04: automatic Library pagination

Baseline: `2e9ede47aa56f5177b1e994ffd4b1c62f6c445ce`.

The public interface is the Server-rendered Library and its bounded
`X-Kinosail-Library-Page: 1` continuation fragment. Show cards are articles
whose canonical detail link is nested. Movies and other media use anchor cards.

Failure modes identified before production changes:

- The wrapper has no href. A null deduplication key drops every later Show.
- Overlapping pages or repeated rows append duplicate cards. Identity must use
  the detail destination, including media type, and update within each page.
- Mixed pages introduce a new group, so deduplication must also work there.
- Invalid, missing, external, query-bearing, or malformed card destinations
  must not become identity. Reject the fragment before appending any cards.
- A delayed response after an HTMX search/sort replacement must not append to
  the new Library or overwrite its status.
- A failed continuation must retain loaded cards, clear busy state, and expose
  a keyboard-accessible retry. Without IntersectionObserver, normal pagination
  must stay accessible. Pending, loaded, empty, and failed surfaces stay usable.

The primary regression runs Playwright against a real disposable Go Server
with synthetic media and actual API/HTML/static responses. Existing
`library-scroll.spec.ts` uses a one-movie hand-authored fixture; it cannot catch
Show wrappers, real canonical IDs, mixed groups, or Server page semantics.
No production-only test seam is needed. Intercepted response tests are retained
only for malformed/overlapping fragments and delayed lifecycle failures which
ordinary stable Server pages do not naturally produce. Those cases are isolated
browser checks, not populated Server E2E evidence.

No household data, devices, deployments, encoders, or native clients are used.
