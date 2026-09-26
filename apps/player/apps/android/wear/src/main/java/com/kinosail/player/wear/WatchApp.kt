package com.kinosail.player.wear

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.wear.compose.foundation.pager.HorizontalPager
import androidx.wear.compose.foundation.pager.rememberPagerState
import androidx.wear.compose.material3.AppScaffold
import androidx.wear.compose.material3.Button
import androidx.wear.compose.material3.ButtonDefaults
import androidx.wear.compose.material3.ColorScheme
import androidx.wear.compose.material3.HorizontalPagerScaffold
import androidx.wear.compose.material3.MaterialTheme
import androidx.wear.compose.material3.Text
import com.kinosail.player.watchcore.HeartPoint
import com.kinosail.player.watchcore.HeartTimeline
import com.kinosail.player.watchcore.WatchPlayer
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private val ink = Color(0xFF0B0D0B)
private val signal = Color(0xFFC4FF47)
private val muted = Color(0xFFA0A79C)
private val raised = Color(0xFF20271E)

@Composable
internal fun WatchApp(
    remote: WearRemoteSession, timeline: HeartTimeline?, heartMessage: String?,
    onStartHeart: (WatchPlayer) -> Unit, onStopHeart: () -> Unit, onRefreshHeart: () -> Unit,
) {
    LaunchedEffect(remote) {
        while (true) {
            remote.refresh()
            onRefreshHeart()
            delay(5_000)
        }
    }
    val pager = rememberPagerState(pageCount = { if (timeline == null) 1 else 2 })
    val scope = rememberCoroutineScope()
    MaterialTheme(colorScheme = ColorScheme(primary = signal, onPrimary = Color(0xFF142000),
        background = ink, onBackground = Color(0xFFF6F8F2), onSurface = Color(0xFFF6F8F2),
        surfaceContainer = raised, onSurfaceVariant = muted)) {
        AppScaffold {
            HorizontalPagerScaffold(pagerState = pager) {
                HorizontalPager(state = pager) { page ->
                    if (page == 0) RemotePage(remote, timeline, heartMessage, onStartHeart) {
                        scope.launch { pager.animateScrollToPage(1) }
                    }
                    else if (timeline != null) HeartPage(timeline, heartMessage, onStopHeart)
                }
            }
        }
    }
}

@Composable
private fun RemotePage(
    remote: WearRemoteSession, timeline: HeartTimeline?, heartMessage: String?,
    onStartHeart: (WatchPlayer) -> Unit, onHeartPage: () -> Unit,
) {
    var choosing by remember { mutableStateOf(false) }
    Column(Modifier.fillMaxSize().background(ink).verticalScroll(rememberScrollState())
        .padding(horizontal = 22.dp, vertical = 36.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text("KINOSAIL", color = signal, fontWeight = FontWeight.Bold, fontSize = 12.sp)
        if (remote.players.size > 1 || choosing) {
            Button(onClick = { choosing = !choosing }, modifier = Modifier.fillMaxWidth(),
                label = { Text(remote.selected?.name ?: "Choose player", maxLines = 1) })
            if (choosing) remote.players.forEach { player ->
                Button(onClick = { remote.select(player.id); choosing = false }, modifier = Modifier.fillMaxWidth(),
                    label = { Text("${player.name} · ${if (player.active) player.title else "Idle"}", maxLines = 1,
                        overflow = TextOverflow.Ellipsis) })
            }
        } else if (remote.players.isNotEmpty()) Text(remote.selected?.name ?: "Remote", color = muted, fontSize = 12.sp)

        val player = remote.selected
        if (timeline != null && player?.active != true) HeartLink(timeline, onHeartPage)
        if (player?.active == true) {
            Text(player.title, fontWeight = FontWeight.Bold, fontSize = 18.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
            if (player.subtitle.isNotEmpty()) Text(player.subtitle, color = muted, fontSize = 12.sp, maxLines = 1)
            if (player.duration > 0) {
                Canvas(Modifier.fillMaxWidth().height(5.dp)
                    .semantics { contentDescription = "Playback ${clock(player.position)} of ${clock(player.duration)}" }) {
                    drawLine(muted.copy(alpha = 0.4f), Offset(0f, center.y), Offset(size.width, center.y), 5.dp.toPx())
                    drawLine(signal, Offset(0f, center.y), Offset(size.width * (player.position / player.duration).toFloat(), center.y),
                        5.dp.toPx(), cap = StrokeCap.Round)
                }
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                    Text(clock(player.position), color = muted, fontSize = 11.sp)
                    Text(clock(player.duration), color = muted, fontSize = 11.sp)
                }
            }
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Control("−15", "Back 15 seconds", !remote.busy) { remote.command("backward") }
                Control(if (player.playing) "Ⅱ" else "▶", if (player.playing) "Pause" else "Play", !remote.busy,
                    primary = true) { remote.command(if (player.playing) "pause" else "play") }
                Control("+30", "Forward 30 seconds", !remote.busy) { remote.command("forward") }
            }
            if (!player.audio && player.duration > 0 && timeline?.tracking != true) {
                Button(onClick = { onStartHeart(player) }, modifier = Modifier.fillMaxWidth(),
                    label = { Text("♡ Start heart graph") })
            }
        } else {
            val disconnected = remote.players.isEmpty() && remote.message != null
            Text(if (disconnected) "Connect phone" else if (remote.selectedId != null && player == null)
                "Selected player unavailable" else "Nothing playing", fontWeight = FontWeight.Bold, fontSize = 17.sp)
            Text(if (disconnected) "Pair Android phone."
                else "Start a title on your Android phone or TV.", color = muted, fontSize = 12.sp)
        }
        if (timeline != null && player?.active == true) HeartLink(timeline, onHeartPage)
        (heartMessage ?: remote.message?.takeUnless { remote.players.isEmpty() })?.let {
            Text(it, color = muted, fontSize = 12.sp)
        }
    }
}

