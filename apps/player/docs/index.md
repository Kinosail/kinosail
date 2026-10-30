---
layout: marketing
title: "Kinosail: Free self-hosted media server and Plex alternative"
description: Kinosail is a free, self-hosted media server for movies, shows, music, audiobooks, and photos. A private Plex, Jellyfin, and Emby alternative that runs in Docker.
faq:
  - q: What is Kinosail?
    a: Kinosail Player is a free, self-hosted media server with a built-in web player. It reads the media folders you already have and organizes movies, TV shows, music, audiobooks, books, comics, and photos into one library. It runs as one Docker container on Linux, a NAS, or Proxmox.
  - q: Is Kinosail a good Plex alternative?
    a: Kinosail suits people who want Plex-style browsing without a Plex account or a paid pass. Remote streaming, hardware transcoding, and skip intro are included for free, and you can import watched history from Plex. Plex still has more store apps and live TV. <a href="/compare/plex/">Compare Kinosail and Plex</a>.
  - q: How is Kinosail different from Jellyfin?
    a: Both are free and need no account. Kinosail adds passkeys and multi-factor sign-in, a hardened single container, Watch Rooms, and an MCP server for AI assistants. Compatible Jellyfin mobile apps can connect to Kinosail, and you can import Jellyfin watched state and favorites. <a href="/compare/jellyfin/">Compare Kinosail and Jellyfin</a>.
  - q: Do I have to pay for remote streaming or hardware transcoding?
    a: No. The Kinosail Server, the web Player, remote streaming, hardware transcoding, and skip markers are free. There is no paid tier to unlock them.
  - q: How do I install Kinosail with Docker?
    a: Install Docker, choose your media folder, and start the published container with Docker Compose or one docker run command. Then open https://localhost:38127 and create the Owner. The <a href="/quickstart/">Docker quickstart</a> has copy-ready commands.
  - q: Can I watch on my TV and phone?
    a: Yes. The web Player works in modern desktop and mobile browsers. Play on another device sends video to AirPlay, Google Cast, and DLNA receivers. Compatible Jellyfin apps for iOS and Android can connect. Native Apple and Android apps, including Apple TV and Android TV, are available as source builds; store releases are not available yet. <a href="/getting-started/connect-devices/">Connect your devices</a>.
  - q: Can I bring my watch history from Plex or Jellyfin?
    a: Yes. Kinosail imports watched state and resume positions from Plex or Jellyfin into a Viewer Profile, with a preview before anything changes. You can also keep a recurring sync while you test. <a href="/owner-guide/migration/">Import your history</a>.
  - q: Does Kinosail change my media files?
    a: No. Kinosail mounts your media folder read-only. It stores its own data in separate Docker volumes, so you can run it beside Plex, Jellyfin, or Emby on the same files.
  - q: Do I need a cloud account?
    a: No. You create the Owner on your own Server. Kinosail does not operate an account service, media relay, or analytics service. <a href="/reference/architecture-and-privacy/">Read about privacy</a>.
  - q: Is Kinosail open source?
    a: Kinosail is source-available under the PolyForm Perimeter License 1.0.1. You can read, build, and run the code for free. The license limits building a competing product, so it is not OSI-approved open source. <a href="https://github.com/Kinosail/kinosail/blob/main/LICENSING.md">Read the license</a>.
  - q: Can I connect an AI assistant?
    a: Yes. Kinosail includes a Model Context Protocol (MCP) server and a versioned HTTP API. You choose the access each connection gets. <a href="/developer-guide/mcp/">Connect an MCP client</a>.
