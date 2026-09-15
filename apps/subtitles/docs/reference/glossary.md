---
title: Glossary
description: Learn the exact Kinosail terms used across the app and documentation.
section: Reference
last_reviewed: 2026-08-28
---

# Glossary

Use these terms consistently when you describe Kinosail behavior.

| Term | Meaning |
| --- | --- |
| API | The versioned JSON interface at `/api/v1`. |
| Automatic | A playback mode that starts direct when possible and can offer a compatible fallback. |
| Compatible | The internal playback value for the `Compatibility` mode, which asks the Server for a compatible HLS representation. |
| Direct | A connection or playback path that sends media from the owner-hosted Server without a Kinosail-operated relay. Direct playback does not transform the source. |
| Direct media | Library Content served from the owner-hosted Server. It is not a cloud copy or hosted proxy stream. |
| Episode | A video item grouped into a Show and Season by its path or metadata. |
| GUI | The Kinosail web interface. Owner settings saved there are one configuration source. |
| Library | A configured root containing media files. A Server can have one or more Libraries. |
| Library Content | The media files and related local sidecars that the Owner mounts for scanning. Kinosail writes only validated subtitle sidecars beside this content. |
| MCP | Model Context Protocol. Kinosail's optional authenticated agent connection. |
| Owner | A Profile with Server administration rights. A Server must retain at least one Owner. |
| Profile | A local identity with library, rating, time, remote, transcoding, and download policy. A Profile can be an Owner or Viewer. |
| Remux | A playback conversion that changes the container without converting the media streams. |
| Server | The single Kinosail Server container and its application state. |
| Show | A browsing group for Episodes and Seasons. |
| Transcode | A playback conversion that changes one or more media streams, often to H.264 video and AAC audio. |
| Viewer | A non-Owner Profile that can use only the Libraries and capabilities allowed by its policy. |
| Viewing Activity | Local progress, watched state, ratings, history, and related playback records. |
| Watch Room | A synchronized viewing session for authenticated Viewers. |
| YAML | The optional human-readable deployment configuration file. Environment values override YAML values. |

## Similar terms

Do not use “cloud streaming” for public HTTPS. Public HTTPS is direct access to the Owner's Server. Do not call a file extension a playback guarantee. Ingest support and client playback support are separate; see [Media compatibility]({{ '/reference/media-compatibility/' | relative_url }}).
