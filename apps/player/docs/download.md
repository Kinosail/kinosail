---
title: Download Kinosail
description: Download Kinosail for your computer. It installs like a normal app, runs in the background, and starts by itself.
section: Start here
last_reviewed: 2026-11-23
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

1. **Install it.**
   - **Windows:** open `Kinosail-Windows-x64.msi`. It installs for you only, so it does not ask for an administrator password.
   - **Mac:** open `Kinosail-macOS.dmg` and drag **Kinosail** to **Applications**. Then open Kinosail from **Applications**.
   - **Linux:** use the terminal command above, or run `sudo apt install ./Kinosail-Linux-x64.deb` in your Downloads folder.

   Windows and Mac warn the first time, because the app is not signed yet. See [The first time you open Kinosail](#first-open).
2. **Open Kinosail.** On Windows and Mac, the setup page opens in your browser by itself. If it does not, go to `https://localhost:38127` on the same computer. The first time, your browser warns about the certificate. This is normal for a new server. Choose to continue.
3. **Follow the setup.** Create your account, pick your movie and show folders, and turn on **Watch on your TV and phone**.

On Linux, if a folder does not appear, the `kinosail` user cannot read it. Move your media to `/srv/media`, or let the `kinosail` user read the folder.

Kinosail keeps itself running. It starts again when the computer restarts. On Windows, find it in the notification area next to the clock. On Mac, find it in the menu bar. Its menu has **Open Kinosail**, **Restart Kinosail**, and **Quit Kinosail**.

## The first time you open Kinosail {#first-open}

The Windows and Mac apps are not signed yet, so your computer warns before it opens them. You only do this once.

**Windows**

1. Open `Kinosail-Windows-x64.msi`.
2. If you see **Windows protected your PC**, select **More info**.
3. Select **Run anyway**.

**Mac**

1. Open Kinosail from **Applications**. macOS says it cannot check Kinosail. Select **Done**.
2. Open **System Settings** and select **Privacy & Security**.
3. Scroll down to **Security**. Next to the message about Kinosail, select **Open Anyway**.
4. Enter your Mac password, and then select **Open Anyway** again.

If you do not see **Open Anyway**, open Kinosail from **Applications** again, and then repeat steps 2 to 4.

## All downloads {#all-downloads}

Every link below always gives you the newest version.

<div class="download-options">
  <section class="download-option" aria-labelledby="download-windows">
    <h3 id="download-windows">Windows</h3>
    <p>For Windows 10 and 11.</p>
    <p><a data-download-file="windows-x64-msi" href="https://github.com/Kinosail/kinosail/releases/latest/download/Kinosail-Windows-x64.msi">Download .msi</a></p>
  </section>
  <section class="download-option" aria-labelledby="download-mac">
    <h3 id="download-mac">Mac</h3>
    <p>For macOS 13 Ventura or newer, on Apple silicon or Intel.</p>
    <p><a data-download-file="macos-dmg" href="https://github.com/Kinosail/kinosail/releases/latest/download/Kinosail-macOS.dmg">Download .dmg</a></p>
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

- **Windows:** open **Settings**, select **Apps** and then **Installed apps**. Find **Kinosail** and select **Uninstall**. Your profiles and watch history stay in `%LOCALAPPDATA%\Kinosail` until you delete that folder.
- **Mac:** select **Quit Kinosail** in the menu bar. Then drag **Kinosail** from **Applications** to the Trash. Your profiles and watch history stay in `~/Library/Application Support/Kinosail` until you delete that folder.
- **Linux:** run `sudo apt remove kinosail` or `sudo dnf remove kinosail`. Your profiles and watch history stay in `/var/lib/kinosail` until you delete that folder.

Kinosail never changes or deletes your movie files.

## Is this download safe?

GitHub Actions builds every file from the public source code. Each release has a `SHA256SUMS` file with a [Sigstore](https://www.sigstore.dev/) signature. The signature proves that the Kinosail release workflow made the files. The terminal command checks the file before it installs anything. [Every release](https://github.com/Kinosail/kinosail/releases) lists its files and signatures.
