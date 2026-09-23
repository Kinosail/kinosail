---
layout: marketing
title: Free self-hosted media server and web player
description: Kinosail Player is a free media server for your own hardware. It includes a web player, reads your existing media folders, and installs with Docker.
---
<section class="hero wrap">
<h1>A free media server<br>for your home.</h1>
<p class="lede">Kinosail Player runs on your own hardware, reads your existing media folders, and includes a web player.</p>
<div class="actions"><a class="button" href="{{ '/docs/' | relative_url }}">Read the docs</a><a class="text-link" href="{{ '/quickstart/' | relative_url }}">Install with Docker</a><a class="text-link" href="#inside">See the Player</a></div>
<p class="fine">The server is free to run. Your media stays on your hardware.</p>
<figure class="hero-screen"><a href="{{ '/assets/images/player-detail.webp' | relative_url }}" aria-label="View full-size Player film screenshot"><img src="{{ '/assets/images/player-detail.webp' | relative_url }}" width="1280" height="720" alt="Kinosail Player displaying the fictional film The Last Observatory, with artwork, film details, Play and My List controls" fetchpriority="high"></a><figcaption>Real Player screenshots. Fictional library and original demo artwork. No personal media or account information.</figcaption></figure>
</section>
<section class="intro wrap" id="inside">
<h2>Use the folders<br>you already have.</h2>
<div class="intro-copy"><p>Kinosail reads your existing folders and brings movies, shows, music, audiobooks, books, comics, and photos into one library.</p><p>Your original media files stay read-only.</p><a class="text-link" href="{{ '/features/' | relative_url }}">See all Player features <span aria-hidden="true">↗</span></a></div>
</section>
<section class="library-stage wrap">
<figure class="screen"><a href="{{ '/assets/images/player-library.webp' | relative_url }}" aria-label="View full-size Player library screenshot"><img src="{{ '/assets/images/player-library.webp' | relative_url }}" width="1280" height="720" loading="lazy" alt="Player home showing an entirely fictional collection of films with original illustrated posters"></a><figcaption>A small demo collection, shown in the real web Player.</figcaption></figure>
</section>
<section class="feature-lines wrap" aria-label="Everyday features">
<div><h3>Resume where you stopped.</h3><p>Continue playback, choose audio and subtitles, and add titles to My List.</p><a href="{{ '/user-guide/playback/' | relative_url }}">Read the playback guide ↗</a></div>
<div><h3>Set up a profile for each person.</h3><p>Choose which libraries each Viewer Profile can use. Start a Watch Room to watch together.</p><a href="{{ '/user-guide/profiles/' | relative_url }}">Set up profiles ↗</a></div>
<div><h3>Connect with an API or MCP.</h3><p>Build an integration or connect an assistant. Set the access allowed for each connection.</p><a href="{{ '/developer-guide/mcp/' | relative_url }}">Connect an MCP client ↗</a></div>
</section>
<section class="story-band"><div class="wrap story-inner"><h2>Why I built Kinosail.</h2><p>I wanted more control over my personal media library. I built Kinosail for that, and I use the project to learn AI engineering. AI tools help me write and test the code.</p><a class="text-link" href="{{ '/why/' | relative_url }}">Read the project story <span aria-hidden="true">↗</span></a></div></section>
<section class="questions wrap" aria-labelledby="questions-title">
<h2 id="questions-title">Common questions.</h2>
<details><summary>What is Kinosail Player?</summary><p>Kinosail Player is a media server and web player that runs on your own hardware. It reads your folders for movies, shows, music, audiobooks, books, comics, and photos. <a href="{{ '/features/' | relative_url }}">See all Player features.</a></p></details>
<details><summary>How do I install Kinosail with Docker?</summary><p>Copy a <a href="{{ '/quickstart/' | relative_url }}">Docker or Docker Compose example</a> from the quickstart. It installs the server and guides you through first setup.</p></details>
<details><summary>Is Kinosail free?</summary><p>Yes. Kinosail Server is free to run on your own hardware, and the web player is included.</p></details>
<details><summary>Can I use a Jellyfin mobile app?</summary><p>Some Jellyfin apps for iOS and Android can connect to Kinosail. Set up trusted HTTPS, then enable <strong>Allow compatible Jellyfin apps to connect</strong> in Owner Settings. <a href="{{ '/getting-started/connect-devices/' | relative_url }}">Follow the setup steps and limits.</a></p></details>
<details><summary>Is Kinosail open source?</summary><p>Kinosail is source-available under the PolyForm Perimeter license. The license has limits on commercial competition. Kinosail is not open source. <a href="https://github.com/Kinosail/kinosail/blob/main/LICENSING.md">Read the license.</a></p></details>
<details><summary>Do I need a cloud account?</summary><p>No. Local use does not need a Kinosail account. You create an Owner on your own Server. Optional external services may have their own privacy rules. <a href="{{ '/reference/architecture-and-privacy/' | relative_url }}">Read about privacy.</a></p></details>
<details><summary>Can I connect an AI assistant through MCP?</summary><p>Yes. Player provides an MCP interface for supported library and media operations, with authenticated access. <a href="{{ '/developer-guide/mcp/' | relative_url }}">The MCP guide</a> covers connection, tools, permissions, and limitations.</p></details>
</section>
<section class="start wrap"><h2>Ready to install?</h2><div><p>Use the Docker quickstart or read the full guide.</p><a class="button" href="{{ '/quickstart/' | relative_url }}">Install with Docker</a><p class="fine"><a href="{{ '/docs/' | relative_url }}">Read the docs</a> · <a href="https://github.com/Kinosail/kinosail">View the source</a></p></div></section>
