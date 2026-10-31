---
title: Complete first setup
description: Secure the Owner and find a subtitle for one video.
section: Start here
last_reviewed: 2026-09-27
---

# Complete first setup

Open the local address from [installation]({{ '/getting-started/install/' | relative_url }}). Create the first Owner with a unique password of at least 12 characters, then enroll the required passkey or TOTP authenticator. Save recovery material privately. Finish this before enabling access from other machines.

## Configure the subtitle plan

1. Follow **Connect a subtitle provider**. One provider is enough. Use the linked account and key instructions.
2. Enter the credentials in Owner Settings. They take effect immediately. Check that the provider shows **Connected** or **Not tested yet** on the Overview.
3. Review the defaults. They suit most households, so you can usually keep them:
   - Languages: English plus the language of your browser at setup. Change the list and its order in **Settings → Languages**.
   - Subtitle role: standard dialogue. Kinosail falls back to SDH/captions when no standard subtitle is found.
   - Folders: the whole media mount. Add specific folders, such as `Movies` or `Shows`, only when you want to limit the scan.
   - Schedule: new media is found automatically, with a safety scan every 10 minutes.
   - Existing subtitle files that you or another app created are never replaced.
4. Select **Finish**.

Deployment-managed settings are read-only. Change the corresponding environment or YAML value when the interface says it is managed.

## Confirm the result

Run a scan, open a wanted video, and fetch a subtitle. Check that the file appears beside the video and that its language and timing fit your copy. Preferred languages start as English plus your browser language at setup, for example English and German. `Film.en.srt` covers English for `Film.mkv`.

No provider result, quota exhaustion, an invalid subtitle, or a write-permission failure can prevent a fetch. Read the reported result before retrying. See [troubleshooting]({{ '/troubleshooting/' | relative_url }}).

Local coverage still works without a provider. Kinosail copies embedded text tracks and reads image subtitles with OCR to fill missing languages. The manual OCR and transcription tools produce drafts for review; those drafts do not replace existing subtitles.
