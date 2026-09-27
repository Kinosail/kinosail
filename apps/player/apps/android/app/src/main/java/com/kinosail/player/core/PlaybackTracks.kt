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

    fun prepare(engine: ExoPlayer, defaultCaptions: Boolean, limitedLanguage: String? = null) {
        clear()
        val parameters = engine.trackSelectionParameters.buildUpon()
            .clearOverridesOfType(C.TRACK_TYPE_AUDIO)
            .clearOverridesOfType(C.TRACK_TYPE_TEXT)
            .setTrackTypeDisabled(C.TRACK_TYPE_TEXT, !defaultCaptions)
            .setSelectTextByDefault(defaultCaptions && limitedLanguage == null)
        if (limitedLanguage != null) parameters.setPreferredTextLanguage(limitedLanguage)
            .setSelectUndeterminedTextLanguage(false)
        engine.trackSelectionParameters = parameters.build()
    }

    fun update(tracks: Tracks, limitedLanguage: String? = null) {
        audio = choices(tracks, C.TRACK_TYPE_AUDIO, "Audio")
        text = if (limitedLanguage == null) choices(tracks, C.TRACK_TYPE_TEXT, "Captions")
            else limitedTextChoices(tracks, limitedLanguage)
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
                PlaybackTrack(group, indices, label(group, indices.first(), fallback), group.isSelected)
            }.mapIndexed { index, choice -> choice.copy(label = "${index + 1}. ${choice.label}") }

        private fun limitedTextChoices(tracks: Tracks, language: String): List<PlaybackTrack> {
            var regularShown = false
            return tracks.groups.filter { it.type == C.TRACK_TYPE_TEXT }.flatMap { group ->
                (0 until group.length).mapNotNull { index ->
                    if (!group.isTrackSupported(index) || !sameLanguage(group.getTrackFormat(index).language, language))
                        return@mapNotNull null
                    val forced = group.getTrackFormat(index).selectionFlags and C.SELECTION_FLAG_FORCED != 0
                    if (regularShown && !forced) return@mapNotNull null
                    regularShown = regularShown || !forced
                    PlaybackTrack(group, listOf(index), label(group, index, "Captions"), group.isTrackSelected(index))
                }
            }.mapIndexed { index, choice -> choice.copy(label = "${index + 1}. ${choice.label}") }
        }

        private fun sameLanguage(actual: String?, preferred: String): Boolean = try {
            actual != null && actual.length <= 32 && actual.matches(Regex("[A-Za-z]{2,3}([_-][A-Za-z0-9]{2,8}){0,3}")) &&
                Locale.forLanguageTag(actual.replace('_', '-')).getISO3Language() == Locale.forLanguageTag(preferred).getISO3Language()
        } catch (_: Exception) { false }

        private fun label(group: Tracks.Group, index: Int, fallback: String): String {
            val format = group.getTrackFormat(index)
            return (format.label ?: format.language?.takeUnless { it == "und" }
                ?.let { Locale.forLanguageTag(it.take(32)).displayLanguage }).orEmpty()
                .take(256).filterNot { it.isISOControl() || Character.getType(it) == Character.FORMAT.toInt() }
                .trim().take(80).ifEmpty { fallback }
        }

        private fun PlaybackTrack.sameTrack(other: PlaybackTrack) =
            group.mediaTrackGroup == other.group.mediaTrackGroup && indices == other.indices
    }
}
