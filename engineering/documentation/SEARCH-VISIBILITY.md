# Search and AI discovery

The primary audience is people who search for a self-hosted media server, or
for an alternative to Plex, Jellyfin, or Emby. The homepage defines Kinosail
Player in its first sentence, shows real screenshots, gives a copy-ready Docker
command, and compares Kinosail with those servers. Existing guide URLs remain
stable.

## Strategy (2026-10-26)

- **Answer first.** Each page states what Kinosail is, or how it compares, in
  its first paragraph. Search engines and AI answer engines quote these
  passages directly.
- **Comparison intent.** `/compare/`, `/compare/plex/`, `/compare/jellyfin/`,
  and `/compare/emby/` target "Plex alternative", "Jellyfin alternative",
  "Emby alternative", and "Plex vs Jellyfin vs Emby" searches. Each page has
  a dated feature table, honest "who should stay with" guidance, switching
  steps, an FAQ, and links to the vendor's own pages.
- **Structured data matches visible content.** FAQ answers come from one
  `faq` front-matter list. `_includes/faq.html` renders the visible answers,
  and `_includes/seo.html` renders the same list as FAQPage JSON-LD. The
  homepage SoftwareApplication lists features, screenshots, price, and help.
- **AI crawlers are welcome.** The generated `robots.txt` allows all agents and
  names OpenAI, Anthropic, Perplexity, Google-Extended, Applebot-Extended,
  Bing, and Common Crawl agents explicitly. `/llms.txt` gives a short, linked
  summary in the llms.txt format.
- **Real product proof.** Screenshots come from a running Server with a
  fictional library. See [DEMO-ASSETS.md](DEMO-ASSETS.md).
- **Internal links.** The masthead links to Compare. The footer links to each
  comparison page with descriptive anchor text.

## Keep comparisons honest

Comparison pages make claims about other products. Before you change one:

1. Check each claim against the vendor's own page and update the date.
2. Keep each "where the other server is stronger" section. It is part of what
   makes the page useful and credible.
3. Do not invent ratings, reviews, or user counts.
4. Recheck prices when a vendor announces a change. Plex raised the Lifetime
   Plex Pass to $749.99 on 2026-07-01.

No ranking, indexing, or AI citation position is guaranteed.

## Implemented

- Static HTML content and ordinary crawlable links, including FAQ answers.
- Descriptive titles and descriptions, canonical URLs, XML sitemap, and a
  generated `robots.txt` that names the sitemap for the build's public URL.
- Organization, WebSite, WebPage, SoftwareApplication, FAQPage, and
  BreadcrumbList JSON-LD.
- `/llms.txt`, excluded from the sitemap and search index.
- Lightweight local fonts, WebP screenshots, dimensions, and lazy loading.
- CI validation of links, assets, canonicals, descriptions, structured data,
  FAQ parity, AI crawler rules, and llms.txt links.

## Account steps outside the repository

These steps need account access. A deployment does not complete them.

1. Verify the site in Google Search Console and Bing Webmaster Tools. Submit
   https://kinosail.com/sitemap.xml.
2. In Cloudflare, keep **Block AI bots** and managed `robots.txt` off for
   kinosail.com, or the origin `robots.txt` and AI access are overridden.
3. Ask for listings where the license allows it. awesome-selfhosted lists only
   free software, so Kinosail does not qualify. Self-hosting newsletters,
   r/selfhosted, AlternativeTo, and selfh.st are better places.
4. Review search impressions, queries, click-through rates, and Bing AI
   citations after 4 weeks. Do not claim ranking improvements without a
   baseline and later measurements.

The public IndexNow ownership file is
`apps/player/docs/188c30dcfde0cb99c9697f2a115d4230.txt`. After a docs
deployment, the post-deploy CI job notifies IndexNow of changed public page URLs.
Shared layout, navigation, or SEO template changes notify all public pages.

## Guidance consulted

- https://developers.google.com/search/docs/appearance/ai-features
- https://developers.google.com/search/docs/appearance/structured-data/intro-structured-data
- https://developers.google.com/search/docs/fundamentals/creating-helpful-content
- https://www.bing.com/webmasters/help/bing-webmaster-guidelines-30fba23a
- https://platform.openai.com/docs/bots
- https://support.anthropic.com/en/articles/8896518-does-anthropic-crawl-data-from-the-web-and-how-can-site-owners-block-the-crawler
- https://docs.perplexity.ai/guides/bots
- https://llmstxt.org/
- https://developers.cloudflare.com/bots/additional-configurations/managed-robots-txt/
- https://www.bing.com/indexnow/getstarted
