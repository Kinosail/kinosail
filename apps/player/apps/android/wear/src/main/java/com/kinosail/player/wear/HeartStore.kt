package com.kinosail.player.wear

import android.content.Context
import com.kinosail.player.watchcore.HeartTimeline
import com.kinosail.player.watchcore.WatchPlayer

internal object HeartStore {
    private const val KEY = "movie_heart_v1"

    @Synchronized fun load(context: Context, nowMs: Long = System.currentTimeMillis()): HeartTimeline? {
        val saved = context.getSharedPreferences(KEY, Context.MODE_PRIVATE).getString(KEY, null) ?: return null
        val timeline = runCatching { HeartTimeline.decode(saved, nowMs) }.getOrNull()
        if (timeline == null) context.getSharedPreferences(KEY, Context.MODE_PRIVATE).edit().remove(KEY).commit()
        if (timeline?.tracking == true && nowMs - timeline.startedMs >= 8 * 60 * 60 * 1000L) {
            return timeline.copy(endedMs = timeline.startedMs + 8 * 60 * 60 * 1000L).also { save(context, it) }
        }
        return timeline
    }

    @Synchronized fun start(context: Context, player: WatchPlayer, nowMs: Long): HeartTimeline {
        val timeline = HeartTimeline.start(player, nowMs)
        save(context, timeline)
        return timeline
    }

    @Synchronized fun note(context: Context, player: WatchPlayer, nowMs: Long): HeartTimeline? {
        val current = load(context, nowMs) ?: return null
        val next = current.note(player, nowMs)
        if (next != current) save(context, next)
        return next
    }

    @Synchronized fun reading(context: Context, atMs: Long, bpm: Double, nowMs: Long): HeartTimeline? {
        val current = load(context, nowMs) ?: return null
        val next = current.addReading(atMs, bpm, nowMs)
        if (next != current) save(context, next)
        return next
    }

    @Synchronized fun stop(context: Context, nowMs: Long): HeartTimeline? {
        val current = load(context, nowMs) ?: return null
        if (!current.tracking) return current
        val next = current.copy(endedMs = nowMs.coerceAtLeast(current.startedMs))
        save(context, next)
        return next
    }

    private fun save(context: Context, timeline: HeartTimeline) {
        check(context.getSharedPreferences(KEY, Context.MODE_PRIVATE).edit()
            .putString(KEY, HeartTimeline.encode(timeline)).commit())
    }
}
