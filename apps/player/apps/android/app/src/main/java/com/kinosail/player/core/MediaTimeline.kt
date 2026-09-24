package com.kinosail.player.core

import kotlin.math.abs
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.doubleOrNull

data class OmittedRange(val start: Double, val end: Double)

class MediaTimeline(val sourceDuration: Double, val duration: Double,
                    val omitted: List<OmittedRange> = emptyList()) {
    init {
        require(sourceDuration.isFinite() && sourceDuration in 0.0..31_536_000.0 &&
            duration.isFinite() && duration in 0.0..31_536_000.0 && omitted.size <= 128) {
            "Invalid playback timeline."
        }
        var last = 0.0
        var removed = 0.0
        omitted.forEach {
            require(it.start.isFinite() && it.end.isFinite() && it.start >= last &&
                it.end > it.start && it.end <= sourceDuration) { "Invalid playback timeline." }
            last = it.end
            removed += it.end - it.start
        }
        require(abs(sourceDuration - removed - duration) < 0.01) { "Invalid playback timeline." }
    }

    fun sourceTime(position: Double): Double {
        val safe = position.coerceIn(0.0, duration)
        var removed = 0.0
        omitted.forEach {
            if (safe < it.start - removed) return (safe + removed).coerceAtMost(sourceDuration)
            removed += it.end - it.start
        }
        return (safe + removed).coerceAtMost(sourceDuration)
    }

    fun presentationTime(position: Double): Double {
        val safe = position.coerceIn(0.0, sourceDuration)
        var removed = 0.0
        omitted.forEach {
            if (safe < it.start) return safe - removed
            if (safe < it.end) return it.start - removed
            removed += it.end - it.start
        }
        return (safe - removed).coerceAtLeast(0.0)
    }

    companion object {
        fun parse(raw: JsonElement): MediaTimeline {
            val value = raw as? JsonObject
            require(value != null && value.keys.all { it in setOf("sourceDuration", "duration", "omitted") } &&
                value.keys.containsAll(setOf("sourceDuration", "duration"))) { "Invalid playback timeline." }
            fun number(rawNumber: JsonElement?): Double {
                val primitive = rawNumber as? JsonPrimitive
                require(primitive != null && !primitive.isString) { "Invalid playback timeline." }
                return primitive.doubleOrNull ?: throw IllegalArgumentException("Invalid playback timeline.")
            }
            val ranges = value["omitted"]?.let {
                val array = it as? JsonArray
                require(array != null && array.size <= 128) { "Invalid playback timeline." }
                array.map { entry ->
                    val range = entry as? JsonObject
                    require(range != null && range.keys == setOf("start", "end")) { "Invalid playback timeline." }
                    OmittedRange(number(range["start"]), number(range["end"]))
                }
            } ?: emptyList()
            return MediaTimeline(number(value["sourceDuration"]), number(value["duration"]), ranges)
        }
    }
}
