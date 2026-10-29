package com.kinosail.player.core

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.hideFromAccessibility
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Button
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.Card
import androidx.tv.material3.CardDefaults
import androidx.tv.material3.Icon
import androidx.tv.material3.IconButton
import androidx.tv.material3.Text
import com.kinosail.player.R
import com.kinosail.player.design.KinoColor

@Composable
internal fun TvHomeHeader(listening: Boolean, browse: (String) -> Unit, settings: () -> Unit, search: FocusRequester,
                          searchFocused: (Boolean) -> Unit) {
    Row(Modifier.fillMaxWidth().padding(horizontal = 48.dp, vertical = 16.dp), verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(16.dp)) {
        Button(onClick = { TvFocusMemory.id = "search"; browse("search") }, contentPadding = ButtonDefaults.ButtonWithIconContentPadding,
            modifier = Modifier.focusRequester(search).restoresTvFocus("search").onFocusChanged { searchFocused(it.isFocused) }) {
            Icon(Icons.Filled.Search, contentDescription = null, modifier = Modifier.size(ButtonDefaults.IconSize))
            Spacer(Modifier.width(ButtonDefaults.IconSpacing))
            Text(interfaceText("Search"))
        }
        Spacer(Modifier.weight(1f))
        Button(onClick = { browse(if (listening) "home" else "listen") }) { Text(interfaceText(if (listening) "Watch" else "Listen")) }
        IconButton(onClick = { TvFocusMemory.id = "settings"; settings() }, modifier = Modifier.restoresTvFocus("settings")) { Icon(Icons.Filled.Settings, contentDescription = interfaceText("Settings")) }
    }
}

@Composable
internal fun TvBrowseTiles(views: List<Pair<String, String>>, browse: (String) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(interfaceText("Browse"), style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold,
            color = KinoColor.text, modifier = Modifier.semantics { heading() })
        LazyRow(Modifier.bleed(24.dp, 0.dp), horizontalArrangement = Arrangement.spacedBy(18.dp),
            contentPadding = PaddingValues(horizontal = 24.dp, vertical = 12.dp)) {
            items(views, key = { it.first }) { (view, title) ->
                val label = interfaceText(title)
                Card(onClick = { TvFocusMemory.id = "browse:$view"; browse(view) },
                    modifier = Modifier.width(200.dp).restoresTvFocus("browse:$view").semantics { contentDescription = label },
                    colors = CardDefaults.colors(containerColor = KinoColor.surface)) {
                    Row(Modifier.fillMaxWidth().heightIn(min = 60.dp)
                        .background(Brush.linearGradient(listOf(KinoColor.raised, KinoColor.surface)))
                        .padding(horizontal = 12.dp), verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(9.dp)) {
                        Icon(painterResource(browseIcon(view)), contentDescription = null, tint = KinoColor.signal,
                            modifier = Modifier.size(30.dp))
                        Text(label, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold,
                            color = KinoColor.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                }
            }
        }
    }
}

@Composable
internal fun TvHomeHero(item: CatalogItem) {
    val metadata = listOf(if (item.showId.isNotEmpty()) "S${item.season} E${item.episode}" else item.year, item.rating, item.genres)
        .filter(String::isNotEmpty).joinToString(" · ")
    Column(Modifier.widthIn(max = 520.dp).padding(top = 20.dp).semantics { hideFromAccessibility() },
        verticalArrangement = Arrangement.spacedBy(5.dp)) {
        Text(item.title, style = MaterialTheme.typography.headlineMedium, fontWeight = FontWeight.Bold, color = KinoColor.text,
            maxLines = 1, overflow = TextOverflow.Ellipsis)
        Text(metadata.ifEmpty { " " }, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Medium,
            color = KinoColor.muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
        Text(item.plot.ifEmpty { " " }, style = MaterialTheme.typography.bodyLarge, color = KinoColor.text.copy(alpha = 0.88f),
            minLines = 2, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.widthIn(max = 430.dp))
    }
}

private fun browseIcon(view: String) = when (view) {
    "movies" -> R.drawable.nav_movies
    "shows" -> R.drawable.nav_shows
    "music" -> R.drawable.nav_music
    "audiobooks" -> R.drawable.nav_audiobooks
    "photos" -> R.drawable.nav_photos
    "list" -> R.drawable.nav_list
    "collections" -> R.drawable.nav_collections
    else -> R.drawable.nav_all
}
