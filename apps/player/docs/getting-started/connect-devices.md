---
title: Connect browsers and Jellyfin apps
description: Open Kinosail from another device in a browser or a compatible Jellyfin mobile app.
section: Start here
---
# Connect browsers and Jellyfin apps

First, confirm that playback works in a browser on the Server host. Then connect another device on your home network.

Kinosail Server is free to run. The web Player is included. You can also connect compatible Jellyfin mobile apps for iOS and Android. This option is off by default and needs trusted HTTPS.

## Finish setup first

Create the Owner first. Add a passkey or TOTP authenticator before you enable home-network access. By default, the web port uses `127.0.0.1`. Only the host can reach it.

On another device, `localhost` means that device—not the Server. Use the Server's reachable address instead.

## Enable LAN access

For a signed Docker installation, run the installer again from the same folder. Use the same media path and port. Add `--lan`:

```sh
./scripts/install.sh /absolute/path/to/media 38127 --lan
```

The installer checks that setup is complete, changes the bind address, and shows the home-network address. If the host has several network connections, check that this address is correct. In the host firewall, allow only your home network.

This does not configure internet access. Do not forward the administration web port on your router.

## Choose the HTTPS address

Kinosail always uses HTTPS. Use the exact address in the Server certificate. Passkeys are tied to this address. If you change it, you may need to add your passkeys again.

To set up trusted HTTPS, open the wizard's **Devices** step or go to **Settings → Access**. The certificate setup supports DuckDNS and deSEC hostnames. Enter your hostname, provider token, and private home-network address. Finish setup and restart when prompted. You do not need to enable Jellyfin support to use the web Player.

The provider publishes your private home-network address in public DNS. Your hostname also appears in public certificate records. This does not open a router port or relay media.

Keep provider tokens private. Use the most limited token you can. Trust only certificates for your own Server.

## Connect a Jellyfin mobile app

1. As the Owner, open **Settings → Access** and wait until **Trusted HTTPS** reports ready.
2. Turn on **Allow compatible Jellyfin apps to connect**, save the choice, and restart Kinosail when prompted.
3. In a compatible Jellyfin app, add a Server and enter Kinosail's complete **Connect address** from Settings.
4. Sign in with the intended Viewer Profile. Use **Quick Connect** when the app offers it; approve the matching request from a trusted Kinosail session.

Kinosail tests common client tasks. It does not support every Jellyfin feature or device. For sign-in help, see [Jellyfin app troubleshooting]({{ '/troubleshooting/sign-in-and-access/' | relative_url }}).

## Sign in and play

1. Connect the other device to your home network.
2. Open the complete Server HTTPS address in its browser, including the port.
3. Sign in with the intended account or Viewer Profile and complete any required MFA.
4. Open a library item and start playback.
5. Check audio, subtitles, and saved progress.

Use a Viewer Profile for everyday viewing. See [profiles and household access]({{ '/user-guide/profiles/' | relative_url }}) for permissions and library boundaries.

## If it does not connect

- **The page times out:** check the Server bind address, firewall, network, and port. Confirm the device is not on an isolated guest Wi-Fi network.
- **The certificate is rejected:** check the exact hostname, certificate coverage, and device clock. Do not change the URL to HTTP.
- **Sign-in or a passkey fails:** use the configured origin and follow [sign-in troubleshooting]({{ '/troubleshooting/sign-in-and-access/' | relative_url }}).
- **The page works but playback fails:** use [playback troubleshooting]({{ '/troubleshooting/playback/' | relative_url }}) and [media compatibility]({{ '/reference/media-compatibility/' | relative_url }}).

For viewing away from home, follow [remote access]({{ '/owner-guide/remote-access/' | relative_url }}). Public viewing uses a separate restricted HTTPS gateway.
