# Public site screenshot provenance

The public homepage images were captured from the actual Player web interface
using a separate loopback-only Podman Server on 2026-09-23. The image was built
from repository commit `bfde2260de23e6aac54f8ba119118e5267facd43`. Playwright
captured Chrome at a 1600 by 900 viewport with a 2x device scale. Each full-size
capture is 3200 by 1800 pixels. The lower-resolution WebP files were made from
those captures for smaller screens and standard-density displays. No personal
library, production state, account details, hostnames, or credentials are in
the captures.

The four film titles, plots, years, ratings, and video files were created for
this demo: The Last Observatory, The Quiet Coast, After the Rain, and A Place
Between. The poster and backdrop artwork was generated for these fictional
films. Screenshots are labelled as a fictional library on the public page; they
do not claim to show licensed films or real viewing history.

The Server was built from the task checkout with the approved green tvOS CinemaSail background enabled. Media was mounted read-only. The Owner Profile and its credentials existed only in the disposable capture environment. This was not an installer or production-hardening qualification.

The screenshots retain the actual UI. The page serves a 3200-pixel image to
high-density screens and smaller responsive versions to other screens. WebP
conversion does not add or remove controls or product capabilities.

To reproduce: run Player with entirely separate data, cache, backup, and media
directories; create synthetic MP4 files and matching basename NFO/PNG sidecars;
complete setup using a demo-only Owner; capture the home and film detail views
without browser chrome. Inspect all visible text and images before publication.
Never reuse real media, real Owner details, or an existing Server's state.

The Why page adapts the creator's supplied marketing thread
01a0a67b-3ebe-7093-a062-115aa222ee55, especially its two stated project goals and
explicit AI-engineering disclosure. It makes no claims about unreleased apps.
