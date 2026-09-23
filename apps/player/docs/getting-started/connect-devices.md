---
title: Connect browsers and Jellyfin apps
description: Open Kinosail from another device in a browser or a compatible Jellyfin mobile app.
section: Start here
---
# Connect browsers and Jellyfin apps

Start by confirming playback in the browser on your Server host. Then make Kinosail reachable from another computer, phone, or tablet on your home network.

The Kinosail Server is free to run. It includes the web Player and can connect to compatible Jellyfin mobile apps for iOS and Android. Kinosail mobile apps may have a separate purchase price. Jellyfin compatibility is optional, off by default, and requires trusted HTTPS.

## Finish setup first

Create your Owner and enroll a passkey or TOTP authenticator before enabling LAN access. The default installation binds the web port to `127.0.0.1` so only the host can reach it.

On another device, `localhost` means that device—not the Server. Use the Server's reachable address instead.

## Enable LAN access

For a signed prebuilt Docker installation, run the installer again from the same installation directory, using the same media path and port, with `--lan`:

```sh
./scripts/install.sh /absolute/path/to/media 38127 --lan
```

The installer checks that setup has completed, changes the bind address, and reports the LAN address. If the host has several network interfaces, verify the reported address. Allow only the intended home network through the host firewall.

This does not configure internet access. Do not forward the administration web port on your router.

## Choose the HTTPS address

Kinosail keeps HTTPS on. Use the exact address covered by the Server certificate. Passkeys belong to an origin, so changing the address can require enrolling them again.

For trusted HTTPS, open the setup wizard's **Devices** step or **Settings → Access**. The local certificate flow supports DuckDNS and deSEC hostnames. Supply your hostname, provider token, and private LAN address, complete the certificate setup, and restart when prompted. Enabling a compatible third-party client is not required to use the web Player.

The provider publishes the private LAN address in public DNS and the hostname appears in public certificate-transparency logs. This local certificate setup does not open a router port or relay your media.

Keep provider tokens private. Use the narrowest token policy available and only trust certificates belonging to your own installation.

## Connect a Jellyfin mobile app

1. As the Owner, open **Settings → Access** and wait until **Trusted HTTPS** reports ready.
2. Turn on **Allow compatible Jellyfin apps to connect**, save the choice, and restart Kinosail when prompted.
3. In a compatible Jellyfin app, add a Server and enter Kinosail's complete **Connect address** from Settings.
4. Sign in with the intended Viewer Profile. Use **Quick Connect** when the app offers it; approve the matching request from a trusted Kinosail session.

Compatibility covers tested client flows, not every Jellyfin app feature or device. For sign-in failures, see [Jellyfin app troubleshooting]({{ '/troubleshooting/sign-in-and-access/' | relative_url }}).

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
