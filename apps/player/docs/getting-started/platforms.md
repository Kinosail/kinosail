---
title: Install on a NAS or Proxmox
description: Import Kinosail Player or Subtitles into common container managers.
section: Start here
last_reviewed: 2026-09-27
---

# Install on a NAS or Proxmox

Kinosail provides two independent containers. Install Player to browse and play media. Install Subtitles to save subtitle files beside media. Both images support Linux AMD64 and ARM64.

The Compose files work with Synology Container Manager, TrueNAS SCALE Custom Apps, QNAP Container Station, CasaOS, Portainer, Dockge, and Docker Compose. They use published images, so you do not need to build Kinosail. Each app has its own project name, port, and saved volumes, so both can run on one host.

## Prepare the media path

1. Find the absolute path on the machine that runs containers. A path on your laptop will not work on a NAS. In a Proxmox VM, mount the media share inside the VM first.
2. Give container user and group 10001:10001 access to that path. Player needs read access. Subtitles needs read and write access to create sidecar files.
3. Keep the machine on a trusted private network. Do not forward ports 38127 or 38128 to the internet.

## Make a ready-to-import Compose file

Choose an app and enter the media path on the machine that runs its container. The helper fills that path into a Compose file for you. It does not send your path to Kinosail. Your browser cannot check that the folder exists or that user 10001 can use it.

<section class="install-builder" hidden data-install-builder data-player-template="{{ '/assets/install/player.yaml' | relative_url }}" data-subtitles-template="{{ '/assets/install/subtitles.yaml' | relative_url }}">
  <div class="install-fields">
    <label>App
      <select data-install-app>
        <option value="player">Player</option>
        <option value="subtitles">Subtitles</option>
      </select>
    </label>
    <label>Media path on the server
      <input type="text" data-install-media placeholder="/mnt/media" maxlength="4096" autocomplete="off" spellcheck="false" aria-describedby="install-media-help">
    </label>
    <label>HTTPS port on the server
      <input type="text" data-install-port value="38127" maxlength="5" inputmode="numeric" aria-describedby="install-port-help">
    </label>
  </div>
  <p class="install-hint" id="install-media-help">Use an existing absolute path, such as <code>/mnt/tank/Movies</code>. For Proxmox, use the path inside the Linux VM.</p>
  <p class="install-hint" id="install-port-help">The default port works unless another app already uses it. Use a port from 1024 to 65535.</p>
  <button type="button" class="install-primary" data-install-create>Make Compose file</button>
  <p class="install-error" role="alert" data-install-error hidden></p>
  <div class="install-result" data-install-result hidden>
    <p class="install-actions"><a data-install-download>Download Compose file</a><button type="button" data-install-copy>Copy file text</button></p>
    <p role="status" aria-live="polite" data-install-status></p>
    <details><summary>Review the file</summary><pre><code data-install-preview></code></pre></details>
  </div>
</section>
<script defer src="{{ '/assets/js/platform-install.js' | relative_url }}"></script>

## Import with a Compose app manager

1. Download the file from the helper above. If the helper is unavailable, download the original [Player Compose file](https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/player/packaging/platform-compose.yaml) or [Subtitles Compose file](https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/subtitles/packaging/platform-compose.yaml). Set `KINOSAIL_MEDIA_PATH` in your manager when you use an original file.
2. Import or paste the file in your manager:

   | Manager | Import location |
   | --- | --- |
   | Synology DSM | **Container Manager → Project → Create**. Upload the YAML as the project source. |
   | TrueNAS SCALE | **Apps → Discover → ⋮ → Install via YAML**. Paste the file. |
   | QNAP | **Container Station → Applications → Create**. Paste the Docker Compose YAML. |
   | CasaOS | Import the Docker Compose file as a custom app. |
   | Portainer | **Stacks → Add stack → Upload** or **Web editor**. |
   | Dockge | Create a stack with **+ Compose**. Paste the file text into its editor. |
   | Docker Compose | Run `docker compose -f YOUR_FILE.yaml up -d` beside the downloaded file. Replace `YOUR_FILE.yaml` with its filename. |

3. Start the project and check that the container reports healthy. Open `https://NAS-IP:38127` for Player or `https://NAS-IP:38128` for Subtitles, using your chosen port. The local certificate can cause a browser warning. Trust it only for your own server.
4. Create the first Owner immediately. In Owner Settings, configure a private backup key and follow any pending restart notice. Run **Back up now** and verify the backup before depending on recovery. Save the key separately from the app volumes.

If you use an original file, an unset `KINOSAIL_MEDIA_PATH` stops Compose before deployment. Check that your manager did not create an empty folder if the entered path was wrong.

The files publish only the HTTPS port. They do not turn on public remote access or hardware acceleration. Add those later after the basic install works.

## Install with Unraid

Download the [Player template](https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/player/packaging/unraid.xml) or [Subtitles template](https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/subtitles/packaging/unraid.xml). Place it in /boot/config/plugins/dockerMan/templates-user/ on the Unraid host. In **Docker → Add Container**, select the Kinosail template.

Set **Media** to an existing share before applying the template. The field has no default. Keep Player's media mapping read-only. Subtitles needs a writable mapping and suitable permissions for user 10001. The default app data paths are under /mnt/user/appdata/; include them in your normal app data backup.

These XML files are local templates. They are not listings in Unraid Community Apps.

## Install with Proxmox VE

Create a Linux VM in Proxmox VE, install Docker Engine and the Compose plugin in that VM, and mount your media share inside it. Then use the [verified Player installer]({{ '/getting-started/install/' | relative_url }}) from the VM, or import the Compose file above with a container manager in the VM. Subtitles has its own [verified installer](https://github.com/Kinosail/kinosail/blob/main/apps/subtitles/scripts/install.sh) under apps/subtitles.

Proxmox recommends a VM for Docker application containers. Keep Docker off the Proxmox host and out of LXC for this install.

## Update and recover

Keep the same project name, app volumes, and media mapping when you update. Back up and verify app state first. Then have the manager pull the new image and redeploy. Check health, sign-in, and one media item before deleting an old image. Do not select an option that deletes app volumes or app data.

The app managers pull the published latest image but do not verify its signature or pin its digest. Use the [verified installer]({{ '/getting-started/install/' | relative_url }}) when you need that check and its automatic backup before update.

Sources: [Synology Project](https://kb.synology.com/en-global/DSM/help/ContainerManager/docker_project), [TrueNAS Custom Apps](https://apps.truenas.com/managing-apps/installing-custom-apps/), [QNAP Container Station](https://www.qnap.com/en/how-to/tutorial/article/how-to-configure-the-default-web-url-port-for-containers-and-applications-in-container-station-3), [CasaOS Compose import](https://wiki.casaos.io/en/contribute/development), [Portainer stacks](https://docs.portainer.io/user/docker/stacks/add), [Dockge stacks](https://github.com/louislam/dockge), [Proxmox container guidance](https://pve.proxmox.com/pve-docs/pct.1.html), and [Docker Compose bind mounts](https://docs.docker.com/reference/compose-file/services/#volumes).
