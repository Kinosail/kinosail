---
title: Profiles and household access
description: Use Owner and Viewer Profiles with personal progress and access limits.
section: Use Kinosail
---

# Profiles and household access

Profiles keep household access and viewing state separate. An Owner creates and manages Profiles from Settings. A Viewer uses the access that the Owner grants.

## Understand the roles

| Role | Can do | Cannot do |
| --- | --- | --- |
| **Owner** | Manage the Server, libraries, Profiles, integrations, playback settings, backups, and shares. | Owners cannot use the public HTTPS Viewer boundary for administration. Use the local network or WireGuard. |
| **Viewer** | Browse permitted libraries, play permitted items, and use permitted personal features. | Cannot manage Server settings or use Owner-only routes. |

A Server can have multiple Owners, but it must always retain at least one Owner. The Owner can remove a Profile. Removing a Profile removes its sessions and associated API keys.

SCIM-managed Profiles are controlled by the identity provider. An Owner cannot edit or delete a SCIM-managed Profile in Kinosail. SCIM provisions Viewer Profiles and never grants Owner access.

## Use personal state

The following state belongs to the signed-in Profile:

- playback position and watched state;
- viewing history and ratings;
- My List membership;
- personal playlist membership; and
- permitted offline downloads.

An Owner may see and manage library content, but a Viewer’s personal state does not become another Viewer’s state.

## Understand Viewer limits

An Owner can set these Viewer controls:

- **Libraries:** no library, every library, or selected libraries.
- **Content rating:** Family, Teen, or unrestricted content ceilings.
- **Viewing hours:** an optional daily start and end time. A schedule can cross midnight.
- **Remote access:** whether the Profile can use the public HTTPS boundary.
- **Transcoding:** whether the Profile can consume transcoding capacity.
- **Downloads:** whether the Profile can prepare and retrieve offline files.

The Server checks these rules for browse, playback, remote connection, Watch Room, and download operations. A hidden or denied item is not a client-side filter.

## Secure your sign-in

Use a unique password of at least 12 characters. Each Owner must enroll a passkey or a time-based one-time password (TOTP) authenticator before normal use. Store TOTP recovery codes in a safe place. Each recovery code works once.

Open **Account** to register or remove passkeys, set up TOTP, review active sessions, and sign out. Use a recent strong sign-in when Kinosail asks for a security step-up.

Remote password login is disabled. A remote Viewer uses a verified passkey or a short-lived, one-use Quick Connect request approved from a strongly authenticated local or WireGuard session.

If sign-in fails, do not share a password, passkey credential, TOTP secret, recovery code, or session cookie. Use [Sign-in and access problems]({{ '/troubleshooting/sign-in-and-access/' | relative_url }}).

{% include screenshot.html title="Profile policy" alt="Future screenshot of an Owner editing a Viewer Profile with library, rating, schedule, remote, transcoding, and download controls." description="Capture policy labels without real names or private library paths." %}

Source of truth: `internal/server/profiles.go`, `profile_policy.go`, and authentication handlers.
