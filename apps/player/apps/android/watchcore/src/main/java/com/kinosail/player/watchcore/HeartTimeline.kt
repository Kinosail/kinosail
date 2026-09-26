package com.kinosail.player.watchcore

import com.fasterxml.jackson.core.JsonFactory
import com.fasterxml.jackson.core.StreamReadConstraints
import com.fasterxml.jackson.core.StreamReadFeature
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

@Serializable
data class HeartAnchor(val timeMs: Long, val position: Double, val playing: Boolean)

@Serializable
data class HeartPoint(val timeMs: Long, val position: Double, val bpm: Double)

@Serializable
data class HeartTimeline(
    val targetId: String, val itemId: String, val title: String, val duration: Double,
    val startedMs: Long, val endedMs: Long? = null,
    val anchors: List<HeartAnchor>, val points: List<HeartPoint> = emptyList(),
) {
    val tracking get() = endedMs == null
    val peak get() = points.maxByOrNull(HeartPoint::bpm)

    fun checked(nowMs: Long = System.currentTimeMillis()): HeartTimeline {
        require(WATCH_TARGET.matches(targetId) && HEART_ITEM.matches(itemId) &&
            title.isNotBlank() && title.toByteArray().size <= 256 && title.none(Char::isISOControl) &&
            duration.isFinite() && duration in 1.0..1_000_000_000.0 &&
            startedMs in 1..nowMs && (endedMs == null || endedMs in startedMs..nowMs) &&
            (endedMs ?: startedMs + MAX_TIME_MS) - startedMs <= MAX_TIME_MS && anchors.size in 1..3_000 &&
            points.size <= 6_000) { INVALID_HEART }
        var previous = startedMs
        for (anchor in anchors) {
            require(anchor.timeMs in previous..(endedMs ?: minOf(nowMs, startedMs + MAX_TIME_MS)) &&
                anchor.position.isFinite() && anchor.position in 0.0..duration) { INVALID_HEART }
            previous = anchor.timeMs
        }
        previous = startedMs
        for (point in points) {
            require(point.timeMs in previous..(endedMs ?: minOf(nowMs, startedMs + MAX_TIME_MS)) && point.position.isFinite() &&
                point.position in 0.0..duration && point.bpm.isFinite() && point.bpm in 25.0..250.0) { INVALID_HEART }
            previous = point.timeMs
        }
        return this
    }

    fun note(player: WatchPlayer, atMs: Long): HeartTimeline {
        if (!tracking || player.id != targetId || atMs < startedMs || atMs < anchors.last().timeMs) return this
        if (player.itemId != itemId || !player.active || player.position > duration || atMs - startedMs >= MAX_TIME_MS)
            return copy(endedMs = atMs.coerceAtMost(startedMs + MAX_TIME_MS))
        return copy(anchors = (if (anchors.size >= 3_000) anchors.drop(1) else anchors) +
            HeartAnchor(atMs, player.position, player.playing))
    }

    fun addReading(atMs: Long, bpm: Double, nowMs: Long): HeartTimeline {
        if (atMs !in startedMs..(endedMs ?: minOf(nowMs, startedMs + MAX_TIME_MS)) || atMs > nowMs ||
            !bpm.isFinite() || bpm !in 25.0..250.0 || points.size >= 6_000 ||
            points.any { it.timeMs == atMs }) return this
        val anchor = anchors.lastOrNull { it.timeMs <= atMs } ?: return this
        if (!anchor.playing || atMs - anchor.timeMs > 15_000) return this
        val position = anchor.position + (atMs - anchor.timeMs) / 1000.0
        if (position !in 0.0..duration) return this
        return copy(points = (points + HeartPoint(atMs, position, bpm)).sortedBy(HeartPoint::timeMs))
    }

    companion object {
        private const val MAX_TIME_MS = 8 * 60 * 60 * 1000L
        private const val INVALID_HEART = "Invalid heart timeline."
        private val WATCH_TARGET = Regex("phone|[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}")
        private val HEART_ITEM = Regex("[A-Za-z0-9_-]{1,128}")
        private val json = Json { ignoreUnknownKeys = false; isLenient = false }
        private val parser = JsonFactory.builder().enable(StreamReadFeature.STRICT_DUPLICATE_DETECTION)
            .streamReadConstraints(StreamReadConstraints.builder().maxNestingDepth(6)
                .maxStringLength(4096).maxNumberLength(32).build()).build()

        fun start(player: WatchPlayer, nowMs: Long): HeartTimeline {
            player.checked()
            require(player.active && !player.audio && player.duration > 0 && nowMs > 0) { INVALID_HEART }
            return HeartTimeline(player.id, player.itemId, player.title, player.duration, nowMs,
                anchors = listOf(HeartAnchor(nowMs, player.position, player.playing))).checked(nowMs)
        }

        fun decode(text: String, nowMs: Long): HeartTimeline {
            require(text.toByteArray().size <= 1_000_000) { INVALID_HEART }
            parser.createParser(text).use { stream -> while (stream.nextToken() != null) {} }
            return json.decodeFromString<HeartTimeline>(text).checked(nowMs)
        }

        fun encode(timeline: HeartTimeline): String = json.encodeToString(timeline.checked()).also {
            require(it.toByteArray().size <= 1_000_000) { INVALID_HEART }
        }
    }
}
