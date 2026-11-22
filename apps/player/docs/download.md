---
title: Download Kinosail
description: Download the Kinosail Player installer for your computer. Kinosail runs in the background as a service and starts by itself when the computer starts.
section: Start here
last_reviewed: 2026-11-22
---

# Download Kinosail

Kinosail installs like a normal app. It runs in the background and starts by itself when your computer starts. You do not need Docker.

<section class="download-pick" aria-labelledby="download-pick-title" data-download>
  <h2 id="download-pick-title">Recommended for your computer</h2>
  <p class="download-detected" data-download-detected>Choose your computer below.</p>
  <p class="download-actions" data-download-primary hidden><a class="install-primary" data-download-button href="https://github.com/Kinosail/kinosail/releases/latest">Download</a></p>
  <p class="download-meta" data-download-meta hidden></p>
  <p class="download-note" data-download-note hidden></p>
</section>
<script defer src="{{ '/assets/js/download.js' | relative_url }}"></script>

## Linux

Kinosail runs on 64-bit Linux with systemd (Intel, AMD, or ARM such as Raspberry Pi 4 and 5).

**One-step install.** Open a terminal and run this command. It picks the right file, checks it, and installs it:

```sh
curl -fsSL https://kinosail.com/install.sh | sh
```

**Or download a package yourself.** Pick the line that matches your Linux:

| Your Linux | Intel or AMD (x64) | ARM (arm64) |
| --- | --- | --- |
| Debian, Ubuntu, Linux Mint, Pop!_OS, Raspberry Pi OS | [Kinosail-Linux-x64.deb](https://github.com/Kinosail/kinosail/releases/latest/download/Kinosail-Linux-x64.deb) | [Kinosail-Linux-arm64.deb](https://github.com/Kinosail/kinosail/releases/latest/download/Kinosail-Linux-arm64.deb) |
| Fedora, RHEL, Rocky, AlmaLinux, openSUSE | [Kinosail-Linux-x64.rpm](https://github.com/Kinosail/kinosail/releases/latest/download/Kinosail-Linux-x64.rpm) | [Kinosail-Linux-arm64.rpm](https://github.com/Kinosail/kinosail/releases/latest/download/Kinosail-Linux-arm64.rpm) |

Not sure which one you have? Run `uname -m` in a terminal. `x86_64` means x64. `aarch64` means arm64.

To install a file you downloaded, run `sudo apt install ./Kinosail-Linux-x64.deb` or `sudo dnf install ./Kinosail-Linux-x64.rpm`.

After you install:

1. On the same computer, open `https://localhost:38127`. Your browser warns about the certificate the first time. This is expected for a new server.
2. Create your account. The setup wizard then helps you protect it, pick your movie and show folders, and share Kinosail with your TV and phone.
3. If a folder does not appear, the `kinosail` user cannot read it. Move your media to `/srv/media`, or let the `kinosail` user read the folder.

To remove Kinosail, run `sudo apt remove kinosail` or `sudo dnf remove kinosail`. Your library data stays in `/var/lib/kinosail` until you delete it.

## Windows

The Windows installer is coming soon. Until then, use [Docker Desktop with the Docker quickstart]({{ '/quickstart/' | relative_url }}).

## macOS

The Mac app is coming soon. Until then, use [Docker Desktop with the Docker quickstart]({{ '/quickstart/' | relative_url }}).

## Other ways to install

- [Docker Compose]({{ '/quickstart/' | relative_url }}) for servers and advanced setups.
- [NAS, Unraid, TrueNAS, and Proxmox]({{ '/getting-started/platforms/' | relative_url }}).
- [Every release and its checksums](https://github.com/Kinosail/kinosail/releases). Each release has a `SHA256SUMS` file and a Sigstore signature bundle.
