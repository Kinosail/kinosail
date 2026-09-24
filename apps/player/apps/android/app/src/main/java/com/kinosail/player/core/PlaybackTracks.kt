package com.kinosail.player.core

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.media3.common.C
import androidx.media3.common.TrackSelectionOverride
import androidx.media3.common.Tracks
import androidx.media3.exoplayer.ExoPlayer
import java.util.Locale

internal data class PlaybackTrack(val group: Tracks.Group, val indices: List<Int>,
                                  val label: String, val selected: Boolean)

internal class PlaybackTracks {
    var audio by mutableStateOf<List<PlaybackTrack>>(emptyList())
        private set
    var text by mutableStateOf<List<PlaybackTrack>>(emptyList())
        private set
    var captionsEnabled by mutableStateOf(false)
        private set
    private var lastTextGroup: androidx.media3.common.TrackGroup? = null

    fun prepare(engine: ExoPlayer, defaultCaptions: Boolean) {
        clear()
        engine.trackSelectionParameters = engine.trackSelectionParameters.buildUpon()
            .clearOverridesOfType(C.TRACK_TYPE_AUDIO)
            .clearOverridesOfType(C.TRACK_TYPE_TEXT)
            .setTrackTypeDisabled(C.TRACK_TYPE_TEXT, !defaultCaptions)
            .setSelectTextByDefault(defaultCaptions).build()
    }

    fun update(tracks: Tracks) {
        audio = choices(tracks, C.TRACK_TYPE_AUDIO, "Audio")
        text = choices(tracks, C.TRACK_TYPE_TEXT, "Captions")
        captionsEnabled = tracks.isTypeSelected(C.TRACK_TYPE_TEXT)
        text.firstOrNull { it.selected }?.let { lastTextGroup = it.group.mediaTrackGroup }
    }

    fun selectAudio(engine: ExoPlayer, choice: PlaybackTrack) {
        if (audio.none { it.sameTrack(choice) }) return
        engine.trackSelectionParameters = engine.trackSelectionParameters.buildUpon()
            .setOverrideForType(TrackSelectionOverride(choice.group.mediaTrackGroup, choice.indices))
            .build()
    }

    fun selectText(engine: ExoPlayer, choice: PlaybackTrack?) {
        if (choice != null && text.none { it.sameTrack(choice) }) return
        if (choice != null) lastTextGroup = choice.group.mediaTrackGroup
        val builder = engine.trackSelectionParameters.buildUpon()
            .clearOverridesOfType(C.TRACK_TYPE_TEXT)
            .setTrackTypeDisabled(C.TRACK_TYPE_TEXT, choice == null)
        if (choice != null) builder.setOverrideForType(
            TrackSelectionOverride(choice.group.mediaTrackGroup, listOf(choice.indices.first())))
        engine.trackSelectionParameters = builder.build()
    }

    fun toggleCaptions(engine: ExoPlayer) {
        if (captionsEnabled) selectText(engine, null)
        else selectText(engine, text.firstOrNull { it.group.mediaTrackGroup == lastTextGroup } ?: text.firstOrNull())
    }

    fun clear() {
        audio = emptyList()
        text = emptyList()
        captionsEnabled = false
        lastTextGroup = null
    }

    companion object {
        internal fun choices(tracks: Tracks, type: Int, fallback: String): List<PlaybackTrack> =
            tracks.groups.filter { it.type == type }.mapNotNull { group ->
                val indices = (0 until group.length).filter(group::isTrackSupported)
                if (indices.isEmpty()) return@mapNotNull null
                val format = group.getTrackFormat(indices.first())
                val name = (format.label ?: format.language?.takeUnless { it == "und" }
                    ?.let { Locale.forLanguageTag(it.take(32)).displayLanguage }).orEmpty()
                    .take(256).filterNot { it.isISOControl() || Character.getType(it) == Character.FORMAT.toInt() }
                    .trim().take(80).ifEmpty { fallback }
                PlaybackTrack(group, indices, name, group.isSelected)
            }.mapIndexed { index, choice -> choice.copy(label = "${index + 1}. ${choice.label}") }

        private fun PlaybackTrack.sameTrack(other: PlaybackTrack) =
            group.mediaTrackGroup == other.group.mediaTrackGroup && indices == other.indices
    }
}
