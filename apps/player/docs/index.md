---
layout: marketing
title: Self-hosted media server and web player
description: A free, self-hosted media server for your browser, with compatible Jellyfin mobile apps. Install Kinosail with Docker.
---
<section class="hero wrap">
<h1>Your collection.<br>Your kind of evening.</h1>
<p class="lede">A free, self-hosted media server for your browser.<br class="desktop-break"> Use the web Player or connect a compatible Jellyfin mobile app.</p>
<div class="actions"><a class="button" href="{{ '/docs/' | relative_url }}">Read the docs</a><a class="text-link" href="{{ '/quickstart/' | relative_url }}">Get started with Docker</a><a class="text-link" href="#inside">See the Player</a></div>
<p class="fine">The Server is free to run; Kinosail mobile apps may be sold separately.</p>
<figure class="hero-screen"><a href="{{ '/assets/images/player-detail.webp' | relative_url }}" aria-label="View full-size Player film screenshot"><img src="{{ '/assets/images/player-detail.webp' | relative_url }}" width="1280" height="720" alt="Kinosail Player displaying the fictional film The Last Observatory, with artwork, film details, Play and My List controls" fetchpriority="high"></a><figcaption>Real Player screenshots. Fictional library and original demo artwork. No personal media or account information.</figcaption></figure>
</section>
<section class="intro wrap" id="inside">
<h2>A place for everything<br>you love to watch.<br><span>And everything else.</span></h2>
<div class="intro-copy"><p>Movies and shows are only the beginning. Keep music, audiobooks, books, comics, and photos together in one browser-based library.</p><p>Use the folders you already have. Player scans your collection and keeps your original media mounted read-only.</p><a class="text-link" href="{{ '/features/' | relative_url }}">Explore every Player feature <span aria-hidden="true">↗</span></a></div>
</section>
<section class="library-stage wrap">
<figure class="screen"><a href="{{ '/assets/images/player-library.webp' | relative_url }}" aria-label="View full-size Player library screenshot"><img src="{{ '/assets/images/player-library.webp' | relative_url }}" width="1280" height="720" loading="lazy" alt="Player home showing an entirely fictional collection of films with original illustrated posters"></a><figcaption>A small demo collection, shown in the real web Player.</figcaption></figure>
</section>
<section class="feature-lines wrap" aria-label="Everyday features">
<div><h3>Pick up where you left off.</h3><p>Resume playback, build My List, choose audio and subtitles, and find your next watch.</p><a href="{{ '/user-guide/playback/' | relative_url }}">Playback guide ↗</a></div>
<div><h3>Make room for everyone.</h3><p>Give each Viewer Profile its own library access and playback permissions. Watch together with Watch Rooms.</p><a href="{{ '/user-guide/profiles/' | relative_url }}">Household profiles ↗</a></div>
<div><h3>Let your tools join in.</h3><p>Connect an assistant through MCP or build on the versioned API. Give integrations the access you choose.</p><a href="{{ '/developer-guide/mcp/' | relative_url }}">Explore MCP ↗</a></div>
</section>
<section class="story-band"><div class="wrap story-inner"><h2>A personal project.<br>A useful alternative.</h2><p>Kinosail began with a wish for more control over a personal media library—and a real project to learn AI engineering through. Human-directed. AI-engineered. Built and used by its creator.</p><a class="text-link" href="{{ '/why/' | relative_url }}">The story behind Kinosail <span aria-hidden="true">↗</span></a></div></section>
<section class="questions wrap" aria-labelledby="questions-title">
<h2 id="questions-title">Before you press play.</h2>
<details><summary>What is Kinosail Player?</summary><p>Kinosail Player is a self-hosted media server and browser-based player for your own movies, shows, music, audiobooks, books, comics, and photos. It runs on your hardware and uses your existing media folders. <a href="{{ '/features/' | relative_url }}">See the complete feature guide.</a></p></details>
<details><summary>How do I install Kinosail with Docker?</summary><p>Use the <a href="{{ '/quickstart/' | relative_url }}">Docker quickstart</a>. The installer verifies the signed public image at <code>ghcr.io/kinosail/kinosail-player:latest</code>, prepares the deployment, and guides you toward first setup. You do not need to build Player from source.</p></details>
<details><summary>Is Kinosail Server free?</summary><p>Yes. Kinosail Server is free to run on your own hardware, and the web Player is included. Kinosail mobile apps may have a separate purchase price. You can also connect a compatible Jellyfin mobile app after enabling Jellyfin support over trusted HTTPS.</p></details>
<details><summary>Can I use my Jellyfin mobile app?</summary><p>Compatible Jellyfin apps for iOS and Android can connect to Kinosail. Set up trusted HTTPS, then enable <strong>Allow compatible Jellyfin apps to connect</strong> in Owner Settings. <a href="{{ '/getting-started/connect-devices/' | relative_url }}">Connect a Jellyfin app</a> for the steps and limitations.</p></details>
<details><summary>Is Kinosail open source?</summary><p>Kinosail is source-available under the PolyForm Perimeter license, which includes a commercial-competition boundary. It is not described as open source. <a href="https://github.com/Kinosail/kinosail/blob/main/LICENSING.md">Read the license.</a></p></details>
<details><summary>Do I need a cloud account?</summary><p>Local use does not require a Kinosail-hosted account. You create an Owner on your own Server. Optional external services have separate privacy implications. <a href="{{ '/reference/architecture-and-privacy/' | relative_url }}">Read about architecture and privacy.</a></p></details>
<details><summary>Can I connect an AI assistant through MCP?</summary><p>Yes. Player provides an MCP interface for supported library and media operations, with authenticated access. <a href="{{ '/developer-guide/mcp/' | relative_url }}">The MCP guide</a> covers connection, tools, permissions, and limitations.</p></details>
</section>
<section class="start wrap"><h2>Make yourself<br>at home.</h2><div><p>Find the answer you need, from a first Docker run to the details of running your own Server.</p><a class="button" href="{{ '/docs/' | relative_url }}">Explore the docs</a><p class="fine"><a href="{{ '/quickstart/' | relative_url }}">Start with Docker</a> · <a href="https://github.com/Kinosail/kinosail">Explore the source</a></p></div></section>
