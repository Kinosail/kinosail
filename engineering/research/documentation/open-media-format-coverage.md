# Open media format coverage

This note defines the format boundary for Kinosail's broad media support. It
covers format and implementation licensing only. Library Content rights remain
the responsibility of the person who stores and streams that content.

## Safe additions

The current implementation adds direct discovery for AIFF (`.aif`, `.aiff`), Ogg
audio (`.oga`), AVIF (`.avif`), and bitmap images (`.bmp`). These formats need
no new decoder dependency in Kinosail; the existing browser and FFmpeg paths
handle them where the client supports the format.

FLAC states that its format and specification are open for commercial and
noncommercial use, and its reference libraries use the New BSD License:
<https://www.xiph.org/flac/license.html>.

Opus provides a BSD-licensed reference implementation and royalty-free patent
permissions, subject to its defensive-termination condition:
<https://www.opus-codec.org/license/>.

AVIF is an AOMedia format. AOMedia publishes its software under permissive
licenses and its working-group patent policy under the Open Media Patent
License:
<https://aomedia.org/about/legal/>.

## Deliberately excluded

Kinosail does not add HEIC/HEIF, WMA, MOBI/AZW, CBR/RAR, or camera RAW formats
in this change. Those formats need separate decoder, patent, or distribution
review before they can enter the runtime image.

AAC support already exists in Kinosail. New AAC encoding should not be added
without review because Via Licensing continues to operate an AAC patent
licensing program:
<https://www.via-la.com/licensing-programs/aac/>.

Fraunhofer states that the former Technicolor/Fraunhofer MP3 licensing program
ended in 2017:
<https://www.iis.fraunhofer.de/en/ff/amm/consumer-electronics/mp3.html>.

## Distribution boundary

Kinosail must preserve the notices for its existing Jellyfin FFmpeg runtime and
base packages. A commercial distribution review must inventory those package
licenses and every later dependency change. See `THIRD_PARTY_NOTICES.md`.
