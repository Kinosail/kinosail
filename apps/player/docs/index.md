---
title: Your media. Your Server.
description: Learn how to install, use, secure, and maintain Kinosail.
hide_contribute: true
---

# Your media. Your Server.

Kinosail brings movies, shows, music, audiobooks, books, and photos together on one private Server. Start with a guided install, then use these docs when you need a specific task or exact setting.

<div class="path-grid">
  <a href="{{ '/getting-started/' | relative_url }}"><strong>Set up a new Server</strong><span>Install Kinosail, create the first Owner, and scan your media.</span></a>
  <a href="{{ '/getting-started/connect-devices/' | relative_url }}"><strong>Connect a phone, TV, or app</strong><span>Use secure local access, or set up required trusted HTTPS for Jellyfin apps.</span></a>
  <a href="{{ '/user-guide/' | relative_url }}"><strong>Use Kinosail</strong><span>Browse, play, organize, and share your library.</span></a>
  <a href="{{ '/owner-guide/' | relative_url }}"><strong>Operate the Server</strong><span>Manage access, playback, integrations, backups, and updates.</span></a>
  <a href="{{ '/developer-guide/' | relative_url }}"><strong>Build with Kinosail</strong><span>Use the HTTP API to create clients, automations, and integrations.</span></a>
  <a href="{{ '/reference/architecture-and-privacy/' | relative_url }}"><strong>Understand the design</strong><span>See how Kinosail handles media, state, privacy, and optional services.</span></a>
  <a href="{{ '/reference/' | relative_url }}"><strong>Look up exact details</strong><span>Find settings, API behavior, formats, terms, and compatibility limits.</span></a>
  <a href="{{ '/troubleshooting/' | relative_url }}"><strong>Fix a problem</strong><span>Start from the symptom and follow focused checks.</span></a>
</div>

{% include screenshot.html title="Kinosail library home" alt="Future screenshot of a populated Kinosail home library on desktop." description="Add this image after the application visual design is final." %}

## What makes Kinosail different

- One Kinosail Server container includes the web interface, API, background work, SQLite state, and required media tools.
- Your Library Content mounts read-only and streams directly from your Server to your device.
- Local use does not require a Kinosail-hosted account or media relay.
- Owners control each Viewer Profile, library boundary, remote permission, download permission, and playback capacity.
- The web app, versioned API, and compatible clients use the same Server rules.

## Find the right kind of help

The documentation separates learning from task instructions and exact reference:

- **Getting started** gives one safe path to a working Server.
- **User and Owner guides** help you complete a specific task.
- **Developer guides** show how to build against the versioned HTTP API.
- **Reference** describes exact behavior, settings, formats, and interfaces.
- **Troubleshooting** starts with visible symptoms and recovery checks.

## Before you begin

You need a supported 64-bit Linux host, Docker Compose or Podman Compose, and an absolute path to media you control. A macOS host can run the Linux container for local use. Read [Choose an install path]({{ '/getting-started/' | relative_url }}) for the full decision.
