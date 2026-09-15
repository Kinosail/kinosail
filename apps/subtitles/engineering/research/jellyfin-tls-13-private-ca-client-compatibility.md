# Jellyfin client compatibility with TLS 1.3 and a private CA

Research snapshot: 2026-08-23.

## Conclusion

No. This report evaluated the original TLS 1.3-only Kinosail listener whose certificate chained to a Kinosail-generated private CA; that configuration could not be described as compatible with all official Jellyfin clients.

The private-CA design can work for current browsers, current Apple devices, and Android/Android TV devices where the owner can install the CA. It is not a universal TV-client path:

- Jellyfin's Android apps still install on Android 6 and later, while Android did not add platform TLS 1.3 until Android 10.
- Roku's official client explicitly uses Roku's bundled public-CA file for API and media requests, rather than a user-installed Kinosail CA.
- LG documents that webOS TV's trusted-root list cannot be modified; webOS 4.x and older also lack TLS 1.3.
- Samsung's Smart TV documentation lists support only through TLS 1.2.

Jellyfin itself therefore recommends HTTPS with a publicly trusted CA and strongly discourages self-signed certificates because of security and compatibility problems ([Jellyfin networking documentation](https://jellyfin.org/docs/general/post-install/networking/#ssl--https)). For the widest official-client compatibility, use a stable DNS hostname, a public-CA certificate, and allow TLS 1.2 and 1.3. TLS 1.2 can still be restricted to forward-secret AEAD cipher suites.

This conclusion concerns TLS transport only. It does not establish that every client supports Kinosail's current Jellyfin-compatible API surface.

## Compatibility matrix

| Official client/platform | TLS 1.3 | Kinosail private CA | Result for this setup |
| --- | --- | --- | --- |
| Jellyfin for Android | Android 10+ has platform TLS 1.3; the official app's minimum is Android 6 | The app explicitly trusts Android's user CA store | **Conditional:** expected on Android 10+ after CA installation; not all supported Android devices |
| Jellyfin for Android TV / Fire TV | Android 10+ and Fire OS 8+ have TLS 1.3; older supported Android/Fire OS releases do not | The app explicitly trusts the user CA store, but certificate installation is device/firmware dependent | **Conditional:** not universal across Android TV and Fire TV hardware |
| Jellyfin Mobile for iOS/iPadOS | iOS 12.2+ enables TLS 1.3 for `URLSession` and Network.framework | A manually installed root works only after the user explicitly enables full SSL trust | **Conditional:** viable on current devices after both installation and full-trust steps; physical-app verification remains necessary |
| Modern desktop/mobile browsers | Current major browsers support TLS 1.3 | They can trust a private root through their OS/browser certificate store | **Conditional:** viable when the entered hostname matches the certificate and the root is trusted; old clients remain excluded |
| Jellyfin for Roku | Roku OS 11 moved to OpenSSL 1.1.1, but Roku does not publish enough client-level evidence here to certify TLS 1.3 negotiation | The Jellyfin client hardcodes `common:/certs/ca-bundle.crt` for discovery/API and media | **No** for the generated private CA without modifying and repackaging the app |
| Jellyfin for webOS | TLS 1.3 is available on webOS 5.x/2020 and later; webOS 4.x and earlier lack it | LG says HTTPS resources must chain to a root already in the TV's immutable trusted-root list | **No** for the generated private CA; TLS 1.3-only also excludes older supported TVs |
| Jellyfin for Tizen | Samsung documents Smart TV TLS support only through TLS 1.2 | Not decisive because the protocol mismatch already prevents connection | **No** with a TLS 1.3-only server |
| Jellyfin Media Player/Desktop, Xbox, JellyCon/Kodi, and other listed clients | Depends on the bundled runtime and host OS | Depends on the runtime/system trust integration | **Uncertified:** no first-party evidence supports a universal claim; test each supported OS/device |

The official-client scope above follows Jellyfin's current [client catalog](https://jellyfin.org/clients/).

## Evidence

### Android Mobile, Android TV, and Fire TV

Both current Jellyfin Android applications deliberately include the user certificate store alongside system roots in their network-security configuration: [Android Mobile](https://github.com/jellyfin/jellyfin-android/blob/5779ef02df46d1b57e23d72e83b65a4bc0f0a1ee/app/src/main/res/xml/network_security_config.xml#L7-L14) and [Android TV](https://github.com/jellyfin/jellyfin-androidtv/blob/a19736b4c89a8d1a76dff069d2ab9c68b7416f47/app/src/main/res/xml/network_security_config.xml#L3-L7). That makes a correctly installed Kinosail CA eligible for app trust; the app does not merely bypass certificate validation.

However, both applications currently declare API 23 (Android 6) as their minimum platform ([Android Mobile build](https://github.com/jellyfin/jellyfin-android/blob/5779ef02df46d1b57e23d72e83b65a4bc0f0a1ee/app/build.gradle.kts#L29-L38), [Android TV build](https://github.com/jellyfin/jellyfin-androidtv/blob/a19736b4c89a8d1a76dff069d2ab9c68b7416f47/app/build.gradle.kts#L8-L20), with `android-minSdk = "23"` in their respective [Mobile](https://github.com/jellyfin/jellyfin-android/blob/5779ef02df46d1b57e23d72e83b65a4bc0f0a1ee/gradle/libs.versions.toml#L1-L6) and [TV](https://github.com/jellyfin/jellyfin-androidtv/blob/a19736b4c89a8d1a76dff069d2ab9c68b7416f47/gradle/libs.versions.toml#L5-L10) catalogs). Google documents that Android added TLS 1.3 in Android 10 and enables it by default there ([Android 10 TLS changes](https://developer.android.com/privacy-and-security/security-ssl#tls-1.3)). Consequently, TLS 1.3-only excludes part of the apps' supported OS range.

Amazon likewise lists Fire OS 6 as Android 7.1-based, Fire OS 7 as Android 9-based, and Fire OS 8 as Android 10/11-based ([Fire OS version mapping](https://developer.amazon.com/docs/fire-tv/fire-os-overview.html#fire-os-versions)). Amazon documents TLS 1.3 as an addition in Fire OS 8 ([Fire OS 8 TLS support](https://developer.amazon.com/docs/fire-tablets/fire-os-8.html#tls-13-support-enabled-by-default)). Amazon also documents a CA-installation flow for Fire TV, while noting that Fire OS 6 apps must opt into user certificates ([Fire TV certificate guidance](https://developer.amazon.com/docs/fire-tv/network-proxy.html#working-with-https-and-encrypted-data-using-charles-proxy)); Jellyfin Android TV does make that opt-in. Installation still needs to be validated on each target Fire TV/Android TV model.

### iOS and browsers

Apple documents that TLS 1.3 is enabled by default for `Network.framework` and `NSURLSession` on iOS 12.2 and later ([Apple Platform Security: TLS security](https://support.apple.com/guide/security/tls-security-sec100a75d12/web)). Apple also documents that a manually installed root is not automatically trusted for SSL: the user must enable it under Certificate Trust Settings ([Apple certificate-trust instructions](https://support.apple.com/en-us/102390)). This supports the private-CA approach on current Apple platforms, provided Kinosail's certificate hostname/IP SAN matches the URL the client uses.

Firefox 120+ searches the Windows, macOS, and Android OS stores for user-installed third-party roots by default; Mozilla also documents explicit enterprise-root and Linux import paths ([Mozilla CA setup](https://support.mozilla.org/en-US/kb/setting-certificate-authorities-firefox)). Other browsers remain subject to their platform/browser trust-store rules. Browser success does not imply native TV-client success.

### Roku

Roku's HTTPS API requires the channel to name a PEM CA bundle with `SetCertificatesFile`; it may use Roku's standard bundle or package another CA in the channel ([Roku `roUrlTransfer`](https://developer.roku.com/docs/references/brightscript/components/rourltransfer.md)). The official Jellyfin Roku client names only `common:/certs/ca-bundle.crt` when probing a server ([server probe](https://github.com/jellyfin/jellyfin-roku/blob/02ed841480e067acddbe5a886f2388b38008fd7e/source/utils/misc.bs#L222-L236)) and applies that same bundle to content/media requests ([request helper](https://github.com/jellyfin/jellyfin-roku/blob/02ed841480e067acddbe5a886f2388b38008fd7e/source/api/baserequest.bs#L219-L222)). A Kinosail-generated root will not be in that bundle. Trusting it would require changing and repackaging the Roku channel, which is not compatibility with the unmodified official client.

Roku OS 11 adopted OpenSSL 1.1.1 ([Roku OS release notes](https://developer.roku.com/docs/release-notes#roku-os-110)), but that alone does not prove that every supported Roku device and every Jellyfin request/playback path negotiates TLS 1.3. The private-CA failure is sufficient to reject universal compatibility regardless.

### LG webOS

LG's platform matrix says TLS 1.3 is supported on webOS 5.x (2020) and later but not webOS 4.x and earlier. The same first-party page says an HTTPS resource must chain to a root in the TV's trusted list or it fails with `ERR_INSECURE_RESPONSE` ([LG TLS and root-certificate matrix](https://webostv.developer.lge.com/develop/specifications/tls)). LG further states that applications cannot deploy their own certificates and the TV's root list cannot be modified ([LG developer response](https://forum.webostv.developer.lge.com/t/how-to-deploy-certificate-on-displays-remotely/4173/2)). Therefore neither installing Kinosail's root nor bypassing the warning is a supported owner action.

### Samsung Tizen

Samsung's Smart TV security documentation lists TLS 1.0, 1.1, and 1.2 as supported and does not list TLS 1.3 ([Samsung Smart TV Security Q&A](https://developer.samsung.com/smarttv/develop/faq/security.html)). Under that first-party contract, an official Tizen client cannot connect to a TLS 1.3-only Kinosail listener, independent of certificate trust.

## Product decision

Keep the generated local-CA mode as a secure local option for Kinosail's bundled web UI and devices that can explicitly trust it, but do not present it as the universal Jellyfin-client mode. A compatibility setting should offer:

1. **Local trust:** TLS 1.2 and 1.3 with the Kinosail private CA for devices where an Owner can install it.
2. **Compatibility Connection:** the same secure protocol policy with a publicly trusted certificate for a stable DNS hostname.

The existing HTTP opt-out can remain an emergency local fallback, but it sends credentials and media-session tokens without transport encryption and should not be the recommended answer to a client trust failure.

## Verification boundary

This report is a primary-source protocol/platform review, not physical-device certification. Exact behavior can vary by client release, OS/firmware version, manufacturer certificate UI, embedded web engine, and whether API, artwork, WebSocket, and media playback use the same trust stack. Before claiming support, test server add, login, artwork, direct play, transcoding/HLS, seeking/ranges, WebSockets, and downloads on each named physical client using the exact certificate chain and hostname.
