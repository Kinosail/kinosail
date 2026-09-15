# Kinosail

Kinosail is the product family for organizing and watching personal media from a server people control.

**Kinosail Player**:
The household-facing app for browsing, playing, and reading Library Content from a Kinosail Server.

**Kinosail Server**:
A user-controlled installation that indexes and serves one or more Libraries.
_Avoid_: Node, instance

**Supporter Badge**:
An app-specific honor that recognizes voluntary support without unlocking product capability.

**Patron Order**:
A permanent Supporter Badge for one Kinosail app and one support level.
_Avoid_: One-off badge, lifetime subscription

**Living Standard**:
An expiring Supporter Badge for active recurring support of one Kinosail app.
_Avoid_: Subscription add-on, badge overlay

**Service Mark**:
A tenure honor on a Living Standard earned through completed months of recurring support.

**Badge Case**:
The app-local record of collected Supporter Badge designs. Reaching a level adds that level and every lower design in its family.

**Masterwork**:
An app-specific fused honor earned when both supporter families are present. Its level equals the lower collected family level.

**Complete Fleet**:
A signed collection honor for support across every app named by its edition.
Living editions can follow the active app family; dated editions remain fixed.

## Language

**Update Manager**:
Installation-owned software that applies one verified Kinosail Server release and reports its result.
It runs only for update work and is not another Server.

**Release Manifest**:
The signed list of supported Kinosail Server release artifacts and their verified digests.

**Recovery Backup**:
An authenticated, encrypted snapshot of all private Server state created before replacement.
It excludes Library Content and cache data.
_Avoid_: Portable backup, media backup

**Library**:
An organized collection of user-owned media available through a Kinosail Server.
_Avoid_: Catalog, collection

**Viewer**:
A person authorized to browse or play content from a Library.
_Avoid_: Consumer, client

**Viewer Profile**:
A named, credentialed identity representing one Viewer and that Viewer’s viewing activity on a Kinosail Server.
_Avoid_: User, account

**Viewing Activity**:
A Viewer Profile’s watched state and resume position for playable Library Content.
_Avoid_: Server history

**Viewing Activity Import**:
A one-time transfer of compatible Viewing Activity, favorites, and Playlists from one source identity into one Viewer Profile.

**My List**:
A Viewer Profile’s private set of favorite Library Content.
_Avoid_: Watchlist

**Playlist**:
A Viewer Profile’s named, ordered grouping of Library Content.

**Viewing Activity Sync**:
A configured one-directional pull that keeps one Viewer Profile’s Viewing Activity current from another media server.
_Avoid_: Two-way sync

**Owner**:
The Viewer Profile allowed to administer a Kinosail Server, its Libraries, and other Viewer Profiles.
A Server may have multiple Owners and must always retain at least one.
_Avoid_: Admin user, superuser

**Direct Connection**:
An authenticated path between a Viewer and a Kinosail Server with no Kinosail-operated intermediary carrying Library Content.

**Verified Direct Connection**:
A Direct Connection carried over an Owner-paired WireGuard tunnel.

**Compatibility Connection**:
A Direct Connection authenticated through ordinary DNS and public HTTPS trust for third-party Jellyfin applications.

**Library Content**:
Media, artwork, subtitles, metadata, and viewing activity owned by or derived from a Library.
_Avoid_: Payload, customer data

**Movie**:
A standalone playable work in a Library.

**Show**:
An episodic work whose Episodes are grouped by season.
_Avoid_: Series

**Episode**:
A playable installment of a Show identified by season and episode number.

**Chapter**:
A named time range within playable Library Content that a Viewer can navigate to.

**Playback Marker**:
A meaningful time range, such as an intro or credits, that a Viewer may skip.

**Collection**:
A named grouping of Library Content curated by an Owner or derived from provider box-set metadata.

**Watch Room**:
An expiring, direct Server session where one Viewer leads synchronized playback for other authenticated Viewers.

**Profile Policy**:
Server-enforced limits on a Viewer’s libraries, ratings, downloads, transcoding, managed remote access, and viewing hours.

**Quick Connect**:
A short-lived, one-time exchange that lets an authenticated Viewer authorize a named device without typing a password on it.

**Media Share**:
An Owner-created, expiring grant to stream explicitly selected Library Content without browsing the surrounding Library. A Media Share is revocable, device- and concurrency-limited, and never grants Owner or download access.

**Single Sign-On**:
Optional OpenID Connect authentication that links a verified provider subject to a local Profile while preserving password access.

**SCIM Provisioning**:
Directory-driven synchronization of SCIM-managed Viewer Profiles on a Kinosail Server.
_Avoid_: Cloud account synchronization, media synchronization

**SCIM-managed Viewer Profile**:
A Viewer Profile whose identity and lifecycle are controlled by an external directory through SCIM.
_Avoid_: Cloud user, directory account
