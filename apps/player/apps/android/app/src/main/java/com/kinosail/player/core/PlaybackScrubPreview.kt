package com.kinosail.player.core

import android.graphics.Bitmap
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.media3.common.util.UnstableApi
import androidx.media3.ui.PlayerView
import androidx.media3.ui.TimeBar
import com.kinosail.player.design.KinoColor
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.async

internal class ScrubFrames(private val scope: CoroutineScope, private val fetch: suspend (Int) -> Bitmap?) {
    private val frames = object : LinkedHashMap<Int, Bitmap>(64, 0.75f, true) {
        override fun removeEldestEntry(eldest: MutableMap.MutableEntry<Int, Bitmap>?) = size > 120
    }
    private val loading = mutableMapOf<Int, Deferred<Bitmap?>>()
    private var last: Int? = null

    fun cached(second: Int): Bitmap? = frames[second]

    suspend fun load(second: Int, limit: Int): Bitmap? {
        val step = last?.let { second - it }?.takeIf { it != 0 } ?: 10
        last = second
        val wanted = frames[second]?.let { null } ?: start(second)
        for (offset in listOf(1, 2, 3, -1)) {
            val next = second + offset * step
            if (next in 0..limit && loading.size < 4 && frames[next] == null) start(next)
        }
        return frames[second] ?: wanted?.await()
    }

    private fun start(second: Int): Deferred<Bitmap?> = loading.getOrPut(second) {
        scope.async {
            try {
                runCatching { fetch(second) }.getOrNull()?.also { frames[second] = it }
            } finally {
                loading.remove(second)
            }
        }
    }
}

private data class ScrubState(val positionMs: Long, val durationMs: Long, val barStart: Int, val barWidth: Int, val barTop: Int)

@androidx.annotation.OptIn(markerClass = [UnstableApi::class])
@Composable
internal fun ScrubPreview(playerView: PlayerView?, playback: PlaybackModel, tv: Boolean) {
    if (playerView == null || !playback.scrubPreviews) return
    val scope = rememberCoroutineScope()
    val frames = remember(playerView, playback.activeItemId) { ScrubFrames(scope) { playback.scrubFrame(it) } }
    var scrub by remember(playerView) { mutableStateOf<ScrubState?>(null) }
    var image by remember(playerView) { mutableStateOf<Bitmap?>(null) }
    var failed by remember(playerView) { mutableStateOf(false) }
    DisposableEffect(playerView) {
        val bar = playerView.findViewById<android.view.View>(androidx.media3.ui.R.id.exo_progress) as? TimeBar
        val listener = object : TimeBar.OnScrubListener {
            fun update(position: Long) {
                val view = bar as? android.view.View ?: return
                val player = IntArray(2).also(playerView::getLocationInWindow)
                val track = IntArray(2).also(view::getLocationInWindow)
                scrub = ScrubState(position, playerView.player?.duration?.takeIf { it > 0 } ?: 0L,
                    track[0] - player[0] + view.paddingLeft, view.width - view.paddingLeft - view.paddingRight, track[1] - player[1])
            }
            override fun onScrubStart(timeBar: TimeBar, position: Long) { image = null; failed = false; update(position) }
            override fun onScrubMove(timeBar: TimeBar, position: Long) = update(position)
            override fun onScrubStop(timeBar: TimeBar, position: Long, canceled: Boolean) { scrub = null; image = null; failed = false }
        }
        bar?.addListener(listener)
        onDispose { bar?.removeListener(listener) }
    }
    val state = scrub ?: return
    val end = if (state.durationMs > 0) (state.durationMs - 1) / 1000 else 43_200
    val second = (state.positionMs / 1000).coerceIn(0, minOf(end, 43_200)).toInt() / 10 * 10
    LaunchedEffect(second) {
        frames.cached(second)?.let { image = it; failed = false; return@LaunchedEffect }
        val frame = frames.load(second, minOf(end, 43_200L).toInt())
        image = frame
        failed = frame == null
    }
    val width = if (tv) 240.dp else 160.dp
    val height = if (tv) 135.dp else 90.dp
    val density = LocalDensity.current
    val fraction = if (state.durationMs > 0) state.positionMs.toFloat() / state.durationMs else 0f
    val center = state.barStart + (state.barWidth * fraction.coerceIn(0f, 1f)).toInt()
    val half = with(density) { width.roundToPx() } / 2
    val lift = with(density) { (height + 40.dp).roundToPx() }
    val left = (center - half).coerceIn(0, maxOf(0, playerView.width - half * 2))
    val clock = mediaClock(state.positionMs / 1000.0)
    Column(Modifier.offset { IntOffset(left, maxOf(0, state.barTop - lift)) }
        .semantics(mergeDescendants = true) { contentDescription = "Preview at $clock" },
        horizontalAlignment = Alignment.CenterHorizontally) {
        Box(Modifier.size(width, height).clip(RoundedCornerShape(8.dp)).background(Color.Black),
            contentAlignment = Alignment.Center) {
            val frame = image
            when {
                frame != null -> Image(frame.asImageBitmap(), null, Modifier.size(width, height), contentScale = ContentScale.Fit)
                failed -> Text("Preview unavailable", color = KinoColor.text, style = MaterialTheme.typography.labelMedium)
                else -> Box(Modifier.size(width, height).background(KinoColor.raised))
            }
        }
        Text(clock, color = KinoColor.text, style = MaterialTheme.typography.labelLarge,
            modifier = Modifier.background(Color.Black.copy(alpha = 0.6f), RoundedCornerShape(4.dp)))
    }
}