---
<section class="hero wrap">
<p class="eyebrow">Free · Self-hosted · No account</p>
<h1>Your own streaming service.<br>On hardware you own.</h1>
<p class="lede">Kinosail Player is a free, source-available media server for movies, shows, music, audiobooks, books, and photos.<br class="desktop-break"> A private alternative to Plex, Jellyfin, and Emby that runs in one Docker container.</p>
<div class="actions"><a class="button" href="{{ '/quickstart/' | relative_url }}">Get started with Docker</a><a class="button button-quiet" href="https://github.com/Kinosail/kinosail">View source on GitHub</a><a class="text-link" href="#inside">See screenshots</a><a class="text-link" href="{{ '/docs/' | relative_url }}">Read the docs</a></div>
<p class="fine">No Plex Pass. No Premiere. No subscription for remote streaming or hardware transcoding.</p>
<figure class="hero-screen"><picture><source media="(max-width: 760px)" srcset="{{ '/assets/images/player-home-mobile.webp' | relative_url }}"><img src="{{ '/assets/images/player-library.webp' | relative_url }}" srcset="{{ '/assets/images/player-library-688.webp' | relative_url }} 688w, {{ '/assets/images/player-library-1024.webp' | relative_url }} 1024w, {{ '/assets/images/player-library.webp' | relative_url }} 1280w, {{ '/assets/images/player-library-2880.webp' | relative_url }} 2880w" sizes="(max-width: 760px) calc(100vw - 50px), (max-width: 1280px) calc(100vw - 98px), 1182px" width="1280" height="800" alt="Kinosail Player Home with a featured fictional film, Resume button, and a Continue watching row" fetchpriority="high"></picture><figcaption>The real web Player with a fictional demo library. <a href="{{ '/assets/images/player-library-2880.webp' | relative_url }}">View desktop image</a> · <a href="{{ '/assets/images/player-home-mobile.webp' | relative_url }}">View phone image</a></figcaption></figure>
</section>
<section class="stats wrap" aria-label="Kinosail at a glance">
<div><strong>$0</strong><span>Server, web Player, remote streaming, and hardware transcoding</span></div>
<div><strong>7</strong><span>media types: movies, shows, music, audiobooks, books, comics, photos</span></div>
<div><strong>1</strong><span>container with the web app, API, and database</span></div>
<div><strong>0</strong><span>vendor accounts, relays, or analytics services</span></div>
</section>
<section class="quick wrap" id="quick-start" aria-labelledby="quick-title">
<div class="quick-copy">
<p class="eyebrow">Quick start</p>
<h2 id="quick-title">Running in five minutes.</h2>
<ol class="quick-steps">
<li><strong>Install Docker</strong> on 64-bit Linux (amd64 or arm64), a NAS, or Proxmox. Docker Desktop on macOS works for local use.</li>
<li><strong>Start Kinosail</strong> with Docker Compose or <code>docker run</code>. Change the media path to your folder. It is mounted read-only, so your files stay untouched.</li>
<li><strong>Open <code>https://localhost:38127</code></strong>, create the Owner, add a passkey, and add <code>/media/Movies</code> as a library.</li>
</ol>
<p class="fine">Prefer a NAS app or a signed, pinned install? <a href="{{ '/quickstart/' | relative_url }}">Docker quickstart</a> · <a href="{{ '/getting-started/platforms/' | relative_url }}">NAS, Unraid, TrueNAS, Proxmox</a></p>
</div>
<div class="terminal" role="region" aria-label="Install commands">
<input class="tab-input" type="radio" name="install-method" id="install-compose" checked>
<input class="tab-input" type="radio" name="install-method" id="install-run">
<div class="terminal-bar"><span></span><span></span><span></span><label for="install-compose">Docker Compose</label><label for="install-run">docker run</label></div>
<div class="pane pane-compose">
<p class="pane-note">Save as <code>compose.yaml</code>, set your media path, then run <code>docker compose up -d</code>.</p>
<pre><code>name: kinosail

services:
  kinosail:
    image: ghcr.io/kinosail/kinosail-player:latest
    container_name: kinosail
    restart: unless-stopped
    init: true
    user: "10001:10001"
    read_only: true
    cap_drop: [ALL]
    security_opt:
      - no-new-privileges:true
    ports:
      - "127.0.0.1:38127:38127"
    environment:
      KINOSAIL_DATA_DIR: /config
      KINOSAIL_CACHE_DIR: /cache
      KINOSAIL_MEDIA_DIR: /media
    volumes:
      - config:/config
      - cache:/cache
      - backups:/backups
      - /path/to/your/media:/media:ro
    tmpfs:
      - /tmp:rw,noexec,nosuid,nodev,size=256m
      - /run:rw,noexec,nosuid,nodev,size=16m

