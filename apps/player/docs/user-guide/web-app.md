---
title: Make the web Player yours
description: Install the web app, use keyboard shortcuts, and adjust your browsing experience.
section: Use Kinosail
last_reviewed: 2026-09-20
---

# Make the web Player yours

The web Player works in a browser on your computer, tablet, or phone. You can also install it as a web app where your browser supports that feature.

## Install the web app

Open your Server in the browser you intend to use, then choose **Actions → Install** or **More → Install** on a smaller screen. Follow the browser's install prompt. On Apple devices, use the browser's share menu and **Add to Home Screen** when offered.

Installation availability depends on the browser and whether the app is already installed. If no install prompt is available, bookmark the Server address and keep using the browser. Installation alone does not save media for offline use: [download each title to the device]({{ '/user-guide/offline/' | relative_url }}).

## Navigate with the keyboard

These shortcuts apply outside text-entry controls and where the corresponding action is available:

| Shortcut | Action |
| --- | --- |
| `/` | Focus the page's search field. |
| Ctrl+K or Cmd+K | Open the actions menu. |
| `?` | Open the actions/shortcut menu. |
| `G`, then a destination key | Go to a destination shown in the menu. |
| `J` / `K` | Move focus through supported browsing items. |

Use Tab and Shift+Tab to move through links and controls, and Enter or Space to activate the focused control. The actions menu shows the destinations available to your Profile.

## Appearance, language, and navigation

Use the theme control to choose the appearance you prefer. Language choices apply to supported interface translations; media titles and descriptions come from your library metadata.

Owners can adjust the Server's navigation in Settings. If a destination is missing, first check navigation and library permissions. See [Profiles and household access]({{ '/user-guide/profiles/' | relative_url }}).

## Play on another device

During playback, open **Watch together → Play on device**. Player asks the browser to select a compatible device using available Remote Playback or AirPlay support. Availability depends on the browser, device discovery, and the receiving device's ability to reach your Server and play the media.

If no device is offered, check that both devices are on the appropriate network and try a browser with the required playback support. This is separate from [Watch Rooms]({{ '/user-guide/sharing/' | relative_url }}), where people watch through their own connected browsers.

Source of truth: `packages/webassets/static/pwa.js`, `shortcuts.js`, `player-devices.js`, and the Player web templates.
