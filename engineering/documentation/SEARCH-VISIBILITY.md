# Search and AI discovery

The primary audience is people evaluating and installing a self-hosted media
server with Docker. The public homepage defines Kinosail Player, and links to
installation, complete features, MCP, privacy, licensing, and the creator story.
Existing guide URLs remain stable. Search intent is covered by useful answers,
not duplicate keyword pages or invented reviews and ratings.

## Implemented

- Static HTML content and ordinary crawlable links, including FAQ answers.
- Descriptive titles and descriptions, canonical URLs, XML sitemap, and a
  generated `robots.txt` that names the sitemap for the build's public URL.
- Organization, WebSite, WebPage, and homepage SoftwareApplication JSON-LD.
- BreadcrumbList data matching the visible documentation trail.
- A first-party Plex/Jellyfin migration guide linked from the homepage and docs.
- Lightweight local fonts, WebP screenshots, dimensions and lazy loading.
- Authentic creator context with explicit AI-engineering disclosure.
- CI validation of links, assets, canonicals, descriptions, and structured data.

## Measurement after deployment

Verify site ownership in Google Search Console and Bing Webmaster Tools, then
submit https://kinosail.com/sitemap.xml. These account steps are
not completed by a repository deployment. Inspect indexing for the homepage,
Docker guide, features, migration guide, and MCP guide. Review search impressions, queries,
click-through rates, and relevant Bing AI citations before choosing new content.
Start with branded searches and specific intents such as self-hosted media
server Docker, browser media player, and media server MCP. Do not claim ranking
improvements without a baseline and subsequent measurements.

The custom domain serves Pages at the origin root, so the sitemap and robots.txt
belong at that root. The sitemap can be submitted directly.

No ranking, indexing, or AI citation position is guaranteed. Neither fabricated
engagement nor mass-produced comparison pages are part of this strategy.

## Primary guidance consulted 2026-09-21

- https://developers.google.com/search/docs/appearance/ai-features
- https://developers.google.com/search/docs/appearance/structured-data/intro-structured-data
- https://www.bing.com/webmasters/help/bing-webmaster-guidelines-30fba23a
- https://www.bing.com/webmasters/help/ai-performance-9f8e7d6c
- https://developers.google.com/search/docs/fundamentals/ai-optimization-guide
- https://developers.cloudflare.com/bots/additional-configurations/managed-robots-txt/

Google applies its foundational SEO requirements to AI search experiences and
requires structured data to match visible content. Bing likewise does not
guarantee rankings or AI citations. There is no special markup that secures a
number-one position.

On 2026-09-24, the live site returned 49 sitemap URLs and mobile Lighthouse
SEO 100 for the homepage and migration guide. Cloudflare's fallback
`robots.txt` had no sitemap directive because the origin had no file. Google
Search Console was signed in but had no Kinosail property, so impressions,
indexing, and ranking changes could not be measured there yet. These are
pre-deployment observations, not evidence that this change is live.