volumes:
  config:
  cache:
  backups:</code></pre>
<pre class="pane-command"><code>docker compose up -d</code></pre>
</div>
<div class="pane pane-run">
<p class="pane-note">Set your media path, then run the command.</p>
<pre><code>export KINOSAIL_MEDIA_PATH="/absolute/path/to/your/media"

docker run --detach --name kinosail \
  --restart unless-stopped --init \
  --user 10001:10001 --read-only \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  --publish 127.0.0.1:38127:38127 \
  --mount type=volume,source=kinosail-config,target=/config \
  --mount type=volume,source=kinosail-cache,target=/cache \
  --mount type=volume,source=kinosail-backups,target=/backups \
  --mount "type=bind,source=${KINOSAIL_MEDIA_PATH},target=/media,readonly" \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=256m \
  --tmpfs /run:rw,noexec,nosuid,nodev,size=16m \
  --env KINOSAIL_DATA_DIR=/config \
  --env KINOSAIL_CACHE_DIR=/cache \
  --env KINOSAIL_MEDIA_DIR=/media \
  ghcr.io/kinosail/kinosail-player:latest</code></pre>
</div>
</div>
</section>
<section class="screens wrap" id="inside" aria-labelledby="screens-title">
<p class="eyebrow">See it running</p>
<h2 id="screens-title">Real screens. Fictional films.</h2>
<p class="section-lede">Every image is the actual Kinosail Player on a test Server. The films, posters, plots, and videos were made up for the demo.</p>
<div class="gallery">
<figure class="shot shot-wide"><a href="{{ '/assets/images/player-movies-2880.webp' | relative_url }}"><img src="{{ '/assets/images/player-movies.webp' | relative_url }}" srcset="{{ '/assets/images/player-movies-688.webp' | relative_url }} 688w, {{ '/assets/images/player-movies-1024.webp' | relative_url }} 1024w, {{ '/assets/images/player-movies.webp' | relative_url }} 1280w, {{ '/assets/images/player-movies-2880.webp' | relative_url }} 2880w" sizes="(max-width: 760px) calc(100vw - 48px), 1182px" width="1280" height="800" loading="lazy" alt="Movies library with twelve original posters, an A to Z jump bar, and sort controls"></a><figcaption>Browse every library with posters, sorting, and an A–Z jump bar.</figcaption></figure>
<figure class="shot"><a href="{{ '/assets/images/player-watch-2880.webp' | relative_url }}"><img src="{{ '/assets/images/player-watch.webp' | relative_url }}" srcset="{{ '/assets/images/player-watch-688.webp' | relative_url }} 688w, {{ '/assets/images/player-watch-1024.webp' | relative_url }} 1024w, {{ '/assets/images/player-watch.webp' | relative_url }} 1280w" sizes="(max-width: 760px) calc(100vw - 48px), 580px" width="1280" height="800" loading="lazy" alt="Video playing with subtitles, Direct Play badge, and Play on another device button"></a><figcaption>Direct Play with subtitles, casting, and picture-in-picture.</figcaption></figure>
<figure class="shot"><a href="{{ '/assets/images/player-show-2880.webp' | relative_url }}"><img src="{{ '/assets/images/player-show.webp' | relative_url }}" srcset="{{ '/assets/images/player-show-688.webp' | relative_url }} 688w, {{ '/assets/images/player-show-1024.webp' | relative_url }} 1024w, {{ '/assets/images/player-show.webp' | relative_url }} 1280w" sizes="(max-width: 760px) calc(100vw - 48px), 580px" width="1280" height="800" loading="lazy" alt="TV show page with Play next, seasons, and an episode list"></a><figcaption>Shows know what to play next.</figcaption></figure>
<figure class="shot"><a href="{{ '/assets/images/player-detail-2880.webp' | relative_url }}"><img src="{{ '/assets/images/player-detail.webp' | relative_url }}" srcset="{{ '/assets/images/player-detail-688.webp' | relative_url }} 688w, {{ '/assets/images/player-detail-1024.webp' | relative_url }} 1024w, {{ '/assets/images/player-detail.webp' | relative_url }} 1280w" sizes="(max-width: 760px) calc(100vw - 48px), 580px" width="1280" height="800" loading="lazy" alt="Film page with backdrop, rating, plot, director, Add to My List, and Mark watched"></a><figcaption>Every title gets its own page with details and controls.</figcaption></figure>
<figure class="shot shot-phones"><div class="phones"><img src="{{ '/assets/images/player-home-mobile.webp' | relative_url }}" width="390" height="844" loading="lazy" alt="Kinosail web Player Home on a phone"><img src="{{ '/assets/images/player-watch-mobile.webp' | relative_url }}" width="390" height="844" loading="lazy" alt="Video playing in the web Player on a phone"><img src="{{ '/assets/images/android-phone-home.webp' | relative_url }}" width="390" height="832" loading="lazy" alt="Kinosail Android app Home with Continue watching"></div><figcaption>The web Player on a phone, and the Android app.</figcaption></figure>
</div>
</section>
<section class="features wrap" id="features" aria-labelledby="features-title">
<p class="eyebrow">Everything included</p>
<h2 id="features-title">The features other servers charge for.</h2>
<div class="feature-grid">
<div><h3>Every kind of media</h3><p>Movies, TV shows, music, audiobooks, books, comics, and photos share one library and one search.</p><a href="{{ '/user-guide/media-types/' | relative_url }}">Media types ↗</a></div>
<div><h3>Direct Play first</h3><p>Kinosail sends the original file when the device can play it. It converts only when a device needs it.</p><a href="{{ '/user-guide/playback/' | relative_url }}">Playback guide ↗</a></div>
<div><h3>Free hardware transcoding</h3><p>Use Intel Quick Sync, NVIDIA, AMD, VA-API, Apple VideoToolbox, or Rockchip. HDR can be adjusted for non-HDR screens.</p><a href="{{ '/owner-guide/playback/' | relative_url }}">Configure playback ↗</a></div>
<div><h3>Watch away from home</h3><p>Stream over your own HTTPS address. There is no relay, no remote pass, and no third-party account.</p><a href="{{ '/owner-guide/remote-access/' | relative_url }}">Remote access ↗</a></div>
<div><h3>Skip intros and credits</h3><p>Kinosail finds intros, outros, and credits after each scan. Add or correct markers yourself.</p><a href="{{ '/owner-guide/metadata/' | relative_url }}">Skip markers ↗</a></div>
<div><h3>A profile for each person</h3><p>Viewer Profiles keep progress separate and limit libraries. Owners sign in with passkeys and multi-factor authentication.</p><a href="{{ '/user-guide/profiles/' | relative_url }}">Profiles ↗</a></div>
<div><h3>Watch together</h3><p>Watch Rooms keep play, pause, and seek in step for everyone in the room.</p><a href="{{ '/user-guide/sharing/' | relative_url }}">Watch Rooms ↗</a></div>
<div><h3>Play on any screen</h3><p>Send video to AirPlay, Google Cast, and DLNA receivers, or connect a compatible Jellyfin app.</p><a href="{{ '/getting-started/connect-devices/' | relative_url }}">Connect devices ↗</a></div>
<div><h3>API and MCP</h3><p>Automate with a versioned HTTP API, or let an AI assistant search and play through MCP.</p><a href="{{ '/developer-guide/mcp/' | relative_url }}">MCP guide ↗</a></div>
</div>
</section>
<section class="compare wrap" id="compare" aria-labelledby="compare-title">
<p class="eyebrow">Compare</p>
<h2 id="compare-title">How Kinosail compares with Plex, Jellyfin, and Emby.</h2>
<div class="table-scroll" role="region" aria-labelledby="compare-title" tabindex="0">
<table>
<thead><tr><th scope="col">Feature</th><th scope="col">Kinosail</th><th scope="col">Plex</th><th scope="col">Jellyfin</th><th scope="col">Emby</th></tr></thead>
<tbody>
<tr><th scope="row">Server price</th><td class="yes">Free</td><td>Free, with paid Plex Pass</td><td class="yes">Free</td><td>Free, with paid Premiere</td></tr>
<tr><th scope="row">Remote streaming of your media</th><td class="yes">Free</td><td>Plex Pass or Remote Watch Pass</td><td class="yes">Free</td><td class="yes">Free</td></tr>
<tr><th scope="row">Hardware transcoding</th><td class="yes">Free</td><td>Plex Pass</td><td class="yes">Free</td><td>Premiere</td></tr>
<tr><th scope="row">Skip intro and credits</th><td class="yes">Free</td><td>Plex Pass</td><td>Plugins</td><td>Premiere</td></tr>
<tr><th scope="row">Vendor account required</th><td class="yes">No</td><td>Yes</td><td class="yes">No</td><td class="yes">No</td></tr>
<tr><th scope="row">TV apps in app stores</th><td>Not yet</td><td class="yes">Yes</td><td class="yes">Yes</td><td class="yes">Yes</td></tr>
<tr><th scope="row">Live TV and DVR</th><td>No</td><td class="yes">Yes</td><td class="yes">Yes</td><td>Premiere</td></tr>
<tr><th scope="row">Lifetime price</th><td class="yes">$0</td><td>$749.99 Plex Pass</td><td class="yes">$0</td><td>$119 Premiere</td></tr>
<tr><th scope="row">License</th><td>Source-available</td><td>Proprietary</td><td>Open source (GPL-2.0)</td><td>Proprietary</td></tr>
</tbody>
</table>
</div>
<p class="fine">Checked October 26, 2026, against each vendor's own pages. Prices are in US dollars. Each comparison page lists its sources.</p>
<div class="compare-links"><a href="{{ '/compare/plex/' | relative_url }}"><strong>Kinosail vs Plex</strong><span>Keep the polish, drop the pass.</span></a><a href="{{ '/compare/jellyfin/' | relative_url }}"><strong>Kinosail vs Jellyfin</strong><span>Free on both sides. Different priorities.</span></a><a href="{{ '/compare/emby/' | relative_url }}"><strong>Kinosail vs Emby</strong><span>No Premiere needed for the essentials.</span></a></div>
</section>
<section class="switch wrap" aria-labelledby="switch-title">
<h2 id="switch-title">Switch without starting over.</h2>
<div class="intro-copy"><p>Run Kinosail beside your current server on the same folders. Your media stays read-only, so nothing changes until you decide.</p><p>Import watched state and resume positions from Plex or Jellyfin. Preview every match first, then keep a recurring sync while your household tries it.</p><a class="text-link" href="{{ '/owner-guide/migration/' | relative_url }}">Import your history <span aria-hidden="true">↗</span></a></div>
</section>
<section class="story-band"><div class="wrap story-inner"><h2>Why I built Kinosail.</h2><p>I wanted more control over my personal media library. I built Kinosail for that, and I use the project to learn AI engineering. AI tools help me write and test the code.</p><a class="text-link" href="{{ '/why/' | relative_url }}">Read the project story <span aria-hidden="true">↗</span></a></div></section>
<section class="questions wrap" id="questions" aria-labelledby="questions-title">
<h2 id="questions-title">Common questions.</h2>
{% include faq.html %}
<p><a class="text-link" href="{{ '/faq/' | relative_url }}">Read all frequently asked questions <span aria-hidden="true">↗</span></a></p>
</section>
<section class="start wrap subtitles-intro" aria-labelledby="subtitles-title"><h2 id="subtitles-title">Need subtitles beside your videos?</h2><div><p>Kinosail Subtitles is a separate, free server that checks movie and episode coverage, finds matches from configured providers, and saves validated sidecar files on your own hardware. It is a self-hosted alternative to Bazarr.</p><a class="button" href="{{ '/subtitles/' | relative_url }}">Explore Kinosail Subtitles</a><p class="fine">Its media mount is writable so it can save subtitle files. Player keeps its media mount read-only.</p></div></section>
<section class="start wrap"><h2>Ready to install?</h2><div><p>Use the Docker quickstart, or <a href="{{ '/getting-started/platforms/' | relative_url }}">install on a NAS or Proxmox</a> with a prepared container file.</p><a class="button" href="{{ '/quickstart/' | relative_url }}">Install with Docker</a><p class="fine"><a href="{{ '/docs/' | relative_url }}">Read the docs</a> · <a href="https://github.com/Kinosail/kinosail">View the source</a></p></div></section>
