package com.kinosail.player.mobile

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.kinosail.player.core.*
import com.kinosail.player.design.KinoColor
import java.util.Locale
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext

private val RATES = listOf(0.5, 0.75, 1.0, 1.25, 1.5, 1.75, 2.0, 2.5, 3.0)
private val LANGUAGES = listOf("auto", "en", "es", "fr", "de", "it", "pt", "ja", "ko", "zh")
private const val LOAD_FAILED = "Could not load your playback preferences. Try again."
private const val SAVE_FAILED = "Could not save your playback preferences. Try again."

@Composable
internal fun PlaybackDefaultsScreen(catalog: CatalogModel) {
    val connection = catalog.connection
    var original by remember { mutableStateOf<PlaybackPreferences?>(null) }
    var edited by remember { mutableStateOf<PlaybackPreferences?>(null) }
    var notice by remember { mutableStateOf<String?>(null) }
    var attempt by remember { mutableIntStateOf(0) }
    var picker by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    val lock = remember { Mutex() }
    LaunchedEffect(connection?.first?.server?.url, connection?.second, attempt) {
        if (original != null) return@LaunchedEffect
        notice = null
        val (saved, viewer) = connection ?: run { notice = LOAD_FAILED; return@LaunchedEffect }
        try {
            val value = withContext(Dispatchers.IO) { AccountApi(saved.server).mediaPreferences(saved.token, viewer) }
            original = value.playback
            edited = value.playback
        } catch (cancelled: CancellationException) { throw cancelled } catch (_: Exception) { notice = LOAD_FAILED }
    }
    fun change(next: PlaybackPreferences) {
        val baseline = original ?: return
        val (saved, viewer) = connection ?: return
        edited = next
        notice = null
        scope.launch {
            lock.withLock {
                try {
                    val result = withContext(Dispatchers.IO) {
                        val api = AccountApi(saved.server)
                        val current = api.mediaPreferences(saved.token, viewer)
                        api.saveMediaPreferences(saved.token, viewer, current.copy(playback = merge(current.playback, baseline, next)))
                    }
                    original = result.playback
                    if (edited == next) edited = result.playback
                } catch (cancelled: CancellationException) { throw cancelled } catch (_: Exception) { notice = SAVE_FAILED }
            }
        }
    }
    val value = edited
    SettingsPage(interfaceText("Playback preferences")) {
        MoreSection(interfaceText("Playback"))
        if (value == null && notice == null) PreferencesLoading()
        if (value != null) {
            MoreGroup {
                MoreRow(interfaceText("Speed"), value = rateLabel(value.rate)) { picker = "speed" }
                MoreDivider(icon = false)
                MoreRow(interfaceText("Audio language"), value = languageLabel(value.audioLanguage)) { picker = "audio" }
                MoreDivider(icon = false)
                MoreRow(interfaceText("Subtitles"), value = languageLabel(value.subtitleLanguage)) { picker = "subtitles" }
            }
            MoreSection(interfaceText("Audio enhancements"))
            MoreGroup {
                SwitchRow(interfaceText("Boost Dialog"), interfaceText("Makes quiet speech easier to hear."),
                    value.dialogueBoost) { change(value.copy(dialogueBoost = it)) }
                MoreDivider(icon = false)
                SwitchRow(interfaceText("Normalize Loudness"), interfaceText("Evens out the volume and softens very loud scenes."),
                    value.nightMode) { change(value.copy(nightMode = it)) }
            }
            Text(interfaceText("Changes save automatically. Audio enhancements use a compatible Server stream."),
                style = MaterialTheme.typography.bodySmall, color = KinoColor.muted, modifier = Modifier.padding(horizontal = 18.dp))
        }
        notice?.let {
            Text(interfaceText(it), color = KinoColor.text, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
            RetryAction { if (value == null) attempt++ else change(value) }
        }
    }
    if (value != null) when (picker) {
        "speed" -> Picker(interfaceText("Speed"), (RATES + value.rate).distinct().sorted(), value.rate, { rateLabel(it) },
            { picker = null }) { change(value.copy(rate = it)) }
        "audio" -> Picker(interfaceText("Audio language"), (LANGUAGES + value.audioLanguage).distinct().sorted(),
            value.audioLanguage, { languageLabel(it) }, { picker = null }) { change(value.copy(audioLanguage = it, audioTrack = "")) }
        "subtitles" -> Picker(interfaceText("Subtitles"), (LANGUAGES + "off" + value.subtitleLanguage).distinct().sorted(),
            value.subtitleLanguage, { languageLabel(it) }, { picker = null }) { change(value.copy(subtitleLanguage = it, subtitleTrack = "")) }
    }
}

private fun merge(current: PlaybackPreferences, baseline: PlaybackPreferences, next: PlaybackPreferences) = current.copy(
    rate = if (next.rate != baseline.rate) next.rate else current.rate,
    audioLanguage = if (next.audioLanguage != baseline.audioLanguage) next.audioLanguage else current.audioLanguage,
    audioTrack = if (next.audioLanguage != baseline.audioLanguage) "" else current.audioTrack,
    subtitleLanguage = if (next.subtitleLanguage != baseline.subtitleLanguage) next.subtitleLanguage else current.subtitleLanguage,
    subtitleTrack = if (next.subtitleLanguage != baseline.subtitleLanguage) "" else current.subtitleTrack,
    dialogueBoost = if (next.dialogueBoost != baseline.dialogueBoost) next.dialogueBoost else current.dialogueBoost,
    nightMode = if (next.nightMode != baseline.nightMode) next.nightMode else current.nightMode)

private fun rateLabel(rate: Double) = if (rate % 1.0 == 0.0) "${rate.toInt()}×" else "$rate×"

@Composable
private fun languageLabel(code: String) = when (code) {
    "auto" -> interfaceText("Automatic")
    "off" -> interfaceText("Off")
    else -> Locale.forLanguageTag(code).getDisplayLanguage(Locale.getDefault()).ifEmpty { code }
}

@Composable
private fun <T> Picker(title: String, options: List<T>, selected: T, label: @Composable (T) -> String,
                       dismiss: () -> Unit, select: (T) -> Unit) {
    val choices = options.map { option -> PlaybackChoice(label(option), option == selected) { if (option != selected) select(option) } }
    PlaybackChoiceSheet(title, listOf(PlaybackChoiceGroup(null, choices)), tv = false, dismiss = dismiss)
}

@Composable
private fun SwitchRow(title: String, detail: String, checked: Boolean, change: (Boolean) -> Unit) {
    Row(Modifier.fillMaxWidth().heightIn(min = 72.dp).toggleable(checked, role = Role.Switch, onValueChange = change)
        .padding(horizontal = 18.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(16.dp)) {
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(title, color = KinoColor.text, style = MaterialTheme.typography.bodyLarge)
            Text(detail, color = KinoColor.muted, style = MaterialTheme.typography.bodySmall)
        }
        Switch(checked, onCheckedChange = null, colors = SwitchDefaults.colors(checkedTrackColor = KinoColor.signal))
    }
}

@Composable
private fun PreferencesLoading() {
    Column(Modifier.fillMaxWidth().clearAndSetSemantics { contentDescription = "Loading preferences" },
        verticalArrangement = Arrangement.spacedBy(12.dp)) {
        repeat(2) { group ->
            Column(Modifier.fillMaxWidth().clip(RoundedCornerShape(24.dp)).background(KinoColor.surface)) {
                repeat(3 - group) {
                    Box(Modifier.fillMaxWidth().height(if (group == 0) 60.dp else 72.dp).padding(horizontal = 18.dp, vertical = 20.dp)
                        .clip(RoundedCornerShape(8.dp)).background(KinoColor.raised))
                }
            }
        }
    }
}
