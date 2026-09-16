# Jellyfin player licensing and Kinosail interoperability

Research snapshot: **2026-08-23**. Sources were accessed on that date. This is a primary-source licensing assessment, not legal advice.

## Conclusion

Kinosail's intended model is supportable under the reviewed **copyright licenses**: users install the **unmodified official Jellyfin clients** from Jellyfin's distribution channels, then connect them to an independently written Kinosail Server over compatible HTTP APIs. That interaction does not copy, modify, combine, or distribute the client code. The Android, Android TV, Web, and Desktop GPL terms therefore do not make Kinosail GPL, and the iOS MPL terms do not make Kinosail MPL. Kinosail Server can remain under [PolyForm Perimeter 1.0.1](../../apps/player/LICENSE), subject to its existing [repository licensing boundaries](../../apps/player/LICENSING.md).

This conclusion depends on keeping a clean boundary. Kinosail should not copy client implementation code, bundle or mirror official client binaries, serve copied Jellyfin Web assets, publish a client fork, or present itself as an official Jellyfin product without a separate licensing and trademark review.

There is, however, one compatibility tradeoff at the branding boundary. Current official Android, Android TV, and iOS clients may require the public system-information response to identify the product exactly as `Jellyfin Server`; Kinosail returns `Kinosail Server` to keep its product identity separate. Kinosail must therefore qualify official-client compatibility by client version and test result, and must not promise that current mobile or TV clients will accept the server. If exact compatibility is required later, obtain written permission for the protocol token or an upstream client change that accepts an independently branded compatible server.

## Reviewed official player families

Jellyfin's [official client downloads page](https://jellyfin.org/downloads/clients/) identifies Android, Android TV, iOS, and desktop clients as official open-source clients.

