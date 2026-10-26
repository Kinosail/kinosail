package com.kinosail.player.core

import android.util.Log
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.kinosail.player.design.KinoColor
import kotlin.math.ceil
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.doubleOrNull

data class WatchSummary(val seconds: Double, val duration: Double) {
    val fraction get() = if (duration > 0) (seconds / duration).toFloat() else null
    val remainingMinutes get() = ceil((duration - seconds) / 60).toInt()

    companion object {
        fun parse(raw: JsonElement): WatchSummary {
            val value = raw as? JsonObject
            require(value != null && value.keys == setOf("seconds", "duration")) { "Invalid watch summary." }
            fun number(key: String): Double {
                val primitive = value.getValue(key) as? JsonPrimitive
                val number = primitive?.takeUnless { it.isString }?.doubleOrNull
                require(number != null && number.isFinite() && number in 0.0..315_360_000.0) { "Invalid watch summary." }
                return number
            }
            val seconds = number("seconds")
            val duration = number("duration")
            require(duration == 0.0 || seconds <= duration) { "Invalid watch summary." }
            return WatchSummary(seconds, duration)
        }
    }
}

private object WatchDurations {
    private val values = object : LinkedHashMap<String, Double>(64, 0.75f, true) {
        override fun removeEldestEntry(eldest: MutableMap.MutableEntry<String, Double>?) = size > 256
    }
    @Synchronized fun get(key: String) = values[key]
    @Synchronized fun put(key: String, duration: Double) { values[key] = duration }
}

@Composable
internal fun rememberWatchSummary(item: CatalogItem, catalog: CatalogModel): WatchSummary? {
    val connection = catalog.connection
    val eligible = item.kind == "video" && item.progress.seconds > 0 && !item.progress.watched
    val key = connection?.takeIf { eligible }?.let { (saved, viewer) -> "${saved.server}|$viewer|${item.id}" }
    val cached = key?.let(WatchDurations::get)?.takeIf { it >= item.progress.seconds }?.let { WatchSummary(item.progress.seconds, it) }
    val summary by produceState(cached, key, item.progress.seconds) {
        val (saved, viewer) = connection ?: return@produceState
        if (key == null) return@produceState
        value = try {
            withContext(Dispatchers.IO) { ProgressApi(saved.server).summary(item.id, saved.token, viewer) }
                .also { WatchDurations.put(key, it.duration) }
        } catch (_: IllegalArgumentException) {
            Log.w("KinosailNetwork", "Watch summary rejected operation=items-watch-progress kind=invalid_response outcome=discarded")
            null
        } catch (cancelled: kotlinx.coroutines.CancellationException) {
            throw cancelled
        } catch (_: Exception) { value }
    }
    return summary
}

@Composable
internal fun WatchSummary.remainingLabel() = "$remainingMinutes ${interfaceText("min left")}"

@Composable
internal fun WatchBar(summary: WatchSummary, modifier: Modifier = Modifier) {
    val fraction = summary.fraction ?: return
    val remaining = summary.remainingLabel()
    LinearProgressIndicator(progress = { fraction }, color = KinoColor.signal, trackColor = Color.Black.copy(alpha = 0.78f),
        strokeCap = StrokeCap.Round, gapSize = 0.dp, drawStopIndicator = {},
        modifier = modifier.fillMaxWidth().height(6.dp).semantics { contentDescription = remaining })
}

@Composable
internal fun WatchPosition(item: CatalogItem, catalog: CatalogModel) {
    val current = rememberWatchSummary(item, catalog)
    val fraction = current?.fraction
    if (current == null || fraction == null) {
        if (item.positionLabel.isNotEmpty()) Text(interfaceText(item.positionLabel), color = KinoColor.muted,
            style = MaterialTheme.typography.bodyMedium)
        return
    }
    val remaining = current.remainingLabel()
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
        LinearProgressIndicator(progress = { fraction }, color = KinoColor.signal, trackColor = KinoColor.raised,
            gapSize = 0.dp, drawStopIndicator = {}, modifier = Modifier.weight(1f).semantics { contentDescription = remaining })
        Text(remaining, color = KinoColor.muted, style = MaterialTheme.typography.bodyMedium)
    }
}
