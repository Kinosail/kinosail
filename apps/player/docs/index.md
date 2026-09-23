---
layout: marketing
title: Self-hosted media server and web player
description: A free, self-hosted media Server with a web Player and compatible Jellyfin support. Get started with Docker.
---
<section class="hero wrap">
<h1>Your collection.<br>Your kind of evening.</h1>
<p class="lede">A free media Server that runs at home.<br class="desktop-break"> Watch in the web Player or connect a compatible Jellyfin mobile app.</p>
<div class="actions"><a class="button" href="{{ '/docs/' | relative_url }}">Read the docs</a><a class="text-link" href="{{ '/quickstart/' | relative_url }}">Get started with Docker</a><a class="text-link" href="#inside">See the Player</a></div>
<p class="fine">Run the Server for free. Keep your media on your own hardware.</p>
<figure class="hero-screen"><a href="{{ '/assets/images/player-detail.webp' | relative_url }}" aria-label="View full-size Player film screenshot"><img src="{{ '/assets/images/player-detail.webp' | relative_url }}" width="1280" height="720" alt="Kinosail Player displaying the fictional film The Last Observatory, with artwork, film details, Play and My List controls" fetchpriority="high"></a><figcaption>Real Player screenshots. Fictional library and original demo artwork. No personal media or account information.</figcaption></figure>
</section>
<section class="intro wrap" id="inside">
<h2>A place for everything<br>you love to watch.<br><span>And everything else.</span></h2>
<div class="intro-copy"><p>Keep movies, shows, music, books, comics, and photos in one library.</p><p>Use the folders you already have. Player scans them and keeps your original media read-only.</p><a class="text-link" href="{{ '/features/' | relative_url }}">Explore Player features <span aria-hidden="true">↗</span></a></div>
</section>
<section class="library-stage wrap">
<figure class="screen"><a href="{{ '/assets/images/player-library.webp' | relative_url }}" aria-label="View full-size Player library screenshot"><img src="{{ '/assets/images/player-library.webp' | relative_url }}" width="1280" height="720" loading="lazy" alt="Player home showing an entirely fictional collection of films with original illustrated posters"></a><figcaption>A small demo collection, shown in the real web Player.</figcaption></figure>
</section>
<section class="feature-lines wrap" aria-label="Everyday features">
<div><h3>Pick up where you left off.</h3><p>Resume playback, build My List, choose audio and subtitles, and find your next watch.</p><a href="{{ '/user-guide/playback/' | relative_url }}">Playback guide ↗</a></div>
<div><h3>Make room for everyone.</h3><p>Give each Viewer Profile access to the libraries and playback features it needs. Watch together with Watch Rooms.</p><a href="{{ '/user-guide/profiles/' | relative_url }}">Household profiles ↗</a></div>
<div><h3>Connect your tools.</h3><p>Connect an assistant with MCP or build on the versioned API. Choose the access each integration can use.</p><a href="{{ '/developer-guide/mcp/' | relative_url }}">Explore MCP ↗</a></div>
</section>
<section class="story-band"><div class="wrap story-inner"><h2>A personal project.<br>A useful alternative.</h2><p>Kinosail began as a way to get more control over a personal media library. It is also a real project for learning AI engineering. Human-directed. AI-engineered. Built and used by its creator.</p><a class="text-link" href="{{ '/why/' | relative_url }}">The story behind Kinosail <span aria-hidden="true">↗</span></a></div></section>
<section class="questions wrap" aria-labelledby="questions-title">
<h2 id="questions-title">Before you press play.</h2>
<details><summary>What is Kinosail Player?</summary><p>Kinosail Player is a media Server and web Player that you run on your own hardware. It uses your existing folders for movies, shows, music, audiobooks, books, comics, and photos. <a href="{{ '/features/' | relative_url }}">See the Player features.</a></p></details>
<details><summary>How do I install Kinosail with Docker?</summary><p>Follow the <a href="{{ '/quickstart/' | relative_url }}">Docker quickstart</a>. The installer checks the signed image, sets up the Server, and guides you through first setup. You do not need to build Player from source.</p></details>
<details><summary>Is Kinosail Server free?</summary><p>Yes. Kinosail Server is free to run on your own hardware. The web Player is included. You can also connect a compatible Jellyfin mobile app. This needs trusted HTTPS and enabled Jellyfin support.</p></details>
<details><summary>Can I use a Jellyfin mobile app?</summary><p>Some Jellyfin apps for iOS and Android can connect to Kinosail. Set up trusted HTTPS, then enable <strong>Allow compatible Jellyfin apps to connect</strong> in Owner Settings. <a href="{{ '/getting-started/connect-devices/' | relative_url }}">Follow the setup steps and limits.</a></p></details>
<details><summary>Is Kinosail open source?</summary><p>Kinosail is source-available under the PolyForm Perimeter license. The license has limits on commercial competition. Kinosail is not open source. <a href="https://github.com/Kinosail/kinosail/blob/main/LICENSING.md">Read the license.</a></p></details>
<details><summary>Do I need a cloud account?</summary><p>No. Local use does not need a Kinosail account. You create an Owner on your own Server. Optional external services may have their own privacy rules. <a href="{{ '/reference/architecture-and-privacy/' | relative_url }}">Read about privacy.</a></p></details>
<details><summary>Can I connect an AI assistant through MCP?</summary><p>Yes. Player provides an MCP interface for supported library and media operations, with authenticated access. <a href="{{ '/developer-guide/mcp/' | relative_url }}">The MCP guide</a> covers connection, tools, permissions, and limitations.</p></details>
</section>
<section class="start wrap"><h2>Make yourself<br>at home.</h2><div><p>Find help for your first Docker run or for running your Server day to day.</p><a class="button" href="{{ '/docs/' | relative_url }}">Explore the docs</a><p class="fine"><a href="{{ '/quickstart/' | relative_url }}">Start with Docker</a> · <a href="https://github.com/Kinosail/kinosail">Explore the source</a></p></div></section>
