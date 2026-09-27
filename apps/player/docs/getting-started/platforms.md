---
title: Install on a NAS or Proxmox
description: Import Kinosail Player or Subtitles into common container managers.
section: Start here
last_reviewed: 2026-09-27
---

# Install on a NAS or Proxmox

Kinosail provides two independent containers. Install [Player](https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/player/packaging/platform-compose.yaml) to browse and play media. Install [Subtitles](https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/subtitles/packaging/platform-compose.yaml) to save subtitle files beside media. Both images support Linux AMD64 and ARM64.

The linked Compose files work with Synology Container Manager, TrueNAS SCALE Custom Apps, QNAP Container Station, CasaOS, Portainer stacks, and ordinary Docker Compose. They use the published images, so you do not need to build Kinosail. Each app has its own port and saved volumes.

## Prepare the media path

1. Find the absolute path on the machine that runs containers. A path on your laptop will not work on a NAS. In a Proxmox VM, mount the media share inside the VM first.
2. Give container user and group 10001:10001 access to that path. Player needs read access. Subtitles needs read and write access to create sidecar files.
3. Keep the machine on a trusted private network. Do not forward ports 38127 or 38128 to the internet.

## Import with a Compose app manager

1. Download the Compose file for the app you want. Open it in a text editor.
2. Set KINOSAIL_MEDIA_PATH in the manager, or replace the entire media source expression with the absolute path to your existing folder. An unset value stops Compose before deployment. Check that your manager did not create an empty folder if the entered path was wrong.
3. Import or paste the edited file in your manager:

   | Manager | Import location |
   | --- | --- |
   | Synology DSM | **Container Manager → Project → Create**. Upload the YAML as the project source. |
   | TrueNAS SCALE | **Apps → Discover → ⋮ → Install via YAML**. Paste the file. |
   | QNAP | **Container Station → Applications → Create**. Paste the Docker Compose YAML. |
   | CasaOS | Import the edited Docker Compose file as a custom app. |
   | Portainer | **Stacks → Add stack → Upload** or **Web editor**. |
   | Docker Compose | Run docker compose -f platform-compose.yaml up -d beside the edited file. |

4. If a host port is occupied, change the number before the colon in the port mapping. Start the project and check that the container reports healthy. Open https://NAS-IP:38127 for Player or https://NAS-IP:38128 for Subtitles, using your chosen port. The local certificate can cause a browser warning. Trust it only for your own server.
5. Create the first Owner immediately. In Owner Settings, configure a private backup key and follow any pending restart notice. Run **Back up now** and verify the backup before depending on recovery. Save the key separately from the app volumes.

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

Sources: [Synology Project](https://kb.synology.com/en-global/DSM/help/ContainerManager/docker_project), [TrueNAS Custom Apps](https://apps.truenas.com/managing-apps/installing-custom-apps/), [QNAP Container Station](https://www.qnap.com/en/how-to/tutorial/article/how-to-configure-the-default-web-url-port-for-containers-and-applications-in-container-station-3), [CasaOS Compose import](https://wiki.casaos.io/en/contribute/development), [Portainer stacks](https://docs.portainer.io/user/docker/stacks/add), [Proxmox container guidance](https://pve.proxmox.com/pve-docs/pct.1.html), and [Docker Compose bind mounts](https://docs.docker.com/reference/compose-file/services/#volumes).
