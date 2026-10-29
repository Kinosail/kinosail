package com.kinosail.player.mobile

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import com.kinosail.player.R
import com.kinosail.player.core.PersonalTabs
import com.kinosail.player.core.interfaceText

@Composable
internal fun LibraryNavigation(tabs: List<String>, selected: String, navigate: (String) -> Unit,
                               content: @Composable () -> Unit) {
    BoxWithConstraints(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background).safeDrawingPadding()) {
        val rail = maxWidth >= 600.dp && maxHeight >= 480.dp
        val destinations = tabs + "more"
        if (rail) Row(Modifier.fillMaxSize()) {
            NavigationRail {
                destinations.forEach { destination ->
                    NavigationRailItem(selected = selected == destination, onClick = { navigate(destination) },
                        icon = { DestinationIcon(destination) }, label = { TabLabel(destination) })
                }
            }
            Box(Modifier.weight(1f)) { content() }
        } else Column(Modifier.fillMaxSize()) {
            Box(Modifier.weight(1f)) { content() }
            NavigationBar(windowInsets = WindowInsets(0, 0, 0, 0)) {
                destinations.forEach { destination ->
                    NavigationBarItem(selected = selected == destination, onClick = { navigate(destination) },
                        icon = { DestinationIcon(destination) }, label = { TabLabel(destination) })
                }
            }
        }
    }
}

@Composable
private fun TabLabel(destination: String) {
    val density = LocalDensity.current
    CompositionLocalProvider(LocalDensity provides Density(density.density, minOf(density.fontScale, 1.15f))) {
        Text(tabTitle(destination), maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

@Composable
internal fun DestinationIcon(destination: String, tint: androidx.compose.ui.graphics.Color = LocalContentColor.current) {
    if (destination == "settings") return Icon(Icons.Filled.Settings, contentDescription = null, tint = tint)
    val icon = when (destination) {
        "home" -> R.drawable.nav_home
        "shows" -> R.drawable.nav_shows
        "movies" -> R.drawable.nav_movies
        "search" -> R.drawable.nav_search
        "more" -> R.drawable.nav_more
        "music" -> R.drawable.nav_music
        "listen" -> R.drawable.nav_listen
        "audiobooks" -> R.drawable.nav_audiobooks
        "books" -> R.drawable.nav_books
        "photos" -> R.drawable.nav_photos
        "collections" -> R.drawable.nav_collections
        "list" -> R.drawable.nav_list
        "all" -> R.drawable.nav_all
        else -> R.drawable.nav_library
    }
    Icon(painterResource(icon), contentDescription = null, tint = tint)
}

@Composable
internal fun tabTitle(id: String) = interfaceText(if (id == "more") "More"
    else PersonalTabs.destinations.first { it.first == id }.second)

@Composable
internal fun MoreScreen(tabs: List<String>, saveTabs: (List<String>) -> Unit, navigate: (String) -> Unit) {
    var editing by rememberSaveable { mutableStateOf(false) }
    BackHandler(editing) { editing = false }
    if (editing) return TabEditor(tabs, saveTabs) { editing = false }
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(20.dp)) {
        Text(interfaceText("More"), style = MaterialTheme.typography.headlineLarge, modifier = Modifier.semantics { heading() })
        MoreGroup {
            PersonalTabs.destinations.filterNot { it.first in tabs }.forEachIndexed { index, (id, title) ->
                if (index > 0) MoreDivider()
                MoreRow(interfaceText(title), icon = id) { navigate(id) }
            }
        }
        MoreGroup { MoreRow(interfaceText("Customize tabs")) { editing = true } }
    }
}

@Composable
internal fun TabEditor(tabs: List<String>, saveTabs: (List<String>) -> Unit, done: () -> Unit) {
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(interfaceText("Customize tabs"), style = MaterialTheme.typography.headlineLarge, modifier = Modifier.semantics { heading() })
        Text(interfaceText("Choose one to four destinations. More keeps everything else available."))
        tabs.forEachIndexed { index, id ->
            Column {
                Text(tabTitle(id), style = MaterialTheme.typography.titleMedium)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    TextButton(onClick = { saveTabs(tabs.toMutableList().apply {
                        java.util.Collections.swap(this, index, index - 1)
                    }) }, enabled = index > 0) { Text("Move ${tabTitle(id)} earlier") }
                    TextButton(onClick = { saveTabs(tabs.toMutableList().apply {
                        java.util.Collections.swap(this, index, index + 1)
                    }) }, enabled = index < tabs.lastIndex) { Text("Move ${tabTitle(id)} later") }
                    TextButton(onClick = { saveTabs(tabs - id) }, enabled = tabs.size > 1) {
                        Text("Remove ${tabTitle(id)}")
                    }
                }
            }
        }
        HorizontalDivider()
        PersonalTabs.destinations.filterNot { it.first in tabs }.forEach { (id, title) ->
            TextButton(onClick = { saveTabs(tabs + id) }, enabled = tabs.size < 4) { Text("Add ${interfaceText(title)}") }
        }
        TextButton(onClick = { saveTabs(PersonalTabs.defaults) }) { Text(interfaceText("Reset tabs")) }
        Button(onClick = done) { Text(interfaceText("Done")) }
    }
}
