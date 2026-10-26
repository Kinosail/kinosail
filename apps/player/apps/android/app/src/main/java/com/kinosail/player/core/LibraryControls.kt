package com.kinosail.player.core

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectVerticalDragGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.min
import com.kinosail.player.R
import com.kinosail.player.design.KinoColor
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.first

internal suspend fun androidx.compose.foundation.lazy.grid.LazyGridState.scrollToLoaded(index: Int) {
    androidx.compose.runtime.snapshotFlow { layoutInfo.totalItemsCount }.first { it > index }
    scrollToItem(index)
}

internal fun sortTitle(sort: String) = when (sort) { "added" -> "Recently added"; "year" -> "Year"; else -> "Title" }

internal fun jumpLetters(state: CatalogState, searching: Boolean): List<CatalogLetter> =
    if (state.total > 60 && state.letters.size > 1 && state.sort == "title" && !searching && state.resultQuery.isNullOrEmpty()) state.letters
    else emptyList()

@Composable
internal fun LibrarySortMenu(sort: String, tv: Boolean, change: (String) -> Unit) {
    var open by remember { mutableStateOf(false) }
    val label = "${interfaceText("Sort")}: ${interfaceText(sortTitle(sort))}"
    Box {
        if (tv) androidx.tv.material3.Button(onClick = { open = true },
            contentPadding = androidx.tv.material3.ButtonDefaults.ButtonWithIconContentPadding) {
            androidx.tv.material3.Icon(painterResource(R.drawable.sort), contentDescription = null, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(8.dp))
            androidx.tv.material3.Text(label)
        } else FilledTonalButton(onClick = { open = true }, contentPadding = ButtonDefaults.ButtonWithIconContentPadding,
            colors = ButtonDefaults.filledTonalButtonColors(containerColor = KinoColor.raised, contentColor = KinoColor.text)) {
            Icon(painterResource(R.drawable.sort), contentDescription = null, modifier = Modifier.size(ButtonDefaults.IconSize))
            Spacer(Modifier.width(ButtonDefaults.IconSpacing))
            Text(label)
        }
        DropdownMenu(open, onDismissRequest = { open = false }, containerColor = KinoColor.surface) {
            LIBRARY_SORTS.forEach { option ->
                if (tv) androidx.tv.material3.ListItem(selected = option == sort, onClick = { open = false; change(option) },
                    headlineContent = { androidx.tv.material3.Text(interfaceText(sortTitle(option))) },
                    trailingContent = { if (option == sort) androidx.tv.material3.Icon(Icons.Filled.Check, contentDescription = null) },
                    modifier = Modifier.width(220.dp))
                else DropdownMenuItem(text = { Text(interfaceText(sortTitle(option)), color = KinoColor.text) },
                    onClick = { open = false; change(option) },
                    trailingIcon = { if (option == sort) Icon(Icons.Filled.Check, contentDescription = null, tint = KinoColor.signal) })
            }
        }
    }
}

@Composable
internal fun LetterIndex(letters: List<CatalogLetter>, tv: Boolean, modifier: Modifier = Modifier, select: (CatalogLetter) -> Unit) {
    val current by rememberUpdatedState(select)
    var focused by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(focused) {
        val letter = letters.firstOrNull { it.label == focused } ?: return@LaunchedEffect
        delay(150)
        current(letter)
    }
    BoxWithConstraints(modifier.width(if (tv) 44.dp else 28.dp)) {
        val row = min(24.dp, (maxHeight - 14.dp) / letters.size.coerceAtLeast(1))
        var dragged by remember { mutableStateOf<String?>(null) }
        Column(Modifier.background(if (tv) KinoColor.surface else Color.Transparent, RoundedCornerShape(if (tv) 7.dp else 14.dp))
            .padding(vertical = 7.dp)
            .then(if (tv) Modifier else Modifier.pointerInput(letters) {
                detectVerticalDragGestures(onDragEnd = { dragged = null }) { change, _ ->
                    val index = (change.position.y / row.toPx()).toInt().coerceIn(0, letters.lastIndex)
                    if (dragged != letters[index].label) { dragged = letters[index].label; current(letters[index]) }
                }
            }), horizontalAlignment = Alignment.CenterHorizontally) {
            letters.forEach { letter ->
                val description = "${letter.label}, ${letter.count} ${interfaceText(if (letter.count == 1) "title" else "titles")}"
                if (tv) androidx.tv.material3.Surface(onClick = { current(letter) }, shape = androidx.tv.material3.ClickableSurfaceDefaults.shape(CircleShape),
                    scale = androidx.tv.material3.ClickableSurfaceDefaults.scale(focusedScale = 1f),
                    colors = androidx.tv.material3.ClickableSurfaceDefaults.colors(containerColor = Color.Transparent, contentColor = KinoColor.text,
                        focusedContainerColor = Color.White, focusedContentColor = Color.Black),
                    modifier = Modifier.size(30.dp, row).onFocusChanged { if (it.isFocused) focused = letter.label else if (focused == letter.label) focused = null }
                        .semantics { contentDescription = description }) {
                    Box(Modifier.size(30.dp, row), contentAlignment = Alignment.Center) {
                        androidx.tv.material3.Text(letter.label, style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.SemiBold)
                    }
                } else Box(Modifier.size(28.dp, row).clickable(role = Role.Button) { current(letter) }
                    .background(if (dragged == letter.label) KinoColor.raised else Color.Transparent, CircleShape)
                    .semantics { contentDescription = description }, contentAlignment = Alignment.Center) {
                    Text(letter.label, style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.SemiBold, color = KinoColor.signal)
                }
            }
        }
    }
}
