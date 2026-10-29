package com.kinosail.player.core

import kotlin.math.max

data class PlaybackMarker(val type: String, val start: Double, val end: Double)

internal sealed interface PlaybackPrompt {
    val title: String
    data class Skip(override val title: String, val to: Double) : PlaybackPrompt
    data object Next : PlaybackPrompt { override val title = "Next Episode" }
}

internal object PlaybackMarkers {
    val TYPES = setOf("intro", "recap", "commercial", "outro", "credits")
    private val SKIPPABLE = setOf("intro", "recap", "commercial")
    const val AUTOMATIC_EPISODE_LIMIT = 3

    fun prompt(source: PlaybackSource, now: Double, completed: Boolean): PlaybackPrompt? {
        val duration = source.duration
        if (duration <= 0) return null
        if (!completed) source.markers.firstOrNull {
            it.type in SKIPPABLE && it.type !in source.autoSkip && now >= it.start && now < it.end - 1
        }?.let {
            return PlaybackPrompt.Skip(when (it.type) { "intro" -> "Skip Intro"; "recap" -> "Skip Recap"; else -> "Skip" }, it.end)
        }
        if (source.nextItemId == null) return null
        val ending = source.markers.filter { (it.type == "credits" || it.type == "outro") && it.start > duration / 2 }
            .minOfOrNull { it.start } ?: max(duration / 2, duration - 30)
        return if (completed || now >= ending) PlaybackPrompt.Next else null
    }

    fun automatic(source: PlaybackSource, now: Double): PlaybackMarker? =
        source.markers.firstOrNull { it.type in source.autoSkip && now >= it.start && now < it.end }

    fun finishesEpisode(source: PlaybackSource, marker: PlaybackMarker) =
        marker.end >= source.duration - 1 && source.nextItemId != null
}
