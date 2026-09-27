package com.kinosail.player.core

import androidx.media3.common.C
import androidx.media3.common.Format
import androidx.media3.common.MimeTypes
import androidx.media3.common.TrackGroup
import androidx.media3.common.Tracks
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class PlaybackTracksTest {
    @Test fun listsSupportedAudioAndCaptionsWithCurrentSelection() {
        val english = group(MimeTypes.AUDIO_AAC, "English", true)
        val french = group(MimeTypes.AUDIO_AAC, "Français", false)
        val captions = group(MimeTypes.TEXT_VTT, "English CC", true)
        val unsupported = group(MimeTypes.TEXT_VTT, "Unsupported", false, C.FORMAT_UNSUPPORTED_TYPE)
        val tracks = PlaybackTracks().apply { update(Tracks(listOf(english, french, captions, unsupported))) }

        assertEquals(listOf("1. English", "2. Français"), tracks.audio.map { it.label })
        assertTrue(tracks.audio.first().selected)
        assertFalse(tracks.audio.last().selected)
        assertEquals(listOf("1. English CC"), tracks.text.map { it.label })
        assertTrue(tracks.captionsEnabled)
        assertEquals(listOf(0), tracks.audio.first().indices)
    }

    @Test fun boundsUntrustedTrackLabelsAndClearsPreviousMedia() {
        val label = "Hi\u202e\u0000\n" + "X".repeat(120)
        val tracks = PlaybackTracks().apply {
            update(Tracks(listOf(group(MimeTypes.TEXT_VTT, label, false))))
        }
        assertEquals("1. Hi" + "X".repeat(78), tracks.text.single().label)
        assertFalse(tracks.captionsEnabled)

        tracks.clear()
        assertTrue(tracks.audio.isEmpty())
        assertTrue(tracks.text.isEmpty())
        assertFalse(tracks.captionsEnabled)
    }

    @Test fun limitsEmbeddedMenuToPreferredRegularAndForcedTracks() {
        val available = Tracks(listOf(
            group(MimeTypes.TEXT_VTT, "English", false, language = "en"),
            group(MimeTypes.TEXT_VTT, "Dutch", true, language = "nld"),
            group(MimeTypes.TEXT_VTT, "English alternate", false, language = "eng"),
            group(MimeTypes.TEXT_VTT, "English forced", false, language = "eng", flags = C.SELECTION_FLAG_FORCED),
            group(MimeTypes.TEXT_VTT, "Unknown", false),
            group(MimeTypes.TEXT_VTT, "English forced regional", false, language = "en-Latn-US", flags = C.SELECTION_FLAG_FORCED),
            group(MimeTypes.TEXT_VTT, "Invalid language", false, language = "en" + "x".repeat(40)),
        ))
        val tracks = PlaybackTracks().apply { update(available, "en") }
        assertEquals(listOf("1. English", "2. English forced", "3. English forced regional"), tracks.text.map { it.label })
        assertTrue(tracks.captionsEnabled)
        assertFalse(tracks.text.any { it.selected })
        tracks.update(available)
        assertEquals(7, tracks.text.size)
    }

    private fun group(mime: String, label: String, selected: Boolean,
                      support: Int = C.FORMAT_HANDLED, language: String? = null, flags: Int = 0): Tracks.Group {
        val format = Format.Builder().setSampleMimeType(mime).setLabel(label).setLanguage(language)
            .setSelectionFlags(flags).build()
        return Tracks.Group(TrackGroup(format), false, intArrayOf(support), booleanArrayOf(selected))
    }
}
