package com.kinosail.player.mobile

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.Image
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.kinosail.player.core.CatalogItem
import com.kinosail.player.core.CatalogModel
import com.kinosail.player.core.ConnectionModel
import com.kinosail.player.core.PlaybackScreen
import com.kinosail.player.core.Viewer
import com.kinosail.player.design.SailBackdrop

@Composable
internal fun MobileLibrary(connection: ConnectionModel, viewer: Viewer) {
    val catalog: CatalogModel = viewModel()
    val state = catalog.state
    var playing by remember { mutableStateOf(false) }
    val keyboard = LocalSoftwareKeyboardController.current
    val submitSearch = {
        catalog.search()
        keyboard?.hide()
        Unit
    }
    LaunchedEffect(viewer.serverId, viewer.id) { catalog.open(viewer) }
    DisposableEffect(Unit) { onDispose { catalog.reset() } }
    BackHandler(state.selected != null && !playing) { catalog.closeDetail() }
    if (playing && state.selected != null) {
        PlaybackScreen(state.selected, viewer, tv = false) { playing = false }
        return
    }
    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        if (isSystemInDarkTheme()) SailBackdrop()
        Column(Modifier.fillMaxSize().safeDrawingPadding().padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically) {
                Text("Kinosail", style = MaterialTheme.typography.headlineMedium,
                    color = MaterialTheme.colorScheme.onBackground)
                TextButton(onClick = connection::signOut, enabled = !connection.busy) { Text("Disconnect") }
            }
            if (state.selected != null) {
                MobileDetail(state.selected, catalog) { playing = true }
            } else {
                Text("Library", style = MaterialTheme.typography.headlineLarge,
                    color = MaterialTheme.colorScheme.onBackground)
                Text("${viewer.name} · ${viewer.server}", color = MaterialTheme.colorScheme.onSurfaceVariant)
                OutlinedTextField(value = catalog.searchInput,
                    onValueChange = { if (it.length <= 512) catalog.searchInput = it },
                    label = { Text("Search your library") }, singleLine = true,
                    keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                    keyboardActions = KeyboardActions(onSearch = { submitSearch() }),
                    modifier = Modifier.fillMaxWidth())
                Button(onClick = submitSearch, modifier = Modifier.fillMaxWidth()) { Text("Search") }
                state.notice?.let { Text(it, color = MaterialTheme.colorScheme.error) }
                if (state.notice != null) TextButton(onClick = catalog::retry) { Text("Retry") }
                if (state.loading && state.items.isEmpty()) CircularProgressIndicator()
                else if (state.items.isEmpty() && state.notice == null) {
                    Text("Nothing in your library yet.", color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                LazyVerticalGrid(columns = GridCells.Adaptive(144.dp), modifier = Modifier.weight(1f),
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                    verticalArrangement = Arrangement.spacedBy(16.dp),
                    contentPadding = PaddingValues(bottom = 24.dp)) {
                    items(state.items, key = CatalogItem::id) { item ->
                        Card(onClick = { catalog.select(item) }, modifier = Modifier.fillMaxWidth()) {
                            Column {
                                CatalogPoster(item, catalog, Modifier.fillMaxWidth())
                                Text(item.title, modifier = Modifier.padding(10.dp),
                                    style = MaterialTheme.typography.titleSmall,
                                    maxLines = 2, overflow = TextOverflow.Ellipsis)
                            }
                        }
                    }
                    if (state.items.size < state.total) item(span = { GridItemSpan(maxLineSpan) }) {
                        Button(onClick = catalog::loadMore, enabled = !state.loading,
                            modifier = Modifier.fillMaxWidth()) {
                            Text(if (state.loading) "Loading…" else "Load more")
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun MobileDetail(item: CatalogItem, catalog: CatalogModel, play: () -> Unit) {
    Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(16.dp)) {
        TextButton(onClick = catalog::closeDetail) { Text("Back to Library") }
        if (item.kind in setOf("video", "music", "audiobook")) {
            Button(onClick = play, modifier = Modifier.fillMaxWidth()) {
                Text(if (item.progress.seconds > 0 && !item.progress.watched) "Resume" else "Play")
            }
        }
        CatalogPoster(item, catalog, Modifier.width(176.dp), dimension = 800)
        Text(item.title, style = MaterialTheme.typography.headlineLarge,
            color = MaterialTheme.colorScheme.onBackground)
        Text(listOf(item.kind.replaceFirstChar(Char::uppercaseChar), item.year).filter(String::isNotEmpty)
            .joinToString(" · "), color = MaterialTheme.colorScheme.onSurfaceVariant)
        if (item.plot.isNotEmpty()) Text(item.plot, style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onBackground)
    }
}

@Composable
internal fun CatalogPoster(item: CatalogItem, catalog: CatalogModel, modifier: Modifier = Modifier,
                           dimension: Int = 400) {
    val image = produceState<android.graphics.Bitmap?>(null, item.artwork, catalog, dimension) {
        value = catalog.artwork(item.artwork, dimension)
    }.value
    Box(modifier.aspectRatio(2f / 3f).background(MaterialTheme.colorScheme.surfaceVariant),
        contentAlignment = Alignment.Center) {
        Text(item.title.firstOrNull()?.uppercase() ?: "K",
            modifier = Modifier.clearAndSetSemantics { },
            style = MaterialTheme.typography.displayMedium,
            color = MaterialTheme.colorScheme.primary)
        image?.let { Image(it.asImageBitmap(), contentDescription = null,
            contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize()) }
    }
}
