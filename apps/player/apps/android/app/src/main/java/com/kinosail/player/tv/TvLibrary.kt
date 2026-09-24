package com.kinosail.player.tv

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.Image
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
import androidx.compose.foundation.lazy.grid.itemsIndexed
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.runtime.produceState
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.tv.material3.Button
import androidx.tv.material3.Card
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import com.kinosail.player.core.CatalogItem
import com.kinosail.player.core.CatalogModel
import com.kinosail.player.core.ConnectionModel
import com.kinosail.player.core.PlaybackScreen
import com.kinosail.player.core.Viewer
import com.kinosail.player.design.KinoColor
import com.kinosail.player.design.SailBackdrop

@Composable
internal fun TvLibrary(connection: ConnectionModel, viewer: Viewer) {
    val catalog: CatalogModel = viewModel()
    val state = catalog.state
    var playing by remember { mutableStateOf(false) }
    val keyboard = LocalSoftwareKeyboardController.current
    var nextFocus by remember { mutableIntStateOf(-1) }
    val submitSearch = {
        nextFocus = -1
        catalog.search()
        keyboard?.hide()
        Unit
    }
    val cardFocus = remember { FocusRequester() }
    val pageFocus = remember { FocusRequester() }
    val searchFocus = remember { FocusRequester() }
    val detailFocus = remember { FocusRequester() }
    val gridState = rememberLazyGridState()
    LaunchedEffect(viewer.serverId, viewer.id) { catalog.open(viewer) }
    DisposableEffect(Unit) { onDispose { catalog.reset() } }
    BackHandler(state.selected != null && !playing) { catalog.closeDetail() }
    LaunchedEffect(state.items.isNotEmpty(), state.selected, playing) {
        if (playing) return@LaunchedEffect
        if (state.selected != null) detailFocus.requestFocus()
        else if (state.items.isNotEmpty()) cardFocus.requestFocus()
        else searchFocus.requestFocus()
    }
    LaunchedEffect(state.items.size, nextFocus) {
        if (nextFocus >= 0 && state.items.size > nextFocus) {
            gridState.scrollToItem(nextFocus)
            withFrameNanos { }
            pageFocus.requestFocus()
        }
    }
    if (playing && state.selected != null) {
        PlaybackScreen(state.selected, viewer, tv = true) { playing = false }
        return
    }
    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        SailBackdrop()
        Column(Modifier.fillMaxSize().safeDrawingPadding().padding(56.dp),
            verticalArrangement = Arrangement.spacedBy(20.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically) {
                Text("Kinosail", style = MaterialTheme.typography.headlineLarge,
                    color = MaterialTheme.colorScheme.onBackground)
                Button(onClick = connection::signOut, enabled = !connection.busy) { Text("Disconnect") }
            }
            if (state.selected != null) {
                TvDetail(state.selected, catalog, Modifier.focusRequester(detailFocus)) { playing = true }
            } else {
                Text("Library", style = MaterialTheme.typography.displayMedium,
                    color = MaterialTheme.colorScheme.onBackground)
                Text("${viewer.name} · ${viewer.server}", style = MaterialTheme.typography.titleLarge,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
                Row(horizontalArrangement = Arrangement.spacedBy(16.dp), verticalAlignment = Alignment.CenterVertically) {
                    OutlinedTextField(value = catalog.searchInput,
                        onValueChange = { if (it.length <= 512) catalog.searchInput = it },
                        label = { androidx.compose.material3.Text("Search your library") },
                        singleLine = true, keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                        keyboardActions = KeyboardActions(onSearch = { submitSearch() }),
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedTextColor = KinoColor.text, unfocusedTextColor = KinoColor.text,
                            focusedLabelColor = KinoColor.signal, unfocusedLabelColor = KinoColor.muted,
                            focusedBorderColor = KinoColor.signal, unfocusedBorderColor = KinoColor.muted),
                        modifier = Modifier.width(650.dp))
                    Button(onClick = submitSearch,
                        modifier = Modifier.focusRequester(searchFocus)) { Text("Search") }
                }
                state.notice?.let { Text(it, color = MaterialTheme.colorScheme.error) }
                if (state.notice != null) Button(onClick = catalog::retry) { Text("Retry") }
                if (state.items.isEmpty() && !state.loading && state.notice == null) {
                    Text("Nothing in your library yet.", style = MaterialTheme.typography.titleLarge,
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                LazyVerticalGrid(columns = GridCells.Fixed(5), modifier = Modifier.weight(1f), state = gridState,
                    horizontalArrangement = Arrangement.spacedBy(20.dp),
                    verticalArrangement = Arrangement.spacedBy(20.dp),
                    contentPadding = PaddingValues(bottom = 28.dp)) {
                    itemsIndexed(state.items, key = { _, item -> item.id }) { index, item ->
                        Card(onClick = { catalog.select(item) }, modifier = when (index) {
                                0 -> Modifier.focusRequester(cardFocus)
                                nextFocus -> Modifier.focusRequester(pageFocus)
                                else -> Modifier
                            }.fillMaxWidth()) {
                            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                                TvPoster(item, catalog, Modifier.fillMaxWidth())
                                Text(item.title, maxLines = 2, overflow = TextOverflow.Ellipsis,
                                    modifier = Modifier.padding(8.dp),
                                    color = MaterialTheme.colorScheme.onBackground)
                            }
                        }
                    }
                    if (state.items.size < state.total) item(span = { GridItemSpan(maxLineSpan) }) {
                        Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.CenterStart) {
                            Button(onClick = { nextFocus = state.items.size; catalog.loadMore() },
                                enabled = !state.loading) {
                                Text(if (state.loading) "Loading…" else "Load more")
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun TvDetail(item: CatalogItem, catalog: CatalogModel, firstModifier: Modifier, play: () -> Unit) {
    Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(24.dp)) {
        if (item.kind in setOf("video", "music", "audiobook")) {
            Button(onClick = play, modifier = firstModifier) {
                Text(if (item.progress.seconds > 0 && !item.progress.watched) "Resume" else "Play")
            }
            Button(onClick = catalog::closeDetail) { Text("Back to Library") }
        } else Button(onClick = catalog::closeDetail, modifier = firstModifier) { Text("Back to Library") }
        Row(horizontalArrangement = Arrangement.spacedBy(32.dp)) {
            TvPoster(item, catalog, Modifier.width(260.dp), ratio = 2f / 3f, dimension = 800)
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Text(item.title, style = MaterialTheme.typography.displayMedium,
                    color = MaterialTheme.colorScheme.onBackground)
                Text(listOf(item.kind.replaceFirstChar(Char::uppercaseChar), item.year).filter(String::isNotEmpty)
                    .joinToString(" · "), style = MaterialTheme.typography.titleLarge,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
                if (item.plot.isNotEmpty()) Text(item.plot, style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onBackground)
            }
        }
    }
}

@Composable
private fun TvPoster(item: CatalogItem, catalog: CatalogModel, modifier: Modifier = Modifier,
                     ratio: Float = 16f / 10f, dimension: Int = 400) {
    val image = produceState<android.graphics.Bitmap?>(null, item.artwork, catalog, dimension) {
        value = catalog.artwork(item.artwork, dimension)
    }.value
    Box(modifier.aspectRatio(ratio).background(KinoColor.raised), contentAlignment = Alignment.Center) {
        Text(item.title.firstOrNull()?.uppercase() ?: "K", style = MaterialTheme.typography.displayLarge,
            modifier = Modifier.clearAndSetSemantics { },
            color = KinoColor.signal)
        image?.let { Image(it.asImageBitmap(), contentDescription = null,
            contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize()) }
    }
}
