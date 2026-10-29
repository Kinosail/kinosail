package com.kinosail.player.mobile

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AccountCircle
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.kinosail.player.R
import com.kinosail.player.core.CatalogModel
import com.kinosail.player.core.Viewer
import com.kinosail.player.core.interfaceText
import com.kinosail.player.design.KinoColor

@Composable
internal fun SettingsScreen(viewer: Viewer, catalog: CatalogModel, tabs: List<String>,
                            saveTabs: (List<String>) -> Unit, signOut: () -> Unit) {
    var page by rememberSaveable(viewer.serverId, viewer.id) { mutableStateOf("main") }
    BackHandler(page != "main") { page = "main" }
    when (page) {
        "playback" -> PlaybackDefaultsScreen(catalog)
        "approve" -> ConnectTvScreen(viewer, catalog) { page = "main" }
        "tabs" -> TabEditor(tabs, saveTabs) { page = "main" }
        "thanks" -> ThanksScreen()
        else -> SettingsList(viewer, catalog, open = { page = it }, signOut = signOut)
    }
}

@Composable
private fun SettingsList(viewer: Viewer, catalog: CatalogModel, open: (String) -> Unit, signOut: () -> Unit) {
    var confirming by remember { mutableStateOf(false) }
    val address = catalog.connection?.first?.server?.url?.toString()?.trimEnd('/')
    SettingsPage(interfaceText("Settings")) {
        MoreSection(interfaceText("Server & Viewer Profile"))
        MoreGroup {
            Row(Modifier.fillMaxWidth().padding(18.dp).semantics(mergeDescendants = true) {
                contentDescription = "Viewer Profile: ${viewer.name}. Server: ${viewer.server}"
            }, verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                Icon(Icons.Filled.AccountCircle, contentDescription = null, tint = KinoColor.signal, modifier = Modifier.size(40.dp))
                Column(Modifier.clearAndSetSemantics { }, verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(viewer.name, style = MaterialTheme.typography.titleMedium, color = KinoColor.text)
                    Text(viewer.server, style = MaterialTheme.typography.bodyMedium, color = KinoColor.muted)
                }
            }
            MoreDivider(icon = false)
            MoreRow(interfaceText("Connect a TV"), icon = "shows") { open("approve") }
            if (address != null) {
                MoreDivider(icon = false)
                Text(address, style = MaterialTheme.typography.bodySmall, color = KinoColor.muted,
                    modifier = Modifier.padding(18.dp))
            }
        }
        MoreSection(interfaceText("Playback & library"))
        MoreGroup { MoreRow(interfaceText("Playback"), vector = Icons.Filled.PlayArrow) { open("playback") } }
        MoreSection(interfaceText("Personalize"))
        MoreGroup { MoreRow(interfaceText("Customize tabs"), icon = "all") { open("tabs") } }
        MoreSection(interfaceText("About"))
        MoreGroup { MoreRow(interfaceText("Made possible by"), vector = Icons.Filled.Favorite) { open("thanks") } }
        Text(interfaceText("Kinosail plays directly from your Server. Your session is stored securely on this device."),
            style = MaterialTheme.typography.bodySmall, color = KinoColor.muted, modifier = Modifier.padding(horizontal = 18.dp))
        MoreGroup { MoreRow(interfaceText("Sign out"), color = KinoColor.danger, chevron = false) { confirming = true } }
    }
    if (confirming) AlertDialog(onDismissRequest = { confirming = false },
        title = { Text(interfaceText("Sign out of this device?")) },
        text = { Text(interfaceText("You’ll need to connect this device to your Server again.")) },
        confirmButton = { TextButton(onClick = { confirming = false; signOut() }) {
            Text(interfaceText("Sign out"), color = KinoColor.danger)
        } },
        dismissButton = { TextButton(onClick = { confirming = false }) { Text(interfaceText("Cancel")) } },
        containerColor = KinoColor.surface)
}

@Composable
internal fun SettingsPage(title: String, content: @Composable ColumnScope.() -> Unit) {
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(20.dp)
        .widthIn(max = 700.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(title, style = MaterialTheme.typography.headlineLarge, color = KinoColor.text,
            modifier = Modifier.padding(bottom = 8.dp).semantics { heading() })
        content()
    }
}

@Composable
private fun ThanksScreen() {
    var notices by remember { mutableStateOf(false) }
    SettingsPage(interfaceText("Made possible by")) {
        Text(interfaceText("Thank you to the people who make private playback possible."),
            style = MaterialTheme.typography.titleLarge, color = KinoColor.text)
        Text("Thank you to the people behind Jetpack Compose, Media3, and the Android libraries that bring Kinosail to this device. FFmpeg and Jellyfin FFmpeg power media tools on your Server.",
            color = KinoColor.text)
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            Image(painterResource(R.drawable.tmdb_logo), contentDescription = "TMDB", modifier = Modifier.size(80.dp, 58.dp))
            Text("This product uses the TMDB API but is not endorsed or certified by TMDB.",
                style = MaterialTheme.typography.bodySmall, color = KinoColor.text)
        }
        TextButton(onClick = { notices = !notices }) { Text(interfaceText("Third-party notices")) }
        if (notices) {
            val context = LocalContext.current
            val text = remember { runCatching { context.assets.open("THIRD_PARTY_NOTICES.md")
                .bufferedReader().use { it.readText() } }.getOrNull() }
            Text(text ?: "Notices could not be opened. Reinstall Kinosail and try again.", color = KinoColor.text)
        }
    }
}
