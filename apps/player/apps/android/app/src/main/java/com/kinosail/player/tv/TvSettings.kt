package com.kinosail.player.tv

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.AccountCircle
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.TextButton
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.tv.material3.*
import com.kinosail.player.R
import com.kinosail.player.core.CatalogModel
import com.kinosail.player.core.Viewer
import com.kinosail.player.core.interfaceText
import com.kinosail.player.core.restoresTvFocus
import com.kinosail.player.core.TvFocusMemory
import com.kinosail.player.design.KinoColor
import com.kinosail.player.design.SailBackdrop
import com.kinosail.player.mobile.ConnectTvScreen
import com.kinosail.player.mobile.PlaybackDefaultsScreen

@Composable
internal fun TvSettings(viewer: Viewer, catalog: CatalogModel, signOut: () -> Unit) {
    var page by remember(viewer.serverId, viewer.id) { mutableStateOf("main") }
    BackHandler(page != "main") { TvFocusMemory.id = "settings:$page"; page = "main" }
    Box(Modifier.fillMaxSize().background(KinoColor.background)) {
        SailBackdrop()
        Row(Modifier.fillMaxSize().safeDrawingPadding().padding(horizontal = 48.dp, vertical = 32.dp),
            horizontalArrangement = Arrangement.spacedBy(64.dp)) {
            Column(Modifier.weight(0.38f).fillMaxHeight(), verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.End) {
                Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
                    Image(painterResource(R.drawable.app_icon), contentDescription = null, modifier = Modifier.size(72.dp))
                    Icon(Icons.Filled.Settings, contentDescription = null, tint = KinoColor.signal, modifier = Modifier.size(48.dp))
                    Text(interfaceText("Settings"), style = MaterialTheme.typography.headlineMedium, fontWeight = FontWeight.Bold,
                        color = KinoColor.text, modifier = Modifier.semantics { heading() })
                }
            }
            Box(Modifier.weight(0.62f).fillMaxHeight()) {
                when (page) {
                    "playback" -> PlaybackDefaultsScreen(catalog)
                    "approve" -> ConnectTvScreen(viewer, catalog) { TvFocusMemory.id = "settings:approve"; page = "main" }
                    else -> Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()),
                        verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        SettingsGroups(viewer, catalog, open = { page = it }, signOut = signOut)
                    }
                }
            }
        }
    }
}

@Composable
private fun SettingsGroups(viewer: Viewer, catalog: CatalogModel, open: (String) -> Unit, signOut: () -> Unit) {
    var confirming by remember { mutableStateOf(false) }
    var thanks by remember { mutableStateOf(false) }
    var notices by remember { mutableStateOf(false) }
    val context = LocalContext.current
    Section(interfaceText("Server & Viewer Profile"))
    Row(Modifier.fillMaxWidth().background(KinoColor.surface, RoundedCornerShape(16.dp)).padding(20.dp)
        .semantics(mergeDescendants = true) { contentDescription = "Viewer Profile: ${viewer.name}. Server: ${viewer.server}" },
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
        Icon(Icons.Filled.AccountCircle, contentDescription = null, tint = KinoColor.signal, modifier = Modifier.size(48.dp))
        Column(Modifier.clearAndSetSemantics { }) {
            Text(viewer.name, style = MaterialTheme.typography.titleLarge, color = KinoColor.text)
            Text(viewer.server, style = MaterialTheme.typography.bodyLarge, color = KinoColor.muted)
        }
    }
    Note(interfaceText("Everyone using this TV shares the connected Viewer Profile, including its library access and watch history."))
    SettingsRow(interfaceText("Connect a TV"), "approve", icon = painterResource(R.drawable.nav_shows)) { open("approve") }
    catalog.connection?.first?.server?.url?.toString()?.trimEnd('/')?.let { Note(it) }
    Section(interfaceText("Playback & library"))
    SettingsRow(interfaceText("Playback"), "playback", vector = Icons.Filled.PlayArrow) { open("playback") }
    Section(interfaceText("About"))
    SettingsRow(interfaceText("Made possible by"), "thanks", vector = Icons.Filled.Favorite, chevron = false) { thanks = !thanks }
    if (thanks) {
        Note("Thank you to the people behind Jetpack Compose, Media3, and the Android libraries that bring Kinosail to this device. FFmpeg and Jellyfin FFmpeg power media tools on your Server.")
        Note("This product uses the TMDB API but is not endorsed or certified by TMDB.")
        SettingsRow(interfaceText("Third-party notices"), "notices", chevron = false) { notices = !notices }
        if (notices) Note(remember { runCatching { context.assets.open("THIRD_PARTY_NOTICES.md").bufferedReader().use { it.readText() } }
            .getOrDefault("Notices could not be opened. Reinstall Kinosail and try again.") })
    }
    Spacer(Modifier.height(12.dp))
    Note(interfaceText("Kinosail plays directly from your Server. Your session is stored securely on this device."))
    SettingsRow(interfaceText("Sign out"), "sign-out", color = KinoColor.danger, chevron = false) { confirming = true }
    if (confirming) AlertDialog(onDismissRequest = { confirming = false },
        title = { androidx.compose.material3.Text(interfaceText("Sign out of this device?")) },
        text = { androidx.compose.material3.Text(interfaceText("You’ll need to connect this device to your Server again.")) },
        confirmButton = { TextButton(onClick = { confirming = false; signOut() }) {
            androidx.compose.material3.Text(interfaceText("Sign out"), color = KinoColor.danger)
        } },
        dismissButton = { TextButton(onClick = { confirming = false }) { androidx.compose.material3.Text(interfaceText("Cancel")) } },
        containerColor = KinoColor.surface)
}

@Composable
private fun Section(title: String) {
    Text(title, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold, color = KinoColor.muted,
        modifier = Modifier.padding(start = 20.dp, top = 16.dp).semantics { heading() })
}

@Composable
private fun Note(text: String) {
    Text(text, style = MaterialTheme.typography.bodyMedium, color = KinoColor.muted, modifier = Modifier.padding(horizontal = 20.dp))
}

@Composable
private fun SettingsRow(title: String, key: String, vector: ImageVector? = null,
                        icon: androidx.compose.ui.graphics.painter.Painter? = null, color: Color = KinoColor.text,
                        chevron: Boolean = true, onClick: () -> Unit) {
    ListItem(selected = false, onClick = { TvFocusMemory.id = null; onClick() },
        modifier = Modifier.fillMaxWidth().restoresTvFocus("settings:$key"),
        shape = ListItemDefaults.shape(RoundedCornerShape(16.dp)),
        colors = ListItemDefaults.colors(containerColor = KinoColor.surface, contentColor = color,
            focusedContainerColor = Color.White, focusedContentColor = Color.Black),
        leadingContent = when {
            vector != null -> { { Icon(vector, contentDescription = null, modifier = Modifier.size(ListItemDefaults.IconSize)) } }
            icon != null -> { { Icon(icon, contentDescription = null, modifier = Modifier.size(ListItemDefaults.IconSize)) } }
            else -> null
        },
        trailingContent = if (chevron) { { Icon(Icons.AutoMirrored.Filled.KeyboardArrowRight, contentDescription = null) } } else null,
        headlineContent = { Text(title, style = MaterialTheme.typography.titleMedium) })
}
