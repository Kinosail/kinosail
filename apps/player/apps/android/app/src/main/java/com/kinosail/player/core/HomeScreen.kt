package com.kinosail.player.core

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.kinosail.player.design.KinoColor
import com.kinosail.player.design.SailBackdrop

@Composable
internal fun HomeScreen(viewer: Viewer, catalog: CatalogModel, tv: Boolean, nowPlaying: CatalogItem?, browse: () -> Unit,
                        open: (CatalogItem) -> Unit, play: (CatalogItem) -> Unit) {
    val model: HomeModel = viewModel()
    val state = model.state
    val featured = state.continueWatching.firstOrNull {
        it.kind in setOf("video", "music", "audiobook")
    } ?: state.recent.firstOrNull {
        it.kind in setOf("video", "music", "audiobook")
    }
    val playFocus = remember { FocusRequester() }
    LaunchedEffect(viewer.serverId, viewer.id) { model.open(viewer) }
    DisposableEffect(Unit) { onDispose { model.reset() } }
    LaunchedEffect(featured?.id, tv) {
        if (tv && featured != null) { withFrameNanos { }; playFocus.requestFocus() }
    }
    BoxWithConstraints(Modifier.fillMaxSize().background(KinoColor.background)) {
        val wideTouch = !tv && maxWidth >= 600.dp
        val heroWidth = if (tv) 140 else if (wideTouch) (maxWidth.value / 4).toInt().coerceIn(240, 320) else 120
        SailBackdrop()
        Column(Modifier.fillMaxSize().safeDrawingPadding().padding(if (tv) 56.dp else 20.dp),
            verticalArrangement = Arrangement.spacedBy(if (tv) 24.dp else 16.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically) {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text("Kinosail", style = MaterialTheme.typography.titleLarge, color = KinoColor.text)
                    Text("For you", style = if (tv) MaterialTheme.typography.displayLarge
                        else MaterialTheme.typography.headlineLarge,
                        color = KinoColor.text)
                    Text(viewer.name, color = KinoColor.muted)
                }
                if (tv) androidx.tv.material3.Button(onClick = browse) {
                    androidx.tv.material3.Text("Browse Library")
                } else TextButton(onClick = browse) { Text("Library") }
            }
            if (nowPlaying != null) {
                val label = "Now playing · ${nowPlaying.title}"
                if (tv) androidx.tv.material3.Button(onClick = { play(nowPlaying) }) {
                    androidx.tv.material3.Text(label)
                } else androidx.compose.material3.Button(onClick = { play(nowPlaying) },
                    modifier = Modifier.fillMaxWidth()) { Text(label) }
            }
            LazyColumn(Modifier.weight(1f), contentPadding = PaddingValues(bottom = 24.dp),
                verticalArrangement = Arrangement.spacedBy(if (tv) 24.dp else 16.dp)) {
                if (state.loading) item { CircularProgressIndicator(color = KinoColor.signal) }
                state.notice?.let { notice -> item {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text(notice, color = KinoColor.text)
                        if (tv) androidx.tv.material3.Button(onClick = model::retry) {
                            androidx.tv.material3.Text("Try again")
                        } else TextButton(onClick = model::retry) { Text("Try again") }
                    }
                } }
                if (featured != null) item {
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(if (tv) 32.dp else 16.dp),
                        verticalAlignment = Alignment.CenterVertically) {
                        HomeArtwork(featured, catalog, heroWidth)
                        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                            Text(featured.title, style = if (tv || wideTouch) MaterialTheme.typography.headlineLarge
                                else MaterialTheme.typography.titleLarge,
                                color = KinoColor.text)
                            val label = if (featured.progress.seconds > 0 && !featured.progress.watched) "Resume" else "Play"
                            if (tv) {
                                androidx.tv.material3.Button(onClick = { play(featured) },
                                    modifier = Modifier.focusRequester(playFocus)) {
                                    androidx.tv.material3.Text(label)
                                }
                                androidx.tv.material3.Button(onClick = { open(featured) }) {
                                    androidx.tv.material3.Text("Details")
                                }
                            } else {
                                androidx.compose.material3.Button(onClick = { play(featured) }) { Text(label) }
                                TextButton(onClick = { open(featured) }) { Text("Details") }
                            }
                        }
                    }
                }
                if (state.continueWatching.isNotEmpty()) item {
                    HomeShelf("Continue Watching", state.continueWatching.take(12), catalog, tv, wideTouch, open)
                }
                if (state.recent.isNotEmpty()) item {
                    HomeShelf("Recently Added", state.recent.take(24), catalog, tv, wideTouch, open)
                }
                if (!state.loading && state.notice == null && state.continueWatching.isEmpty() &&
                    state.recent.isEmpty()) item {
                    Text("Media added to your Server will appear here.", color = KinoColor.muted)
                }
            }
        }
    }
}

@Composable
private fun HomeShelf(title: String, items: List<CatalogItem>, catalog: CatalogModel, tv: Boolean, wideTouch: Boolean,
                      open: (CatalogItem) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(title, style = MaterialTheme.typography.titleLarge, color = KinoColor.text)
        LazyRow(horizontalArrangement = Arrangement.spacedBy(if (tv) 20.dp else if (wideTouch) 16.dp else 12.dp)) {
            items(items, key = CatalogItem::id) { item ->
                if (tv) androidx.tv.material3.Card(onClick = { open(item) },
                    modifier = Modifier.width(200.dp).semantics { contentDescription = item.title }) {
                    Column {
                        HomeArtwork(item, catalog, 200)
                        androidx.tv.material3.Text(item.title, maxLines = 2, modifier = Modifier.padding(8.dp))
                    }
                } else androidx.compose.material3.Card(onClick = { open(item) },
                    modifier = Modifier.width(if (wideTouch) 190.dp else 144.dp)
                        .semantics { contentDescription = item.title },
                    colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface)) {
                    Column {
                        HomeArtwork(item, catalog, if (wideTouch) 190 else 144)
                        Text(item.title, maxLines = 2, modifier = Modifier.padding(8.dp))
                    }
                }
            }
        }
    }
}

@Composable
private fun HomeArtwork(item: CatalogItem, catalog: CatalogModel, width: Int) {
    val bitmap by produceState<android.graphics.Bitmap?>(null, item.artwork, catalog, width) {
        value = catalog.artwork(item.artwork, 400)
    }
    Box(Modifier.width(width.dp).aspectRatio(2f / 3f)
        .background(KinoColor.raised),
        contentAlignment = Alignment.Center) {
        Text(item.title.firstOrNull()?.uppercase() ?: "K",
            color = KinoColor.signal,
            style = MaterialTheme.typography.displayMedium, modifier = Modifier.clearAndSetSemantics { })
        bitmap?.let { Image(it.asImageBitmap(), contentDescription = null,
            contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize()) }
    }
}
