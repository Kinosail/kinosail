package com.kinosail.player.core

import android.graphics.Paint
import android.graphics.Typeface
import androidx.compose.ui.Alignment
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.*
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import com.kinosail.player.design.KinoColor

@Composable
internal fun EpubReaderScreen(item: CatalogItem, viewer: Viewer, close: () -> Unit) {
    val model: EpubModel = viewModel()
    val state = model.state
    val context = LocalContext.current
    val preferences = remember { context.getSharedPreferences("kinosail_reader", 0) }
    val scope = remember(viewer.serverId, viewer.id) {
        java.security.MessageDigest.getInstance("SHA-256").digest("${viewer.serverId}\n${viewer.id}".toByteArray())
            .joinToString("") { "%02x".format(it) }
    }
    var fontSize by remember(scope) { mutableIntStateOf(runCatching { preferences.getInt("$scope-font", 18) }.getOrDefault(18).coerceIn(16, 32)) }
    var light by remember(scope) { mutableStateOf(runCatching { preferences.getBoolean("$scope-light", false) }.getOrDefault(false)) }
    var contents by remember { mutableStateOf(false) }
    var appearance by remember { mutableStateOf(false) }
    LaunchedEffect(item.id, viewer) { model.open(item, viewer) }
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    DisposableEffect(model, lifecycle) {
        val observer = LifecycleEventObserver { _, event -> if (event == Lifecycle.Event.ON_STOP) model.flush() }
        lifecycle.addObserver(observer)
        onDispose { lifecycle.removeObserver(observer); model.flush() }
    }
    val book = state.book
    val saved = state.session
    val textColor = if (light) Color(0xFF162011) else KinoColor.text
    val background = if (light) Color(0xFFF4F8EF) else KinoColor.background
    val displayDensity = LocalDensity.current
    val readerFontSize = (fontSize * displayDensity.fontScale).toInt().coerceIn(16, 96)
    val columnWidth = remember(readerFontSize, displayDensity.density) {
        val glyph = FloatArray(1)
        Paint(Paint.ANTI_ALIAS_FLAG or Paint.LINEAR_TEXT_FLAG or Paint.SUBPIXEL_TEXT_FLAG)
            .apply { typeface = Typeface.DEFAULT; textSize = readerFontSize * displayDensity.density }.getTextWidths("0", glyph)
        with(displayDensity) { (glyph[0] * 68).toDp() + 32.dp }
    }
    Column(Modifier.fillMaxSize().background(KinoColor.surface).safeDrawingPadding().background(background)) {
        FlowRow(Modifier.fillMaxWidth().background(KinoColor.surface).padding(horizontal = 12.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            TextButton(onClick = { model.flush(); close() }) { Text(interfaceText("Done")) }
            TextButton(onClick = { contents = true }, enabled = book != null) { Text(interfaceText("Contents")) }
            Box {
                TextButton(onClick = { appearance = true }) { Text(interfaceText("Appearance")) }
                DropdownMenu(expanded = appearance, onDismissRequest = { appearance = false }) {
                    DropdownMenuItem(text = { Text(interfaceText("Larger text")) }, enabled = fontSize < 32,
                        onClick = { fontSize += 2; preferences.edit().putInt("$scope-font", fontSize).apply(); model.retry() })
                    DropdownMenuItem(text = { Text(interfaceText("Smaller text")) }, enabled = fontSize > 16,
                        onClick = { fontSize -= 2; preferences.edit().putInt("$scope-font", fontSize).apply(); model.retry() })
                    DropdownMenuItem(text = { Text(interfaceText(if (light) "Dark page" else "Light page")) },
                        onClick = { light = !light; preferences.edit().putBoolean("$scope-light", light).apply(); appearance = false; model.retry() })
                }
            }
        }
        Text(item.title, style = MaterialTheme.typography.titleMedium, color = textColor,
            modifier = Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 12.dp))
        Box(Modifier.weight(1f).fillMaxWidth()) {
            if (book != null && saved != null && state.notice == null) {
                val page = state.position.page
                key(book.id, page, state.revision, fontSize, light) {
                    EpubDocument(book, saved, viewer, state.position,
                        readerFontSize, light,
                        Modifier.fillMaxSize(), result = { model.chapterResult(page, it) },
                        offset = { model.offset(page, it) }, navigate = model::turn)
                }
            }
            if (state.loading || state.chapterLoading && state.notice == null) Box(
                Modifier.fillMaxSize().background(background), contentAlignment = Alignment.TopCenter) {
                Column(Modifier.widthIn(max = columnWidth).fillMaxWidth().padding(16.dp)
                    .clearAndSetSemantics { contentDescription = "Opening your book" },
                    verticalArrangement = Arrangement.spacedBy((readerFontSize * 0.65f).dp)) {
                    repeat(7) { index -> Box(Modifier.fillMaxWidth(if (index == 6) 0.5f else 1f).height(readerFontSize.dp)
                        .clip(RoundedCornerShape(4.dp)).background(if (light) Color(0xFFE1E6DB) else KinoColor.raised)) }
                }
            }
            state.notice?.let { notice ->
                Column(Modifier.fillMaxWidth().padding(20.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    Text(interfaceText(notice), color = textColor, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
                    Button(onClick = model::retry) { Text(interfaceText("Try again")) }
                }
            }
        }
        state.conflict?.let {
            Column(Modifier.fillMaxWidth().background(KinoColor.surface).padding(12.dp)) {
                Text(interfaceText("The Server has a different reading position."), color = KinoColor.text,
                    modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
                TextButton(onClick = { model.resolve(true) }) { Text(interfaceText("Keep this device’s position")) }
                TextButton(onClick = { model.resolve(false) }) { Text(interfaceText("Use the Server’s position")) }
            }
        }
        state.syncNotice?.let {
            Text(interfaceText(it), color = textColor, modifier = Modifier.padding(horizontal = 20.dp).semantics { liveRegion = LiveRegionMode.Polite })
            TextButton(onClick = model::retrySync) { Text(interfaceText("Retry sync")) }
        }
        if (book != null) FlowRow(Modifier.fillMaxWidth().background(KinoColor.surface).padding(12.dp),
            horizontalArrangement = Arrangement.SpaceBetween, verticalArrangement = Arrangement.spacedBy(8.dp)) {
            TextButton(onClick = { model.turn(state.position.page - 1) }, enabled = state.position.page > 1) { Text(interfaceText("Previous")) }
            TextButton(onClick = { contents = true }) { Text("Chapter ${state.position.page} of ${book.chapters.size}") }
            TextButton(onClick = { model.turn(state.position.page + 1) }, enabled = state.position.page < book.chapters.size) { Text(interfaceText("Next")) }
        }
    }
    if (contents && book != null) AlertDialog(onDismissRequest = { contents = false },
        title = { Text(interfaceText("Contents")) }, confirmButton = { TextButton(onClick = { contents = false }) { Text(interfaceText("Done")) } },
        text = {
            LazyColumn(Modifier.heightIn(max = 400.dp)) {
                itemsIndexed(book.chapters) { index, chapter ->
                    TextButton(onClick = { model.turn(index + 1); contents = false }, modifier = Modifier.fillMaxWidth()) {
                        Text(chapter.title.ifEmpty { "Chapter ${index + 1}" }, modifier = Modifier.fillMaxWidth())
                    }
                }
            }
        })
}
