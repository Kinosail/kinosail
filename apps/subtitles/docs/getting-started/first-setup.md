---
title: Complete first setup
description: Create the first Owner and define the initial subtitle plan.
section: Start here
---

# Complete first setup

The first-launch wizard has two steps. Create the Owner first. Then define the language, write scope, and scan schedule.

## Step 1: Create your Owner

Open the local address from [Install Kinosail]({{ '/getting-started/install/' | relative_url }}). On **Set up Kinosail Subtitles**, enter:

1. Enter a name in **Name**.
2. Enter a unique password of at least 12 characters in **Password**.
3. Keep **MFA - Add extra sign-in protection now** selected.
4. Choose automatic updates or manual approval.
5. Select **Create Owner & continue**.

Kinosail then guides you through time-based one-time password (TOTP) setup. You can use a passkey instead from the Owner account page.

Keep your password and TOTP recovery details in a password manager. Kinosail stores account and subtitle state on this Server.

## Step 2: Define the subtitle plan

On **Define what ready means**, review these settings:

1. Choose the **Primary language**.
2. Choose **Standard dialogue** or **SDH and captions**.
3. Confirm the writable folders inside the media mount.
4. Choose the safety scan schedule.

The first language is primary. Add more Preferred Languages later in **Settings → Languages**.

Kinosail can start without a Subtitle Provider. Local sidecars and embedded text tracks still count toward coverage.

Select **Finish and open overview**. You can reopen this guide from **Settings → Account → Open setup guide**.

## Complete the Server setup

After the wizard:

- Open **Settings → Provider** to add one free Subtitle Provider.
- Open **Settings → Libraries** to confirm the writable Media Library scope.
- Open **Settings → Automation** to review the safety scan schedule.
- Open **Settings → Access** when phones or browsers need Trusted HTTPS.

Trusted HTTPS is optional. It publishes the private LAN address in public DNS, but it does not open a router port.

## Confirm setup

Confirm that:

- the Owner can sign in with the password and configured authenticator;
- the overview shows coverage, Wanted Items, readiness, and provider health;
- the configured Media Libraries are readable and writable;
- a scan finds the expected Media Files; and
- a Sidecar Write never replaces an existing household file.

## Source of truth

Sources: `internal/server/profiles_http.go`, `internal/server/subtitle_onboarding.go`, `internal/server/subtitle_settings.go`, and `README.md`.
