---
title: Download Kinosail
description: Download Kinosail for your computer. It installs like a normal app, runs in the background, and starts by itself.
section: Start here
last_reviewed: 2026-11-22
---

# Download Kinosail

Install Kinosail on the computer that holds your movies and shows. It runs in the background and starts by itself. Then you watch on your TV, phone, tablet, or any browser at home.

<section class="download-hero" aria-labelledby="download-hero-title" data-download>
  <h2 id="download-hero-title" data-download-title>Choose your computer</h2>
  <p class="download-hero-text" data-download-text>Pick your computer from the list below.</p>
  <p class="download-actions" data-download-primary hidden><a class="install-primary download-button" data-download-button href="#all-downloads">Download Kinosail</a></p>
  <p class="download-hero-small" data-download-small hidden></p>
  <div class="download-command" data-download-command hidden>
    <p><strong>Easiest on Linux:</strong> open a terminal, paste this, and press Enter. It picks the right file for you.</p>
    <div class="download-command-row"><code>curl -fsSL https://kinosail.com/install.sh | sh</code><button type="button" data-download-copy>Copy</button></div>
  </div>
  <p class="download-hero-other"><a href="#all-downloads">Not your computer? See all downloads</a></p>
</section>
<script defer src="{{ '/assets/js/download.js' | relative_url }}"></script>

## After you download

1. **Install it.** Open the file you downloaded. On Linux, use the terminal command above, or run `sudo apt install ./Kinosail-Linux-x64.deb` in your Downloads folder.
2. **Open Kinosail.** On the same computer, go to `https://localhost:38127`. The first time, your browser warns about the certificate. This is normal for a new server. Choose to continue.
3. **Follow the setup.** Create your account, pick your movie and show folders, and turn on **Watch on your TV and phone**.

Kinosail keeps itself running. It starts again when the computer restarts.

## All downloads {#all-downloads}

Every link below always gives you the newest version.

<div class="download-options">
  <section class="download-option" aria-labelledby="download-windows">
    <h3 id="download-windows">Windows</h3>
    <p class="download-option-status">Coming soon</p>
    <p>Until then, use <a href="{{ '/quickstart/' | relative_url }}">Docker Desktop</a>.</p>
  </section>
  <section class="download-option" aria-labelledby="download-mac">
    <h3 id="download-mac">Mac</h3>
    <p class="download-option-status">Coming soon</p>
    <p>Until then, use <a href="{{ '/quickstart/' | relative_url }}">Docker Desktop</a>.</p>
  </section>
  <section class="download-option download-option-wide" aria-labelledby="download-linux">
    <h3 id="download-linux">Linux</h3>
    <p>For 64-bit Linux with systemd, including Raspberry Pi 4 and 5.</p>
    <table>
      <thead><tr><th scope="col">Your Linux</th><th scope="col">Intel or AMD</th><th scope="col">ARM (Raspberry Pi)</th></tr></thead>
      <tbody>
        <tr><th scope="row">Ubuntu, Debian, Linux Mint, Pop!_OS, Raspberry Pi OS</th><td><a data-download-file="linux-x64-deb" href="https://github.com/Kinosail/kinosail/releases/latest/download/Kinosail-Linux-x64.deb">Download .deb</a></td><td><a data-download-file="linux-arm64-deb" href="https://github.com/Kinosail/kinosail/releases/latest/download/Kinosail-Linux-arm64.deb">Download .deb</a></td></tr>
        <tr><th scope="row">Fedora, RHEL, Rocky, AlmaLinux, openSUSE</th><td><a data-download-file="linux-x64-rpm" href="https://github.com/Kinosail/kinosail/releases/latest/download/Kinosail-Linux-x64.rpm">Download .rpm</a></td><td><a data-download-file="linux-arm64-rpm" href="https://github.com/Kinosail/kinosail/releases/latest/download/Kinosail-Linux-arm64.rpm">Download .rpm</a></td></tr>
      </tbody>
    </table>
    <p>Or paste this in a terminal. It picks the right file and checks it before it installs:</p>
    <pre><code>curl -fsSL https://kinosail.com/install.sh | sh</code></pre>
  </section>
  <section class="download-option download-option-wide" aria-labelledby="download-server">
    <h3 id="download-server">NAS, Unraid, Proxmox, or Docker</h3>
    <p>Already run a home server? Use the container instead.</p>
    <p><a href="{{ '/getting-started/platforms/' | relative_url }}">Install on a NAS or Proxmox</a> · <a href="{{ '/quickstart/' | relative_url }}">Install with Docker</a></p>
  </section>
</div>

<details class="download-help">
<summary>Not sure which Linux file you need?</summary>
<ul>
<li><strong>Intel or AMD, or ARM?</strong> Run <code>uname -m</code> in a terminal. <code>x86_64</code> means Intel or AMD. <code>aarch64</code> means ARM. A Raspberry Pi is ARM.</li>
<li><strong>.deb or .rpm?</strong> Ubuntu, Debian, Mint, and Raspberry Pi OS use <code>.deb</code>. Fedora, RHEL, and openSUSE use <code>.rpm</code>.</li>
<li><strong>Still unsure?</strong> Use the terminal command. It chooses for you.</li>
</ul>
</details>

## Remove Kinosail

On Linux, run `sudo apt remove kinosail` or `sudo dnf remove kinosail`. Your profiles and watch history stay in `/var/lib/kinosail` until you delete that folder. Kinosail never changes or deletes your movie files.

## Is this download safe?

GitHub Actions builds every file from the public source code. Each release has a `SHA256SUMS` file with a [Sigstore](https://www.sigstore.dev/) signature. The signature proves that the Kinosail release workflow made the files. The terminal command checks the file before it installs anything. [Every release](https://github.com/Kinosail/kinosail/releases) lists its files and signatures.