@Composable
private fun HeartLink(timeline: HeartTimeline, onClick: () -> Unit) {
    Button(onClick = onClick, modifier = Modifier.fillMaxWidth(),
        colors = ButtonDefaults.buttonColors(containerColor = raised, contentColor = signal),
        label = { Text(if (timeline.tracking) "♥ Heart graph" else "♥ Last heart graph", fontSize = 12.sp) })
}

@Composable
private fun Control(label: String, description: String, enabled: Boolean, primary: Boolean = false,
                    command: suspend () -> Unit) {
    val scope = rememberCoroutineScope()
    Button(onClick = { scope.launch { command() } }, enabled = enabled,
        colors = ButtonDefaults.buttonColors(containerColor = if (primary) signal else raised,
            contentColor = if (primary) ink else Color.White),
        modifier = Modifier.size(48.dp).semantics { contentDescription = description },
        label = { Text(label, fontWeight = FontWeight.Bold) })
}

@Composable
private fun HeartPage(timeline: HeartTimeline, message: String?, onStop: () -> Unit) {
    Column(Modifier.fillMaxSize().background(ink).verticalScroll(rememberScrollState())
        .padding(horizontal = 22.dp, vertical = 36.dp), verticalArrangement = Arrangement.spacedBy(5.dp)) {
        Text("HEART / MOVIE", color = signal, fontWeight = FontWeight.Bold, fontSize = 12.sp)
        Text(timeline.title, fontWeight = FontWeight.Bold, fontSize = 17.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
        if (timeline.points.isEmpty()) {
            Text("No readings yet", fontWeight = FontWeight.Bold)
            Text("Readings appear at their movie time as you watch.", color = muted, fontSize = 12.sp)
        } else {
            timeline.peak?.let {
                Text("♥ ${it.bpm.toInt()} bpm · ${clock(it.position)}", color = signal, fontSize = 11.sp,
                    modifier = Modifier.semantics {
                        contentDescription = "Peak ${it.bpm.toInt()} beats per minute at ${clock(it.position)}"
                    })
            }
            HeartPlot(timeline.points, timeline.duration)
            Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp), horizontalArrangement = Arrangement.SpaceBetween) {
                Text("0:00", color = muted, fontSize = 11.sp)
                Text(clock(timeline.duration), color = muted, fontSize = 11.sp)
            }
        }
        Text("Gaps mean no reading or movie position was available.", color = muted, fontSize = 11.sp)
        if (timeline.tracking) Button(onClick = onStop, modifier = Modifier.fillMaxWidth(),
            label = { Text("Stop tracking") })
        message?.let { Text(it, color = muted, fontSize = 12.sp) }
    }
}

@Composable
private fun HeartPlot(points: List<HeartPoint>, duration: Double) {
    val low = points.minOf(HeartPoint::bpm)
    val high = points.maxOf(HeartPoint::bpm)
    val peak = points.maxByOrNull(HeartPoint::bpm)
    Canvas(Modifier.fillMaxWidth().padding(horizontal = 20.dp).height(50.dp).semantics {
        contentDescription = "${points.size} heart readings, ${low.toInt()} to ${high.toInt()} beats per minute"
    }) {
        val floor = maxOf(25.0, low - 10)
        val range = maxOf(20.0, high - floor + 10)
        val path = Path()
        var previous: HeartPoint? = null
        for (point in points) {
            val x = size.width * (point.position / duration).toFloat()
            val y = size.height * (1 - (point.bpm - floor) / range).toFloat()
            val old = previous
            if (old != null && point.timeMs - old.timeMs <= 30_000 &&
                point.position >= old.position && point.position - old.position <= 30) path.lineTo(x, y)
            else path.moveTo(x, y)
            if (point == peak) drawCircle(signal, 4.dp.toPx(), Offset(x, y), style = Stroke(2.dp.toPx()))
            drawCircle(signal, 2.dp.toPx(), Offset(x, y))
            previous = point
        }
        drawPath(path, signal, style = Stroke(2.dp.toPx(), cap = StrokeCap.Round))
    }
}

private fun clock(seconds: Double): String {
    val value = seconds.coerceAtLeast(0.0).toLong()
    return if (value >= 3600) "%d:%02d:%02d".format(value / 3600, value / 60 % 60, value % 60)
        else "%d:%02d".format(value / 60, value % 60)
}
