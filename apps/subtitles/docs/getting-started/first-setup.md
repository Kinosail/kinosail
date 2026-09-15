---
title: Complete first setup
description: Secure the Owner and find a subtitle for one video.
section: Start here
last_reviewed: 2026-09-15
---

# Complete first setup

Open the local address from [installation]({{ '/getting-started/install/' | relative_url }}). Create the first Owner with a unique password of at least 12 characters, then enroll the required passkey or TOTP authenticator. Save recovery material privately. Finish this before enabling access from other machines.

## Configure the subtitle plan

1. Follow **Connect a subtitle provider**. One provider is sufficient; use the linked account and key instructions.
2. Enter credentials in Owner Settings. Restart when prompted and confirm that the provider is configured.
3. Choose the primary language and subtitle role: standard dialogue or SDH/captions.
4. Review **Choose where to save subtitle files**. Add folders inside the media mount, such as `Movies` or `Shows`.
5. Choose a safety-scan schedule and save the plan.

Deployment-managed settings are read-only. Change the corresponding environment or YAML value when the interface says it is managed.

## Confirm the result

Run a scan, open a wanted video, and fetch a subtitle. Check that the file appears beside the video and that its language and timing fit your copy. The preferred language defaults to `en`; for example, `Film.en.srt` covers English for `Film.mkv`.

No provider result, quota exhaustion, an invalid subtitle, or a write-permission failure can prevent a fetch. Read the reported result before retrying. See [troubleshooting]({{ '/troubleshooting/' | relative_url }}).

Local coverage still works without a provider. Optional OCR/transcription produces local drafts for review; those drafts do not automatically replace existing subtitles.
