# Kinosail Subtitles

Kinosail Subtitles maintains subtitle sidecar files for media stored on an owner-controlled Server.

## Language

**Kinosail Subtitles Server**:
An owner-controlled installation that scans Media Libraries and writes approved Subtitle Files.
_Avoid_: Node, instance, cloud service

**Owner**:
The authenticated person allowed to configure Libraries, providers, languages, automation, and file changes.
_Avoid_: Admin user, superuser

**Media Library**:
One configured folder tree containing owner-controlled Movies or Episodes.
_Avoid_: Catalog, collection

**Media File**:
A scanned video file that can have one or more Subtitle Files.
_Avoid_: Stream, asset

**Movie**:
A standalone Media File identified by its title, year, and available provider identifiers.

**Show**:
An episodic work that groups Episodes.
_Avoid_: Series

**Episode**:
A Media File identified by its Show, season number, and episode number.

**Subtitle File**:
A validated SRT or WebVTT sidecar stored beside its Media File.
_Avoid_: Caption blob, provider payload

**Preferred Language**:
The two- or three-letter language code an Owner wants each Media File to cover.

**Default Subtitle**:
An untagged Subtitle File whose base name exactly matches its Media File.

**Language Subtitle**:
A Subtitle File with a language tag between its media base name and extension, such as `Arrival.en.srt`.

**Ready Item**:
A Media File with a Default Subtitle or a Language Subtitle matching the Preferred Language.
_Avoid_: Complete item

**Wanted Item**:
A Media File without coverage for the Preferred Language.
_Avoid_: Missing media

**Subtitle Provider**:
An Owner-enabled external service used to search for and download subtitle candidates.

**Subtitle Candidate**:
One provider result that can be validated and considered for a Media File.

**Sidecar Write**:
A new Subtitle File placed beside its Media File after provider and content validation succeeds. It never replaces a file already owned by the household.
_Avoid_: Cache download

**Wanted Search**:
An Owner request to find Subtitle Candidates for selected Wanted Items and report the outcome for each item.
_Avoid_: Search all

**Patron Order**:
A permanent supporter badge for one Kinosail app and one paid level. Reaching a level records that design and every lower design.
_Avoid_: One-time tier, cumulative badge

**Living Standard**:
A supporter badge for one Kinosail app and one active monthly level. An expired Living Standard remains an archived honor, not an active entitlement.
_Avoid_: Subscription tier, recurring badge

**Badge Case**:
The app-local record of collected supporter designs in both families.

**Masterwork**:
An app-specific fused honor earned when both families are present. Its level equals the lower collected family level.

**Complete Fleet**:
A signed supporter honor that covers a named set of Kinosail apps. A Living edition follows configured apps while active; a dated Patron edition stays fixed.
_Avoid_: Bundle flag, all-app badge

**Share Certificate**:
A downloadable badge record that excludes private activation and installation data. It can include an explicit public recognition name at levels 7–10.
_Avoid_: Supporter certificate, activation certificate
