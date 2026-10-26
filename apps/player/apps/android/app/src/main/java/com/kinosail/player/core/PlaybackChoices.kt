package com.kinosail.player.core

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.kinosail.player.design.KinoColor
import kotlinx.coroutines.launch

internal data class PlaybackChoice(val label: String, val selected: Boolean, val select: () -> Unit)

internal data class PlaybackChoiceGroup(val title: String?, val choices: List<PlaybackChoice>)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun PlaybackChoiceSheet(title: String, groups: List<PlaybackChoiceGroup>, tv: Boolean, dismiss: () -> Unit) {
    if (tv) return PlaybackChoicePanel(title, groups, dismiss)
    val state = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val scope = rememberCoroutineScope()
    ModalBottomSheet(onDismissRequest = dismiss, sheetState = state, containerColor = KinoColor.surface) {
        Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).navigationBarsPadding()
            .padding(bottom = 16.dp)) {
            ChoiceHeading(title, MaterialTheme.typography.titleLarge, Modifier.padding(horizontal = 24.dp, vertical = 8.dp))
            groups.forEachIndexed { index, group ->
                if (index > 0) HorizontalDivider(Modifier.padding(vertical = 8.dp), color = KinoColor.text.copy(alpha = 0.12f))
                group.title?.let { ChoiceHeading(it, MaterialTheme.typography.labelLarge,
                    Modifier.padding(horizontal = 24.dp, vertical = 8.dp), KinoColor.text.copy(alpha = 0.7f)) }
                group.choices.forEach { choice ->
                    Row(Modifier.fillMaxWidth().heightIn(min = 56.dp)
                        .selectable(choice.selected, role = Role.RadioButton) {
                            choice.select()
                            scope.launch { state.hide() }.invokeOnCompletion { dismiss() }
                        }.padding(horizontal = 24.dp),
                        verticalAlignment = Alignment.CenterVertically) {
                        Text(choice.label, color = KinoColor.text, style = MaterialTheme.typography.bodyLarge,
                            modifier = Modifier.weight(1f))
                        if (choice.selected) Icon(Icons.Filled.Check, contentDescription = null, tint = KinoColor.signal)
                    }
                }
            }
        }
    }
}

@Composable
private fun PlaybackChoicePanel(title: String, groups: List<PlaybackChoiceGroup>, dismiss: () -> Unit) {
    val focus = remember { FocusRequester() }
    val initial = remember(groups) { groups.flatMap { it.choices }.let { all -> all.firstOrNull { it.selected } ?: all.firstOrNull() } }
    LaunchedEffect(Unit) { runCatching { focus.requestFocus() } }
    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.CenterEnd) {
        Column(Modifier.width(400.dp).fillMaxHeight().background(KinoColor.surface)
            .padding(horizontal = 32.dp, vertical = 40.dp)) {
            ChoiceHeading(title, MaterialTheme.typography.headlineSmall, Modifier.padding(bottom = 16.dp))
            Column(Modifier.weight(1f).verticalScroll(rememberScrollState()),
                verticalArrangement = Arrangement.spacedBy(4.dp)) { groups.forEach { group ->
                group.title?.let { ChoiceHeading(it, MaterialTheme.typography.titleSmall,
                    Modifier.padding(top = 16.dp, bottom = 4.dp), KinoColor.text.copy(alpha = 0.7f)) }
                group.choices.forEach { choice ->
                    androidx.tv.material3.DenseListItem(selected = choice.selected,
                        onClick = { choice.select(); dismiss() },
                        headlineContent = { androidx.tv.material3.Text(choice.label) },
                        trailingContent = { if (choice.selected) androidx.tv.material3.Icon(Icons.Filled.Check, contentDescription = null) },
                        modifier = Modifier.semantics { selected = choice.selected }
                            .then(if (choice === initial) Modifier.focusRequester(focus) else Modifier))
                }
            } }
        }
    }
}

@Composable
private fun ChoiceHeading(text: String, style: androidx.compose.ui.text.TextStyle, modifier: Modifier,
                          color: androidx.compose.ui.graphics.Color = KinoColor.text) {
    Text(text, color = color, style = style, fontWeight = FontWeight.SemiBold,
        modifier = modifier.semantics { heading() })
}
