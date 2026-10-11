package com.kinosail.player.core

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.grid.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.*
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.kinosail.player.design.KinoColor
import com.kinosail.player.design.SailBackdrop

@Composable
internal fun CollectionsScreen(viewer: Viewer, catalog: CatalogModel, tv: Boolean, close: () -> Unit) {
    val model: CollectionsModel = viewModel()
    val state = model.state
    LaunchedEffect(viewer.serverId, viewer.id) { model.open(viewer) }
    BackHandler(state.name != null) { model.back() }
    Box(Modifier.fillMaxSize().background(KinoColor.background)) {
        SailBackdrop()
        Column(Modifier.fillMaxSize().then(if (tv) Modifier.safeDrawingPadding() else Modifier)
            .padding(if (tv) 48.dp else 20.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                CollectionAction("Back", tv) { if (state.name == null) close() else model.back() }
                CollectionAction("Refresh", tv, !state.loading) { model.refresh() }
            }
            Text(state.name ?: interfaceText("Collections"), style = MaterialTheme.typography.headlineLarge,
                color = KinoColor.text, modifier = Modifier.semantics { heading() })
            state.notice?.let {
                Text(interfaceText(it), color = KinoColor.text, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
                CollectionAction("Try again", tv, !state.loading) { model.refresh() }
            }
            if (state.loading && (if (state.name == null) state.names.isEmpty() else state.items.isEmpty())) {
                if (state.name != null) LibraryLoading(tv = tv)
                else Column(Modifier.fillMaxWidth().clearAndSetSemantics { contentDescription = "Loading collections" },
                    verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    repeat(4) { Box(Modifier.fillMaxWidth().height(64.dp).clip(RoundedCornerShape(12.dp)).background(KinoColor.raised)) }
                }
            } else if (state.name == null) {
                if (state.names.isEmpty() && state.notice == null) {
                    Text(interfaceText("No collections yet"), style = MaterialTheme.typography.titleLarge)
                    Text(interfaceText("Collections from your Server will appear here."), color = KinoColor.muted)
                }
                LazyColumn(Modifier.weight(1f), contentPadding = PaddingValues(if (tv) 12.dp else 0.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    items(state.names, key = { it }) { name ->
                        if (tv) androidx.tv.material3.Card(onClick = { model.select(name) }, modifier = Modifier.fillMaxWidth()) {
                            androidx.tv.material3.Text(name, style = MaterialTheme.typography.titleLarge,
                                modifier = Modifier.fillMaxWidth().heightIn(min = 64.dp).padding(16.dp))
                        } else TextButton(onClick = { model.select(name) }, modifier = Modifier.fillMaxWidth().heightIn(min = 64.dp)) {
                            Text(name, modifier = Modifier.fillMaxWidth())
                        }
                        if (!tv) HorizontalDivider()
                    }
                }
            } else {
                if (state.items.isEmpty() && state.notice == null) Text(interfaceText("This collection is empty."), color = KinoColor.muted)
                val largeText = LocalDensity.current.fontScale >= 1.5f
                LazyVerticalGrid(if (largeText) GridCells.Fixed(1) else GridCells.Adaptive(if (tv) 160.dp else 144.dp),
                    modifier = Modifier.weight(1f), contentPadding = PaddingValues(if (tv) 12.dp else 0.dp),
                    horizontalArrangement = Arrangement.spacedBy(if (tv) 20.dp else 18.dp),
                    verticalArrangement = Arrangement.spacedBy(28.dp)) {
                    items(state.items, key = CatalogItem::id) { item ->
                        val content: @Composable () -> Unit = {
                            Column {
                                MediaArtwork(item, catalog, Modifier.fillMaxWidth())
                                Text(item.title, style = MaterialTheme.typography.titleMedium,
                                    color = if (tv) androidx.tv.material3.LocalContentColor.current else KinoColor.text,
                                    maxLines = if (largeText) Int.MAX_VALUE else 2, overflow = TextOverflow.Ellipsis,
                                    modifier = Modifier.padding(10.dp))
                            }
                        }
                        if (tv) androidx.tv.material3.Card(onClick = { catalog.select(item) }) { content() }
                        else Card(onClick = { catalog.select(item) }, colors = CardDefaults.cardColors(containerColor = KinoColor.surface)) { content() }
                    }
                }
            }
        }
    }
}

@Composable
private fun CollectionAction(label: String, tv: Boolean, enabled: Boolean = true,
                             modifier: Modifier = Modifier, action: () -> Unit) {
    if (tv) androidx.tv.material3.Button(onClick = action, enabled = enabled, modifier = modifier) {
        androidx.tv.material3.Text(interfaceText(label))
    } else TextButton(onClick = action, enabled = enabled, modifier = modifier) {
        Text(interfaceText(label), modifier = Modifier.padding(vertical = 8.dp))
    }
}
