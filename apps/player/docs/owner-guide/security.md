---
title: Secure accounts
description: Manage Owners, Viewers, passkeys, authenticators, sessions, and identity providers.
section: Own the Server
---

# Secure accounts

Kinosail keeps sign-in protections, same-origin browser checks, private responses, bounded requests, and revocable access enabled. Use **Settings → Profiles** and each Profile's **Profile** page to manage account access.

## Use Owner and Viewer roles

An Owner can administer the Server. A Viewer can use only the Libraries and capabilities allowed by its policy. Keep at least one Owner. New Viewers start with family ratings and no Libraries.

For each Viewer, set:

- **Content**: **Through PG / TV-PG**, **Through PG-13 / TV-14**, or **All ratings**;
- **Libraries**: no folders, `all`, or selected folder names;
- **From** and **Until**: an optional daily viewing window;
- **Managed remote access**;
- **Allow transcoding**; and
- **Allow downloads**.

Select **Save**. To remove a Profile, open **Remove <name>?**, then select **Confirm remove <name>**. Removal signs out the Profile and deletes it. A removed Profile cannot be recovered without a suitable backup.

## Require two-factor authentication

Kinosail requires extra sign-in protection for every Viewer Profile by default. In **Settings → Protected automatically**, clear **Require extra sign-in protection for every Viewer Profile** only if your household needs to opt out. Turning this on signs everyone out. A Profile without TOTP can sign in only to set it up.

Each Profile can use a TOTP authenticator and one-use recovery codes. Recovery codes are secrets. Store them in a password manager or another protected location. A used recovery code is removed.

Owners must retain a passkey or authenticator. Do not disable the last Owner's only second factor.

## Add passkeys and manage sessions

Sign in at the exact configured HTTPS address. Open your Profile menu, then **Sign-in methods**. Choose **Add a passkey** or **Set up or replace authenticator**. Passkeys require the configured Server address and trusted HTTPS. Use Face ID, Touch ID, Windows Hello, or a device PIN when your authenticator offers it.

Use **Settings → Automatic sign-out** to set **After inactivity** and **Always after** limits. The recommended values are 15 minutes and 8 hours. Use **Sign out other devices** to revoke other sessions. Revoke one device beside its session when you need a narrower action.

## Use external identity carefully

OpenID Connect and Security Assertion Markup Language (SAML) are optional. Configure them under **Settings → Deployment configuration**. Link an external identity from an already authenticated local Profile. An unknown provider identity cannot create a Profile. Names and email addresses are not identity keys.

SCIM provisioning creates passwordless, least-privilege Viewer Profiles. Configure one SCIM token and its expiration together. SCIM-managed Profiles do not support local passwords and are controlled by the provider. Group provisioning is not supported.

## Create and revoke API keys

In **Settings → API keys**, enter a name and select only required scopes. **Browse library** is the safe default. Optional scopes are **Manage personal library state**, **Stream media**, **Download media**, and **Administer server**.

Select **Create API key** and copy the secret immediately. Kinosail shows it once and stores only a hash. Revoke it from the same section when the device or application no longer needs it.

## Source of truth

Sources: `internal/server/settings_http.go`, `internal/server/profiles_mfa.go`, `internal/server/profiles_credentials.go`, `internal/server/profiles_oidc.go`, `README.md`, and `docs/research/documentation-information-architecture.md`.
