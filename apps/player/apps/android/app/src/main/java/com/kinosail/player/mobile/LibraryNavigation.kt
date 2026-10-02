package com.kinosail.player.mobile

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.kinosail.player.R
import com.kinosail.player.core.PersonalTabs
import com.kinosail.player.core.Viewer
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
                        icon = { DestinationIcon(destination) }, label = { Text(tabTitle(destination)) })
                }
            }
            Box(Modifier.weight(1f)) { content() }
        } else Column(Modifier.fillMaxSize()) {
            Box(Modifier.weight(1f)) { content() }
            NavigationBar(windowInsets = WindowInsets(0, 0, 0, 0)) {
                destinations.forEach { destination ->
                    NavigationBarItem(selected = selected == destination, onClick = { navigate(destination) },
                        icon = { DestinationIcon(destination) }, label = { Text(tabTitle(destination)) })
                }
            }
        }
    }
}

@Composable
private fun DestinationIcon(destination: String) {
    val icon = when (destination) {
        "home" -> R.drawable.nav_home
        "shows" -> R.drawable.nav_shows
        "movies" -> R.drawable.nav_movies
        "search" -> R.drawable.nav_search
        "more" -> R.drawable.nav_more
        else -> R.drawable.nav_library
    }
    Icon(painterResource(icon), contentDescription = null)
}

@Composable
internal fun tabTitle(id: String) = interfaceText(if (id == "more") "More"
    else PersonalTabs.destinations.first { it.first == id }.second)

@Composable
internal fun MoreScreen(viewer: Viewer, tabs: List<String>, saveTabs: (List<String>) -> Unit,
                        navigate: (String) -> Unit, signOut: () -> Unit) {
    var editing by remember { mutableStateOf(false) }
    BackHandler(editing) { editing = false }
    var thanks by remember { mutableStateOf(false) }
    var notices by remember { mutableStateOf(false) }
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(interfaceText(if (editing) "Customize tabs" else "More"), style = MaterialTheme.typography.headlineLarge)
        if (editing) {
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
            Button(onClick = { editing = false }) { Text(interfaceText("Done")) }
        } else {
            PersonalTabs.destinations.filterNot { it.first in tabs }.forEach { (id, title) ->
                TextButton(onClick = { navigate(id) }, modifier = Modifier.fillMaxWidth()) {
                    Text(interfaceText(title), modifier = Modifier.fillMaxWidth())
                }
            }
            HorizontalDivider()
            Text(interfaceText("Settings"), style = MaterialTheme.typography.titleLarge)
            Text(viewer.name, style = MaterialTheme.typography.titleMedium)
            Text(viewer.server, color = MaterialTheme.colorScheme.onSurfaceVariant)
            TextButton(onClick = { editing = true }) { Text(interfaceText("Customize tabs")) }
            TextButton(onClick = { thanks = !thanks }) { Text("Made possible by") }
            if (thanks) {
                Text("Thank you to the people behind Jetpack Compose, Media3, and the Android libraries that bring Kinosail to this device. FFmpeg and Jellyfin FFmpeg power media tools on your Server.")
                Text("This product uses the TMDB API but is not endorsed or certified by TMDB.")
                TextButton(onClick = { notices = !notices }) { Text("Third-party notices") }
                if (notices) {
                    val context = LocalContext.current
                    val text = remember { runCatching { context.assets.open("THIRD_PARTY_NOTICES.md")
                        .bufferedReader().use { it.readText() } }.getOrNull() }
                    Text(text ?: "Notices could not be opened. Reinstall Kinosail and try again.")
                }
            }
            TextButton(onClick = signOut) { Text(interfaceText("Sign out")) }
        }
    }
}
