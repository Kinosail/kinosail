package com.kinosail.player.core

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.BringIntoViewSpec
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Button
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import com.kinosail.player.design.KinoColor

@OptIn(ExperimentalFoundationApi::class)
@Composable
internal fun TvShowPage(state: ShowState, catalog: CatalogModel, season: Int?, choose: (Int) -> Unit, retry: () -> Unit,
                        play: (CatalogItem) -> Unit) {
    val detail = state.detail
    val featured = detail?.next
    val hero = if (featured != null && detail != null) featured.copy(title = detail.title, plot = detail.plot, year = detail.year) else null
    val restoring = remember(detail != null) { detail?.episodes?.any { it.id == TvFocusMemory.id } == true }
    Box(Modifier.fillMaxSize().background(KinoColor.background)) {
        hero?.let { TvBackdrop(it, catalog) }
        CompositionLocalProvider(LocalBringIntoViewSpec provides MinimalBringIntoView) {
        LazyColumn(Modifier.fillMaxSize().safeDrawingPadding(), contentPadding = PaddingValues(horizontal = 48.dp, vertical = 32.dp),
            verticalArrangement = Arrangement.spacedBy(28.dp)) {
            if (state.loading && detail == null) item { TvShowLoading() }
            state.notice?.let { notice -> item {
                Text(notice, color = KinoColor.text, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
                RetryAction(true, onClick = retry)
            } }
            if (featured != null && hero != null) {
                item {
                    val summary = listOf("S${featured.season}", "E${featured.episode}", featured.episodeTitle, featured.rating)
                        .filter(String::isNotEmpty).joinToString(" · ")
                    TvHeroInformation(hero, catalog, listOf(detail.year, featured.genres).filter(String::isNotEmpty).joinToString(" · "),
                        summary) {
                        Button(onClick = { play(featured) }, modifier = if (restoring) Modifier else Modifier.initialTvFocus(featured.id)) {
                            Text("${interfaceText(featured.playLabel)} · S${featured.season} E${featured.episode}")
                        }
                    }
                }
                item {
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp), contentPadding = PaddingValues(vertical = 8.dp)) {
                        items(detail.seasons, key = { it }) { number ->
                            val chosen = number == season
                            Button(onClick = { choose(number) }, modifier = Modifier.semantics { selected = chosen },
                                colors = if (chosen) ButtonDefaults.colors(containerColor = Color.White.copy(alpha = 0.28f),
                                    contentColor = KinoColor.text) else ButtonDefaults.colors()) {
                                Text(if (number == 0) "Specials" else "Season $number")
                            }
                        }
                    }
                }
                item {
                    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                        Text(interfaceText("Episodes"), style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold,
                            color = KinoColor.text, modifier = Modifier.semantics { heading() })
                        LazyRow(Modifier.bleed(24.dp, 0.dp), horizontalArrangement = Arrangement.spacedBy(18.dp),
                            contentPadding = PaddingValues(horizontal = 24.dp, vertical = 12.dp)) {
                            items(detail.episodes.filter { it.season == season }, key = CatalogItem::id) { episode ->
                                TvMediaCard(episode.copy(title = episode.episodeTitle), catalog, Modifier.width(homeCardWidth(true, true)),
                                    landscape = true) { play(episode) }
                            }
                        }
                    }
                }
            } else if (detail != null) item {
                Text(detail.title, color = KinoColor.text, style = MaterialTheme.typography.headlineLarge)
                Text(interfaceText("This show has no available episodes."), color = KinoColor.muted)
            }
        }
        }
    }
}

@OptIn(ExperimentalFoundationApi::class)
private object MinimalBringIntoView : BringIntoViewSpec {
    override fun calculateScrollDistance(offset: Float, size: Float, containerSize: Float): Float {
        val trailing = offset + size
        return when {
            offset >= 0 && trailing <= containerSize -> 0f
            offset < 0 && trailing > containerSize -> 0f
            kotlin.math.abs(offset) < kotlin.math.abs(trailing - containerSize) -> offset
            else -> trailing - containerSize
        }
    }
}