| Player family | Current first-party license/source | Effect on the connection model |
| --- | --- | --- |
| Jellyfin for Android | [GNU GPL version 2](https://github.com/jellyfin/jellyfin-android/blob/5779ef02df46d1b57e23d72e83b65a4bc0f0a1ee/LICENSE.md); [repository/README](https://github.com/jellyfin/jellyfin-android/tree/5779ef02df46d1b57e23d72e83b65a4bc0f0a1ee) | GPLv2 section 0 puts activities other than copying, distribution, and modification outside its scope and does not restrict running the program. An independent server exchanging HTTP with the unmodified app is not, on these facts, a work based on the app. If Kinosail distributed a client copy or derivative, GPL notices, source, and whole-work obligations would apply. |
| Jellyfin for Android TV | [GNU GPL version 2](https://github.com/jellyfin/jellyfin-androidtv/blob/a19736b4c89a8d1a76dff069d2ab9c68b7416f47/LICENSE); [repository/README](https://github.com/jellyfin/jellyfin-androidtv/tree/a19736b4c89a8d1a76dff069d2ab9c68b7416f47) | The same GPL boundary applies: an unmodified app and independent HTTP server remain separate; copying or distributing the app is a different model with GPL obligations. |
| Jellyfin for iOS (the App Store's "Jellyfin Mobile") | [Mozilla Public License 2.0](https://github.com/jellyfin/jellyfin-ios/blob/2d0980aa5a25a1462a6255dd4422175d8f1c1dc1/LICENSE); [repository/README](https://github.com/jellyfin/jellyfin-ios/tree/2d0980aa5a25a1462a6255dd4422175d8f1c1dc1) | MPL 2.0 section 2.1 grants use; sections 3.1-3.4 impose obligations when Covered Software is distributed. Network use by an independent server does not place Kinosail files under MPL. A distributed fork would need to keep covered and modified covered files under MPL, preserve notices, and make their source available; separate files in a Larger Work may use other terms. |
| Jellyfin Web | [GNU GPL version 2](https://github.com/jellyfin/jellyfin-web/blob/57ee0ddb9cab846cf5d1beb76c52c3c9f115af25/LICENSE); [repository/README](https://github.com/jellyfin/jellyfin-web/tree/57ee0ddb9cab846cf5d1beb76c52c3c9f115af25) | The README says Jellyfin Web supplies the frontend used by clients including Android and iOS. Implementing compatible API behavior and serving Kinosail's own UI does not invoke this GPL. Copying, modifying, bundling, or serving Jellyfin Web code/assets would be distribution of GPL material and needs a separate compliance and license-compatibility analysis. |
| Jellyfin Desktop (formerly Jellyfin Media Player) | [GNU GPL version 2](https://github.com/jellyfin/jellyfin-desktop/blob/4e1010b90ff3a8e0dca9026618ba12e6cee1ddd5/LICENSE); [repository/README](https://github.com/jellyfin/jellyfin-desktop/tree/4e1010b90ff3a8e0dca9026618ba12e6cee1ddd5) | The same GPL boundary applies: a user-run, unmodified desktop client communicating over HTTP does not relicense Kinosail; distributing or deriving from the client would trigger GPL obligations. |

The official [Android README](https://github.com/jellyfin/jellyfin-android/tree/5779ef02df46d1b57e23d72e83b65a4bc0f0a1ee#release-flavors) also distinguishes a proprietary build with Google Chromecast support from a libre build without it. The proprietary build is distributed through Google Play and Amazon. Its store binary should especially not be mirrored, extracted, or treated as entirely GPL-covered; users should obtain it from an official channel.

## What Kinosail may and should not do

| Action | Assessment |
| --- | --- |
| Tell a user to install the official app, enter the Kinosail URL, and use the unmodified app | Within the reviewed licenses. Kinosail and the app remain separate programs communicating over HTTP. |
| Independently implement compatible endpoints, JSON shapes, authentication headers, and playback behavior | Does not by itself copy a player. Keep the implementation independent and do not paste client/server source, generated SDK code, documentation text, or artwork into the PolyForm codebase without reviewing that material's license. |
| Keep Kinosail Server under PolyForm Perimeter 1.0.1 | Compatible with this arms-length model provided no GPL/MPL client code is included in Kinosail. GPL's copyleft and PolyForm's noncompete restriction then never apply to the same work. |
| Mirror an official APK/IPA/desktop binary, ship it with Kinosail, or publish a fork | Outside the model. Distribution invokes GPL/MPL duties, dependency licenses, platform-signing/store terms, and Jellyfin's mandatory fork rebranding rule. Android's proprietary Chromecast flavor adds a separate non-GPL component concern. |
| Bundle or serve Jellyfin Web from Kinosail | Outside the clean boundary. Jellyfin Web is GPLv2. Its source, notices, derivative/combination status, and compatibility with the surrounding PolyForm distribution would need counsel-level review. Use Kinosail's own independently written web UI instead. |

This memo does not treat API names and structures as categorically uncopyrightable. The U.S. Supreme Court's [Google v. Oracle opinion](https://www.supremecourt.gov/opinions/20pdf/18-956_d18f.pdf) assumed API copyrightability for argument's sake and decided fair use on the specific record. The conservative implementation rule is to reproduce only the protocol elements necessary for interoperability and not Jellyfin's expressive source implementation.

## Trademark, compatibility-token, support, and app-store boundary

Copyright licenses do not grant Jellyfin trademark rights. Jellyfin's [branding policy](https://jellyfin.org/docs/project/branding/) states that the name and logo are Jellyfin, Inc. trademarks, requires third-party projects to use their own logo, permits the name as an affix to describe compatibility, prohibits names that imply an official client, and requires any publicly distributed fork to use a different name and logo. Because Kinosail is not an instance of Jellyfin software, it should not rely on the policy's separate implicit branding permission for free Jellyfin instances.

Recommended public wording:

> Kinosail is independently developed and compatible with selected official Jellyfin clients. Kinosail is not affiliated with, endorsed by, or supported by Jellyfin, Inc. Jellyfin is a trademark of Jellyfin, Inc.

Use the Kinosail name and logo throughout the visible product, including `ServerName`. Do not call Kinosail "Jellyfin," "an official Jellyfin server," or use the Jellyfin logo. "Kinosail is compatible with selected Jellyfin clients" is the safer descriptive construction.

### Exact `ProductName` compatibility issue

The current clients add a narrower problem that the software licenses do not answer:

- Jellyfin for iOS accepts a server only when [`ProductName === "Jellyfin Server"`](https://github.com/jellyfin/jellyfin-ios/blob/2d0980aa5a25a1462a6255dd4422175d8f1c1dc1/utils/ServerValidator.js#L69-L89).
- The shared Android/Android TV SDK assigns a bad score to any other product name in [`RecommendedServerDiscovery.kt`](https://github.com/jellyfin/jellyfin-sdk-kotlin/blob/60be93850884ecf471c678ba8568e65af0acf19d/jellyfin-core/src/commonMain/kotlin/org/jellyfin/sdk/discovery/RecommendedServerDiscovery.kt#L26-L68). Android then rejects bad recommendations in [`ConnectionHelper.kt`](https://github.com/jellyfin/jellyfin-android/blob/5779ef02df46d1b57e23d72e83b65a4bc0f0a1ee/app/src/main/java/org/jellyfin/mobile/setup/ConnectionHelper.kt#L30-L63), and Android TV does the same in [`ServerRepository.kt`](https://github.com/jellyfin/jellyfin-androidtv/blob/a19736b4c89a8d1a76dff069d2ab9c68b7416f47/app/src/main/java/org/jellyfin/androidtv/auth/repository/ServerRepository.kt#L101-L138).
- Kinosail returns `"ProductName": "Kinosail Server"` in its [compatibility response](../../apps/player/internal/server/jellyfin.go). This preserves independent product identity but can fail current client identity checks.

That field is machine-readable and Kinosail remains visibly branded through `ServerName`, but the Jellyfin policy does not specifically bless this exact third-party use of the mark. It is not prudent to infer trademark permission from GPL/MPL or from the policy's allowance for descriptive compatibility wording. The clean resolution is a written permission narrowly covering the protocol token, or an accepted upstream change that separates protocol compatibility from product identity. If neither is available, unmodified current mobile/TV clients and strict Kinosail branding are in tension.

The [Google Play listing](https://play.google.com/store/apps/details?id=org.jellyfin.mobile) and [Apple App Store listing](https://apps.apple.com/us/app/jellyfin-mobile/id1480192618) both describe their app as the official companion and say a Jellyfin server is required. That is an upstream compatibility/support representation, not language found in the GPL or MPL grant. It does not promise support for Kinosail, prevent upstream from changing the protocol or adding server checks, or establish that every compatible Kinosail feature will continue to work. Kinosail should therefore claim only the client/version/features it has tested, direct users to Kinosail for support, and avoid implying Jellyfin endorsement. Jellyfin's [server policy](https://jellyfin.org/docs/general/community-standards/servers/) likewise tells operators to identify themselves as the support contact and not present themselves as the Jellyfin project.

## Practical launch guardrails

1. Keep clients user-installed from official stores or Jellyfin's download links; do not redistribute them.
2. Keep all Kinosail server and web presentation code independently authored and Kinosail-branded.
3. Maintain a tested client/version compatibility matrix and qualify compatibility claims by feature; licensing is not a compatibility guarantee.
4. If exact current mobile/TV compatibility is required, obtain written Jellyfin permission for the exact protocol token or get the hard-coded client check changed upstream.
5. Re-run this review before importing any Jellyfin code/assets, bundling a client or Jellyfin Web, distributing a fork, submitting an app, or offering a service branded around Jellyfin.
6. Obtain counsel review before commercial launch if the product plan moves beyond this narrow connection model, especially into client redistribution, a modified player, copied web assets, paid hosted media access, or reliance on the unresolved product-name token.
