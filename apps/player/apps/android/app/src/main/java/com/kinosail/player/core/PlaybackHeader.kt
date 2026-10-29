package com.kinosail.player.core

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.FlowRowScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.FilledTonalIconButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.kinosail.player.R

@Composable
internal fun PlaybackHeader(item: CatalogItem, tv: Boolean, close: () -> Unit, actions: @Composable FlowRowScope.() -> Unit) {
    val episode = item.showId.isNotEmpty()
    val subtitle = if (episode) "S${item.season} E${item.episode}" else item.year
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            if (!tv) FilledTonalIconButton(onClick = close, modifier = Modifier.size(44.dp), shape = CircleShape,
                colors = IconButtonDefaults.filledTonalIconButtonColors(containerColor = Color.White.copy(alpha = 0.18f),
                    contentColor = Color.White)) {
                Icon(painterResource(R.drawable.chevron_down), contentDescription = interfaceText("Close player"))
            }
            Column(Modifier.weight(1f)) {
                Text(if (episode) item.episodeTitle else item.title, color = Color.White, maxLines = 2,
                    overflow = TextOverflow.Ellipsis, fontWeight = FontWeight.SemiBold,
                    style = if (tv) MaterialTheme.typography.headlineSmall else MaterialTheme.typography.titleMedium,
                    modifier = Modifier.semantics { heading() })
                if (subtitle.isNotEmpty()) Text(subtitle, color = Color.White.copy(alpha = 0.7f), maxLines = 1,
                    style = MaterialTheme.typography.labelMedium)
            }
        }
        FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp), content = actions)
    }
}

internal val PlaybackControllerHeight = 96.dp

@Composable
internal fun PlaybackPromptButton(prompt: PlaybackPrompt, tv: Boolean, controlsVisible: Boolean, focus: FocusRequester,
                                  onClick: () -> Unit) {
    Box(Modifier.fillMaxSize().safeDrawingPadding()
        .padding(end = if (tv) 48.dp else 24.dp, bottom = (if (controlsVisible) PlaybackControllerHeight else 0.dp) + 24.dp),
        contentAlignment = Alignment.BottomEnd) {
        val label = interfaceText(prompt.title)
        if (tv) androidx.tv.material3.Button(onClick = onClick, modifier = Modifier.focusRequester(focus),
            colors = androidx.tv.material3.ButtonDefaults.colors(containerColor = Color.White, contentColor = Color.Black)) {
            androidx.tv.material3.Text(label, fontWeight = FontWeight.SemiBold)
        } else androidx.compose.material3.Button(onClick = onClick, modifier = Modifier.focusRequester(focus),
            colors = androidx.compose.material3.ButtonDefaults.buttonColors(containerColor = Color.White, contentColor = Color.Black)) {
            Text(label, fontWeight = FontWeight.SemiBold)
        }
    }
}

@Composable
internal fun PlaybackTopFade(tv: Boolean) {
    Box(Modifier.fillMaxWidth().height(if (tv) 320.dp else 240.dp)
        .background(Brush.verticalGradient(listOf(Color.Black.copy(alpha = 0.8f), Color.Transparent))))
}

@Composable
internal fun PlaybackAction(label: String, tv: Boolean, onClick: () -> Unit, enabled: Boolean = true,
                            focus: FocusRequester? = null) {
    val modifier = focus?.let { Modifier.focusRequester(it) } ?: Modifier
    if (tv) androidx.tv.material3.Button(onClick = onClick, enabled = enabled, modifier = modifier) {
        androidx.tv.material3.Text(label)
    } else TextButton(onClick = onClick, enabled = enabled, modifier = modifier,
        colors = androidx.compose.material3.ButtonDefaults.textButtonColors(
            containerColor = Color.White.copy(alpha = 0.16f), contentColor = Color.White)) {
        Text(label, fontWeight = FontWeight.Medium)
    }
}
